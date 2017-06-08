package pipe

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"leb.io/hashland/siphash"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/crypto"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
)

const MaxWindowSize = 256
const MaxAckBacklog = 20

type Resolver interface {
	LookupSelector(*mesh.PipeSelector) (mesh.Identity, string, error)
	RemoveAdvertisement(*pb.Advertisement)
}

type pipeKey struct {
	ep string
	id uint64
}

type unknownData struct {
	pipeMessage
	expire time.Time
}

type DataHandler struct {
	PipeBacklog int
	AckBacklog  int

	peerProto   int32
	resolver    Resolver
	sender      mesh.Sender
	identityKey crypto.DHKey

	nextSession *uint64

	pipeLock  sync.Mutex
	listening map[string]*ListenPipe
	pipes     map[pipeKey]*Pipe

	unknownLock  sync.Mutex
	unknownDatas map[uint64][]unknownData
}

func (d *DataHandler) Setup(proto int32, s mesh.Sender, k crypto.DHKey) {
	if d.PipeBacklog == 0 {
		d.PipeBacklog = 10
	}

	if d.AckBacklog == 0 {
		d.AckBacklog = MaxAckBacklog
	}

	d.peerProto = proto
	d.sender = s
	d.identityKey = k
	d.nextSession = new(uint64)
	d.listening = make(map[string]*ListenPipe)
	d.pipes = make(map[pipeKey]*Pipe)
	d.unknownDatas = make(map[uint64][]unknownData)

	go d.invalidateUnknowns()
}

func (d *DataHandler) Cleanup() {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	for k, p := range d.pipes {
		d.closePipeInAnger(p, k)
	}
}

func (d *DataHandler) invalidateUnknowns() {
	ctx := context.Background()

	for {
		d.unknownLock.Lock()

		now := time.Now()

		var toDelete []uint64

		for sess, sl := range d.unknownDatas {
			for _, uk := range sl {
				if uk.expire.After(now) {
					toDelete = append(toDelete, sess)
					log.Debugf("sending unknown session message")

					var errM Message
					errM.Type = PIPE_UNKNOWN
					errM.Session = sess
					errM.AckId = uk.msg.SeqId

					d.sender.SendData(ctx, uk.hdr.Sender, d.peerProto, &errM)
					break
				}
			}
		}

		for _, sess := range toDelete {
			delete(d.unknownDatas, sess)
		}

		d.unknownLock.Unlock()
		time.Sleep(500 * time.Millisecond)
	}
}

func (d *DataHandler) sessionId(dest mesh.Identity) uint64 {
	sh := siphash.New(d.identityKey.Private)
	sh.Write(dest)

	sess := atomic.AddUint64(d.nextSession, 1)
	binary.Write(sh, binary.BigEndian, sess)

	// Adding the current time will prevent us from being confused
	// by messages for sessions that occured before this peer
	// was restarted.
	binary.Write(sh, binary.BigEndian, time.Now().UnixNano())

	return sh.Sum64()
}

func (d *DataHandler) Handle(ctx context.Context, hdr *pb.Header) error {
	msg := new(Message)

	err := msg.Unmarshal(hdr.Body)
	if err != nil {
		return err
	}

	log.Debugf("%s handle: %s", d.desc(), msg.Type)

	switch msg.Type {
	case PIPE_OPEN:
		d.newPipeRequest(ctx, hdr, msg)
	case PIPE_OPENED:
		d.setPipeOpened(ctx, hdr, msg)
	case PIPE_DATA:
		d.newPipeData(ctx, hdr, msg)
	case PIPE_DATA_ACK:
		d.ackPipeData(ctx, hdr, msg)
	case PIPE_CLOSE:
		d.setPipeClosed(ctx, hdr, msg)
	case PIPE_UNKNOWN:
		d.setPipeUnknown(ctx, hdr, msg)
	default:
		log.Debugf("Unknown pipe operation: %s", msg.Type)
	}

	return nil
}

func (d *DataHandler) desc() string {
	return d.identityKey.Identity().Short()
}

