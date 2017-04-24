package peer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dchest/siphash"
	"github.com/evanphx/mesh/log"
	"github.com/flynn/noise"
)

func (p *Peer) sessionId(dest Identity) uint64 {
	sh := siphash.New(p.identityKey.Private)
	sh.Write(dest)

	sess := atomic.AddUint64(p.nextSession, 1)
	binary.Write(sh, binary.BigEndian, sess)

	// Adding the current time will prevent us from being confused
	// by messages for sessions that occured before this peer
	// was restarted.
	binary.Write(sh, binary.BigEndian, time.Now().UnixNano())

	return sh.Sum64()
}

type Pipe struct {
	peer    *Peer
	pending chan struct{}
	lazy    bool
	other   Identity
	session uint64
	closed  bool
	err     error
	service string

	message chan []byte

	lock sync.Mutex

	hs       *noise.HandshakeState
	csr, csw *noise.CipherState
}

type ListenPipe struct {
	name  string
	peer  *Peer
	adver *Advertisement

	newPipes chan *Pipe

	lock sync.Mutex

	err error
}

func (l *ListenPipe) Accept(ctx context.Context) (*Pipe, error) {
	select {
	case pipe, ok := <-l.newPipes:
		if !ok {
			return nil, l.err
		}

		return pipe, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (l *ListenPipe) Close() error {
	if l.err != nil {
		return l.err
	}

	l.peer.pipeLock.Lock()
	defer l.peer.pipeLock.Unlock()

	delete(l.peer.listening, l.name)

	if l.adver != nil {
		l.peer.opChan <- removeAdver{l.adver}
	}

	l.err = ErrClosed
	close(l.newPipes)

	return nil
}

func (p *Peer) ListenPipe(name string) (*ListenPipe, error) {
	p.pipeLock.Lock()
	defer p.pipeLock.Unlock()

	lp := &ListenPipe{
		name:     name,
		peer:     p,
		newPipes: make(chan *Pipe, p.PipeBacklog),
	}

	p.listening[name] = lp

	log.Debugf("listen pipe created: %s", name)

	if name[0] != ':' {
		adver := &Advertisement{
			Owner: p.Identity(),
			Pipe:  name,
		}

		lp.adver = adver

		err := p.Advertise(adver)
		if err != nil {
			return nil, err
		}
	}

	return lp, nil
}

var ErrNoName = errors.New("no pipe name specified")

func (p *Peer) Listen(adver *Advertisement) (*ListenPipe, error) {
	p.pipeLock.Lock()
	defer p.pipeLock.Unlock()

	name := adver.Pipe

	if name == "" {
		return nil, ErrNoName
	}

	lp := &ListenPipe{
		name:     name,
		peer:     p,
		adver:    adver,
		newPipes: make(chan *Pipe, p.PipeBacklog),
	}

	p.listening[name] = lp

	log.Debugf("listen pipe created: %s", name)

	err := p.Advertise(adver)
	if err != nil {
		return nil, err
	}

	return lp, nil

}

func mkpipeKey(id Identity, ses uint64) pipeKey {
	return pipeKey{id.String(), ses}
}

func (p *Peer) makePendingPipe(dest Identity) *Pipe {
	p.pipeLock.Lock()
	defer p.pipeLock.Unlock()

	id := p.sessionId(dest)

	pipe := &Pipe{
		other:   dest,
		peer:    p,
		session: id,
		pending: make(chan struct{}),
		message: make(chan []byte, p.PipeBacklog),
	}

	p.pipes[mkpipeKey(dest, id)] = pipe

	return pipe
}

func (p *Peer) ConnectPipe(ctx context.Context, dst Identity, name string) (*Pipe, error) {
	pipe := p.makePendingPipe(dst)

	log.Debugf("%s open pipe to %s:%s", p.Desc(), dst.Short(), name)

	var hdr Header
	hdr.Destination = dst
	hdr.Type = PIPE_OPEN
	hdr.Session = pipe.session
	hdr.PipeName = name
	hdr.Encrypted = true

	pipe.hs = noise.NewHandshakeState(noise.Config{
		CipherSuite:   CipherSuite,
		Random:        RNG,
		Pattern:       noise.HandshakeKK,
		Initiator:     true,
		StaticKeypair: p.identityKey,
		PeerStatic:    dst,
	})

	hdr.Body, _, _ = pipe.hs.WriteMessage(nil, nil)

	log.Debugf("%s opening sync encrypted pipe", p.Desc())

	err := p.send(&hdr)
	if err != nil {
		return nil, err
	}

	select {
	case <-pipe.pending:
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		if pipe.closed {
			return nil, pipe.err
		}

		return pipe, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Peer) LazyConnectPipe(ctx context.Context, dst Identity, name string) (*Pipe, error) {
	p.pipeLock.Lock()
	defer p.pipeLock.Unlock()

	log.Debugf("%s lazy pipe to %s:%s", p.Desc(), dst.Short(), name)

	id := p.sessionId(dst)

	pipe := &Pipe{
		other:   dst,
		peer:    p,
		session: id,
		service: name,
		lazy:    true,
		message: make(chan []byte, p.PipeBacklog),
	}

	p.pipes[mkpipeKey(dst, id)] = pipe

	return pipe, nil
}

func (p *Peer) Connect(ctx context.Context, sel *PipeSelector) (*Pipe, error) {
	peer, name, err := p.lookupSelector(sel)
	if err != nil {
		return nil, err
	}

	return p.ConnectPipe(ctx, peer, name)
}

var (
	ErrUnknownPipe = errors.New("unknown pipe endpoint")
	ErrClosed      = errors.New("closed pipe")
)

func (p *Peer) setPipeOpened(hdr *Header) {
	p.pipeLock.Lock()
	defer p.pipeLock.Unlock()

	if pipe, ok := p.pipes[mkpipeKey(hdr.Sender, hdr.Session)]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		if hdr.Encrypted {
			if pipe.hs == nil {
				panic(fmt.Sprintf("%s encrypted opened but handshake not setup: %d", p.Desc(), hdr.Session))
			}

			_, csr, csw, err := pipe.hs.ReadMessage(nil, hdr.Body)
			if err != nil {
				log.Printf("%s error finishing encryption handshake: %s", err)
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
			p.Desc(), Identity(hdr.Sender).Short(), hdr.Session)
	}
}

func (p *Peer) setPipeUnknown(hdr *Header) {
	p.pipeLock.Lock()
	defer p.pipeLock.Unlock()

	if pipe, ok := p.pipes[mkpipeKey(hdr.Sender, hdr.Session)]; ok {
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
			p.Desc(), Identity(hdr.Sender).Short(), hdr.Session)
	}
}

func (p *Peer) newPipeRequest(hdr *Header) {
	p.pipeLock.Lock()
	defer p.pipeLock.Unlock()

	log.Debugf("requesting new pipe")

	if lp, ok := p.listening[hdr.PipeName]; ok {
		pipe := &Pipe{
			other:   Identity(hdr.Sender),
			peer:    p,
			session: hdr.Session,
			message: make(chan []byte, p.PipeBacklog),
		}

		_, ok := p.pipes[mkpipeKey(pipe.other, hdr.Session)]
		if ok {
			panic("already using pipe!")
		}

		p.pipes[mkpipeKey(pipe.other, hdr.Session)] = pipe

		var retPayload []byte

		if hdr.Encrypted {
			hs := noise.NewHandshakeState(noise.Config{
				CipherSuite:   CipherSuite,
				Random:        RNG,
				Pattern:       noise.HandshakeKK,
				StaticKeypair: p.identityKey,
				PeerStatic:    hdr.Sender,
			})

			out, _, _, err := hs.ReadMessage(nil, hdr.Body)
			if err != nil {
				log.Printf("%s error decrypting responder handshake: %s", p.Desc(), err)

				err := p.sendMessage(hdr.Sender, PIPE_UNKNOWN, hdr.Session, nil)
				if err != nil {
					log.Debugf("Error sending PIPE_UNKNOWN: %s", err)
				}

				return
			}

			// Marshal and encrypt the return message to complete
			// the handshake

			enc, csw, csr := hs.WriteMessage(nil, nil)
			retPayload = enc

			pipe.csr = csr
			pipe.csw = csw

			hdr.Body = out
		}

		// TODO implement a backlog via a buffered channel and a
		// nonblocking send here
		lp.newPipes <- pipe

		// This is to support 0-RTT connects
		if len(hdr.Body) > 0 {
			log.Debugf("detected 0-rtt pipe open: %d bytes", len(hdr.Body))
			pipe.message <- hdr.Body
		}

		var ret Header
		ret.Destination = hdr.Sender
		ret.Sender = p.Identity()
		ret.Type = PIPE_OPENED
		ret.Session = hdr.Session

		if hdr.Encrypted {
			log.Debugf("%s responding with encrypted PIPE_OPENED to %s (%d)",
				p.Desc(), Identity(hdr.Sender).Short(), hdr.Session)
			ret.Encrypted = true
			ret.Body = retPayload
		} else {
			log.Debugf("%s responding with unencrypted PIPE_OPENED %s (%d)",
				p.Desc(), Identity(hdr.Sender).Short(), hdr.Session)
		}

		err := p.send(&ret)
		if err != nil {
			log.Debugf("Error sending PIPE_OPENED: %s", err)
		}
	} else {
		log.Debugf("%s unknown pipe requested: %s", p.Identity(), hdr.PipeName)

		err := p.sendMessage(hdr.Sender, PIPE_UNKNOWN, hdr.Session, nil)
		if err != nil {
			log.Debugf("Error sending PIPE_UNKNOWN: %s", err)
		}
	}
}

func (p *Peer) newPipeData(hdr *Header) {
	p.pipeLock.Lock()
	defer p.pipeLock.Unlock()

	if pipe, ok := p.pipes[mkpipeKey(hdr.Sender, hdr.Session)]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		if pipe.closed {
			log.Debugf("injected data to closed pipe: %d", hdr.Session)
			p.sendMessage(hdr.Sender, PIPE_CLOSE, hdr.Session, nil)
		} else {
			log.Debugf("inject pipe data: %s", hdr.Session)

			if hdr.Encrypted {
				if pipe.csr != nil {
					data, err := pipe.csr.Decrypt(nil, nil, hdr.Body)
					if err != nil {
						log.Printf("%s error decrypting pipe data: %s", err)
						return
					}

					hdr.Body = data
				}
			}

			pipe.message <- hdr.Body
		}
	} else {
		log.Debugf("unknown pipe: %d", hdr.Session)
		p.sendMessage(hdr.Sender, PIPE_UNKNOWN, hdr.Session, nil)
	}
}

func (p *Peer) setPipeClosed(hdr *Header) {
	p.pipeLock.Lock()
	defer p.pipeLock.Unlock()

	key := mkpipeKey(hdr.Sender, hdr.Session)

	if pipe, ok := p.pipes[key]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		if !pipe.closed {
			delete(p.pipes, key)

			// This is to support a final message without having to transmit
			// a close as a second message
			if len(hdr.Body) > 0 {
				pipe.message <- hdr.Body
			}

			close(pipe.message)

			pipe.err = ErrClosed
			pipe.closed = true
		}
	} else if hdr.PipeName != "" && len(hdr.Body) > 0 {
		if lp, ok := p.listening[hdr.PipeName]; ok {
			pipe := &Pipe{
				other:   Identity(hdr.Sender),
				peer:    p,
				session: hdr.Session,
				message: make(chan []byte, p.PipeBacklog),
				closed:  true,
			}

			// We don't even bother to register this pipe, no more
			// data is available on it.
			log.Debugf("noreply zero-rtt pipe created")

			// TODO implement a backlog via a buffered channel and a
			// nonblocking send here
			lp.newPipes <- pipe

			// This is to support 0-RTT connects
			pipe.message <- hdr.Body
		} else {
			log.Debugf("unknown service '%s'in zero-rtt noreply pipe: %d",
				hdr.Session, hdr.PipeName)
		}
	} else {
		log.Printf("%s unknown pipe reported as closed? %s.%d",
			p.Desc(), Identity(hdr.Sender).Short(), hdr.Session)
	}
}

