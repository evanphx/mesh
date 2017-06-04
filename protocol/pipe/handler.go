package pipe

import (
	"context"
	"encoding/binary"
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

type Resolver interface {
	LookupSelector(*mesh.PipeSelector) (mesh.Identity, string, error)
	RemoveAdvertisement(*pb.Advertisement)
}

type pipeKey struct {
	ep string
	id uint64
}

type DataHandler struct {
	PipeBacklog int

	peerProto   int32
	resolver    Resolver
	sender      mesh.Sender
	identityKey crypto.DHKey

	nextSession *uint64

	pipeLock  sync.Mutex
	listening map[string]*ListenPipe
	pipes     map[pipeKey]*Pipe
}

func (d *DataHandler) Setup(proto int32, s mesh.Sender, k crypto.DHKey) {
	if d.PipeBacklog == 0 {
		d.PipeBacklog = 10
	}

	d.peerProto = proto
	d.sender = s
	d.identityKey = k
	d.nextSession = new(uint64)
	d.listening = make(map[string]*ListenPipe)
	d.pipes = make(map[pipeKey]*Pipe)
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

func (d *DataHandler) newPipeRequest(ctx context.Context, hdr *pb.Header, msg *Message) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	log.Debugf("%s requesting new pipe from %s", d.desc(), hdr.Sender.Short())

	if lp, ok := d.listening[msg.PipeName]; ok {
		pipe := &Pipe{
			other:     hdr.Sender,
			handler:   d,
			session:   msg.Session,
			message:   make(chan pipeMessage, d.PipeBacklog),
			nextSeqId: 1,
		}

		_, ok := d.pipes[mkpipeKey(pipe.other, msg.Session)]
		if ok {
			panic("already using pipe!")
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
			log.Debugf("detected 0-rtt pipe open: %d bytes", len(msg.Data))
			pipe.message <- pipeMessage{hdr, msg, zrrtData}
		}

		var ret Message
		ret.Type = PIPE_OPENED
		ret.Session = msg.Session

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

func (d *DataHandler) newPipeData(ctx context.Context, hdr *pb.Header, msg *Message) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	if pipe, ok := d.pipes[mkpipeKey(hdr.Sender, msg.Session)]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		if msg.SeqId <= pipe.inputThreshold {
			log.Debugf("%s duplicate data packet on %d (%d)", d.desc(), msg.Session, msg.SeqId)
			return
		}

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
		}
	} else {
		log.Debugf("unknown pipe: %d", msg.Session)
		var errM Message
		errM.Type = PIPE_UNKNOWN
		errM.Session = msg.Session

		d.sender.SendData(ctx, hdr.Sender, d.peerProto, &errM)
	}
}

func (d *DataHandler) ackPipeData(ctx context.Context, hdr *pb.Header, msg *Message) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	if pipe, ok := d.pipes[mkpipeKey(hdr.Sender, msg.Session)]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		pipe.recvThreshold = msg.SeqId
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

			close(pipe.message)

			pipe.err = ErrClosed
			pipe.closed = true
		}
	} else if msg.PipeName != "" && len(msg.Data) > 0 {
		if lp, ok := d.listening[msg.PipeName]; ok {
			pipe := &Pipe{
				other:     hdr.Sender,
				handler:   d,
				session:   msg.Session,
				message:   make(chan pipeMessage, d.PipeBacklog),
				closed:    true,
				nextSeqId: 1,
			}

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