func (d *DataHandler) setPipeOpened(ctx context.Context, hdr *pb.Header, msg *Message) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	if pipe, ok := d.pipes[mkpipeKey(hdr.Sender, msg.Session)]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		log.Debugf("%s pipe opened: %d", d.desc(), msg.Session)

		d.clearAcks(pipe, msg.AckId)
		pipe.recvThreshold = msg.AckId

		if msg.Encrypted {
			if pipe.ks == nil {
				panic(fmt.Sprintf("%s encrypted opened but handshake not setup: %d", d.desc(), msg.Session))
			}

			_, csr, csw, err := pipe.ks.Finish(nil, msg.Data)
			if err != nil {
				log.Printf("%s error finishing encryption handshake: %s", d.desc(), err)
				return
			}

			pipe.csr = csr
			pipe.csw = csw
		}

		if pipe.pending != nil {
			close(pipe.pending)
		}
	} else {
		log.Printf("%s unknown pipe reported as opened? %s.%d",
			d.desc(), hdr.Sender.Short(), msg.Session)
	}
}

func (d *DataHandler) setPipeUnknown(ctx context.Context, hdr *pb.Header, msg *Message) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	if pipe, ok := d.pipes[mkpipeKey(hdr.Sender, msg.Session)]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		pipe.lifetimeCancel()

		d.clearAcks(pipe, msg.AckId)

		if pipe.closed == true {
			// kewl
			return
		}

		close(pipe.message)

		pipe.err = ErrUnknownPipe
		pipe.closed = true

		if pipe.pending != nil {
			close(pipe.pending)
		}
	} else {
		log.Printf("%s unknown pipe reported as unknown? %s.%d",
			d.desc(), hdr.Sender.Short(), msg.Session)
	}
}

func (d *DataHandler) drainPossibleUnknowns(ctx context.Context, pipe *Pipe, session uint64) {
	if sl, ok := d.unknownDatas[session]; ok {
		for _, pm := range sl {
			// Store them in the window because they might be out of order!
			d.storeInWindow(pipe, pm.hdr, pm.msg)
		}

		// Now deal with anything in the window we can
		d.drainWindow(ctx, pipe)
	}
}

func (d *DataHandler) newPipeRequest(ctx context.Context, hdr *pb.Header, msg *Message) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	log.Debugf("%s requesting new pipe from %s", d.desc(), hdr.Sender.Short())

	if lp, ok := d.listening[msg.PipeName]; ok {
		pipe := &Pipe{
			other:      hdr.Sender,
			handler:    d,
			session:    msg.Session,
			message:    make(chan pipeMessage, d.PipeBacklog),
			nextSeqId:  1,
			ackBacklog: d.AckBacklog,
		}

		pipe.init()

		_, ok := d.pipes[mkpipeKey(pipe.other, msg.Session)]
		if ok {
			// Umm, ok?
			return
		}

		d.pipes[mkpipeKey(pipe.other, msg.Session)] = pipe

		var (
			zrrtData   []byte
			retPayload []byte
		)

		if msg.Encrypted {
			ks := crypto.NewKKResponder(d.identityKey, hdr.Sender)

			out, err := ks.Start(nil, msg.Data)
			if err != nil {
				log.Printf("%s error decrypting responder handshake: %s", d.desc(), err)

				var out Message
				out.Type = PIPE_UNKNOWN
				out.Session = msg.Session

				err := d.sender.SendData(ctx, hdr.Sender, d.peerProto, &out)
				if err != nil {
					log.Debugf("Error sending PIPE_UNKNOWN: %s", err)
				}

				return
			}

			// Marshal and encrypt the return message to complete
			// the handshake

			enc, csw, csr := ks.Finish(nil, nil)
			retPayload = enc

			pipe.csr = csr
			pipe.csw = csw

			zrrtData = out
		}

		// TODO implement a backlog via a buffered channel and a
		// nonblocking send here
		lp.newPipes <- pipe

		// This is to support 0-RTT connects
		if len(zrrtData) > 0 {
			pipe.inputThreshold = msg.SeqId
			log.Debugf("%s detected 0-rtt pipe open: %d bytes (%d)", d.desc(), len(msg.Data), msg.SeqId)
			pipe.message <- pipeMessage{hdr, msg, zrrtData}
		}

		var ret Message
		ret.Type = PIPE_OPENED
		ret.Session = msg.Session
		ret.AckId = msg.SeqId

		if msg.Encrypted {
			log.Debugf("%s responding with encrypted PIPE_OPENED to %s (%d)",
				d.desc(), hdr.Sender.Short(), msg.Session)
			ret.Encrypted = true
			ret.Data = retPayload
		} else {
			log.Debugf("%s responding with unencrypted PIPE_OPENED %s (%d)",
				d.desc(), hdr.Sender.Short(), msg.Session)
		}

		err := d.sender.SendData(ctx, hdr.Sender, d.peerProto, &ret)
		if err != nil {
			log.Debugf("Error sending PIPE_OPENED: %s", err)
		}

		if len(zrrtData) > 0 {
			d.drainPossibleUnknowns(ctx, pipe, msg.Session)
		}
	} else {
		log.Debugf("%s unknown pipe requested: %s", d.desc(), msg.PipeName)

		var errM Message
		errM.Type = PIPE_UNKNOWN
		errM.Session = msg.Session

		err := d.sender.SendData(ctx, hdr.Sender, d.peerProto, &errM)
		if err != nil {
			log.Debugf("Error sending PIPE_UNKNOWN: %s", err)
		}
	}
}