func (p *Pipe) Send(msg []byte) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.closed {
		return p.err
	}

	var hdr Header
	hdr.Destination = p.other
	hdr.Type = PIPE_OPEN
	hdr.Session = p.session

	hdr.Body = msg

	if p.lazy {
		hs := noise.NewHandshakeState(noise.Config{
			CipherSuite:   CipherSuite,
			Random:        RNG,
			Pattern:       noise.HandshakeKK,
			Initiator:     true,
			StaticKeypair: p.peer.identityKey,
			PeerStatic:    p.other,
		})

		out, _, _ := hs.WriteMessage(nil, msg)

		p.hs = hs
		hdr.Body = out
		hdr.Encrypted = true
		hdr.Type = PIPE_OPEN
		hdr.PipeName = p.service

		log.Debugf("%s initialing with encrypted pipe", p.peer.Desc())

		p.lazy = false
	} else {
		if p.csw != nil {
			hdr.Encrypted = true
			hdr.Body = p.csw.Encrypt(nil, nil, msg)
		}

		hdr.Type = PIPE_DATA
	}

	return p.peer.send(&hdr)
}

func (p *Pipe) SendFinal(msg []byte) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.closed {
		return p.err
	}

	close(p.message)

	p.closed = true
	p.err = ErrClosed

	var hdr Header
	hdr.Destination = p.other
	hdr.Type = PIPE_CLOSE
	hdr.Session = p.session
	hdr.Body = msg

	if p.lazy {
		p.lazy = false
		hdr.PipeName = p.service

		hs := noise.NewHandshakeState(noise.Config{
			CipherSuite:   CipherSuite,
			Random:        RNG,
			Pattern:       noise.HandshakeKK,
			Initiator:     true,
			StaticKeypair: p.peer.identityKey,
			PeerStatic:    p.other,
		})

		out, _, _ := hs.WriteMessage(nil, msg)

		hdr.Body = out
		hdr.Encrypted = true

		p.hs = hs
	}

	return p.peer.send(&hdr)
}

func (p *Pipe) Recv(ctx context.Context) ([]byte, error) {
	select {
	case m, ok := <-p.message:
		if !ok {
			return nil, p.err
		}

		return m, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Pipe) Close() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.closed {
		return nil
	}

	p.peer.pipeLock.Lock()

	delete(p.peer.pipes, mkpipeKey(p.other, p.session))

	p.peer.pipeLock.Unlock()

	close(p.message)

	p.err = ErrClosed
	p.closed = true

	return p.peer.sendMessage(p.other, PIPE_CLOSE, p.session, nil)
}