var ErrWindowTooLarge = errors.New("window would exceed maximum size")

func (d *DataHandler) storeInWindow(pipe *Pipe, hdr *pb.Header, msg *Message) error {
	pos := msg.SeqId - pipe.inputThreshold - 1

	if pos >= MaxWindowSize {
		log.Printf("%s Maximum window distance detected, aborting pipe: %d", d.desc(), pipe.session)
		return ErrWindowTooLarge
	}

	for uint64(len(pipe.window)) <= pos {
		pipe.window = append(pipe.window, pipeMessage{})
	}

	if pipe.windowUsed <= pos {
		pipe.windowUsed = pos + 1
	}

	log.Debugf("store in window (%d => %d). used=%d", msg.SeqId, pos, pipe.windowUsed)

	pipe.window[pos] = pipeMessage{hdr, msg, nil}
	return nil
}

func (d *DataHandler) drainWindow(ctx context.Context, pipe *Pipe) {
	log.Debugf("%s drain window from %d => %d", d.desc(), pipe.windowStart, pipe.windowUsed)
	for i := pipe.windowStart; i < pipe.windowUsed; i++ {
		pm := pipe.window[i]

		// Oop, hit a hole, can't continue
		if pm.hdr == nil {
			log.Debugf("window hole at %d", i)
			pipe.windowStart = i
			return
		}

		log.Debugf("deliver from window: %d (%d)", i, pm.msg.SeqId)

		pipe.message <- pm
	}

	pipe.windowStart = 0
	pipe.windowUsed = 0
}

func (d *DataHandler) closePipeInAnger(pipe *Pipe, key pipeKey) {
	delete(d.pipes, key)

	pipe.lifetimeCancel()

	if pipe.closed {
		return
	}

	pipe.err = ErrClosed
	pipe.closed = true

	close(pipe.message)
}

func (d *DataHandler) newPipeData(ctx context.Context, hdr *pb.Header, msg *Message) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	key := mkpipeKey(hdr.Sender, msg.Session)

	if pipe, ok := d.pipes[key]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		if msg.SeqId <= pipe.inputThreshold {
			log.Debugf("%s duplicate data packet on %d (%d)", d.desc(), msg.Session, msg.SeqId)
			return
		}

		// Detect out of order messages
		if pipe.windowUsed > 0 {
			// we're in out-of-order recovery mode. put everything into the window, then drain it.
			log.Debugf("%s Out of order recovery mode detected, using window", d.desc())
			err := d.storeInWindow(pipe, hdr, msg)
			if err != nil {
				d.closePipeInAnger(pipe, key)
			} else {
				d.drainWindow(ctx, pipe)
			}
			return
		} else if pipe.inputThreshold+1 != msg.SeqId {
			log.Debugf("%s Out of order message detected (%d). expected %d, got %d", d.desc(), msg.Session, pipe.inputThreshold+1, msg.SeqId)
			err := d.storeInWindow(pipe, hdr, msg)
			if err != nil {
				d.closePipeInAnger(pipe, key)
			}
			return
		}

		log.Debugf("%s set default threshold of %d to %d", d.desc(), pipe.session, msg.SeqId)
		pipe.inputThreshold = msg.SeqId

		if pipe.closed {
			log.Debugf("injected data to closed pipe: %d", msg.Session)
			var closeM Message
			closeM.Type = PIPE_CLOSE
			closeM.Session = msg.Session

			d.sender.SendData(ctx, hdr.Sender, d.peerProto, &closeM)
		} else {
			log.Debugf("%s inject pipe data: %v", d.desc(), msg.Session)

			pipe.message <- pipeMessage{hdr, msg, nil}

			d.drainWindow(ctx, pipe)
		}
	} else {
		d.unknownLock.Lock()

		d.unknownDatas[msg.Session] = append(
			d.unknownDatas[msg.Session],
			unknownData{
				pipeMessage{hdr, msg, nil},
				time.Now().Add(1 * time.Second),
			},
		)

		d.unknownLock.Unlock()
	}
}

func (d *DataHandler) clearAcks(pipe *Pipe, id uint64) {
	log.Debugf("%s clear acks under %d", d.desc(), id)
	var head int

	for i, uk := range pipe.unackedMessages {
		if uk.msg == nil {
			break
		} else if uk.msg.SeqId <= id {
			head = i
		} else {
			break
		}
	}

	if head == len(pipe.unackedMessages)-1 {
		// Reset the slice entirely to prevent messages from accidentally living
		// a long time in the array underneith.
		pipe.unackedMessages = nil
	} else {
		pipe.unackedMessages = pipe.unackedMessages[head:]
	}
}

func (d *DataHandler) ackPipeData(ctx context.Context, hdr *pb.Header, msg *Message) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	if pipe, ok := d.pipes[mkpipeKey(hdr.Sender, msg.Session)]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		d.clearAcks(pipe, msg.AckId)

		log.Debugf("%s update %d threshold to %d (unacked=%d)", d.desc(), pipe.session, msg.AckId, len(pipe.unackedMessages))
		pipe.recvThreshold = msg.AckId

		pipe.cond.Broadcast()
	}
}

func (d *DataHandler) setPipeClosed(ctx context.Context, hdr *pb.Header, msg *Message) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	key := mkpipeKey(hdr.Sender, msg.Session)

	if pipe, ok := d.pipes[key]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		if !pipe.closed {
			delete(d.pipes, key)

			// This is to support a final message without having to transmit
			// a close as a second message
			if len(msg.Data) > 0 {
				pipe.message <- pipeMessage{hdr, msg, nil}
			}

			pipe.lifetimeCancel()

			pipe.err = ErrClosed
			pipe.closed = true

			close(pipe.message)
		}
	} else if msg.PipeName != "" && len(msg.Data) > 0 {
		if lp, ok := d.listening[msg.PipeName]; ok {
			pipe := &Pipe{
				other:      hdr.Sender,
				handler:    d,
				session:    msg.Session,
				message:    make(chan pipeMessage, d.PipeBacklog),
				closed:     true,
				nextSeqId:  1,
				ackBacklog: d.AckBacklog,
			}

			pipe.init()

			// We don't even bother to register this pipe, no more
			// data is available on it.
			log.Debugf("noreply zero-rtt pipe created")

			// TODO implement a backlog via a buffered channel and a
			// nonblocking send here
			lp.newPipes <- pipe

			// This is to support 0-RTT connects
			pipe.message <- pipeMessage{hdr, msg, nil}
		} else {
			log.Debugf("unknown service '%s'in zero-rtt noreply pipe: %d",
				msg.Session, msg.PipeName)
		}
	} else {
		log.Printf("%s unknown pipe reported as closed? %s.%d",
			d.desc(), hdr.Sender.Short(), msg.Session)
	}
}
