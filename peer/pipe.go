package peer

import (
	"context"
	"errors"
	"sync"

	"github.com/evanphx/mesh/log"
)

func (p *Peer) newSessionId() int64 {
	p.nextSession++
	return p.nextSession
}

type Pipe struct {
	peer    *Peer
	pending chan struct{}
	other   Identity
	session int64
	closed  bool
	err     error

	message chan []byte

	lock sync.Mutex
}

type ListenPipe struct {
	name string
	peer *Peer

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

	l.peer.lock.Lock()
	defer l.peer.lock.Unlock()

	delete(l.peer.listening, l.name)

	l.err = ErrClosed
	close(l.newPipes)

	return nil
}

func (p *Peer) ListenPipe(name string) (*ListenPipe, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	lp := &ListenPipe{
		name:     name,
		peer:     p,
		newPipes: make(chan *Pipe, p.PipeBacklog),
	}

	p.listening[name] = lp

	return lp, nil
}

func (p *Peer) makePendingPipe(dest Identity) *Pipe {
	p.lock.Lock()
	defer p.lock.Unlock()

	id := p.newSessionId()

	pipe := &Pipe{
		other:   dest,
		peer:    p,
		session: id,
		pending: make(chan struct{}),
	}

	p.pipes[id] = pipe

	return pipe
}

func (p *Peer) ConnectPipe(ctx context.Context, dst Identity, name string) (*Pipe, error) {
	pipe := p.makePendingPipe(dst)

	err := p.sendMessage(dst, PIPE_OPEN, pipe.session, nil)
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

var (
	ErrUnknownPipe = errors.New("unknown pipe endpoint")
	ErrClosed      = errors.New("closed pipe")
)

func (p *Peer) setPipeOpened(hdr *Header) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if pipe, ok := p.pipes[hdr.Session]; ok {
		if pipe.pending != nil {
			close(pipe.pending)
		}
	}
}

func (p *Peer) setPipeUnknown(hdr *Header) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if pipe, ok := p.pipes[hdr.Session]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		pipe.err = ErrUnknownPipe
		pipe.closed = true

		if pipe.pending != nil {
			close(pipe.pending)
		}
	}
}

func (p *Peer) newPipeRequest(hdr *Header) {
	p.lock.Lock()
	defer p.lock.Unlock()

	log.Debugf("requesting new pipe")

	if lp, ok := p.listening[hdr.PipeName]; ok {
		pipe := &Pipe{
			other:   Identity(hdr.Sender),
			peer:    p,
			session: hdr.Session,
			message: make(chan []byte, p.PipeBacklog),
		}

		p.pipes[hdr.Session] = pipe

		log.Debugf("sending new pipe to listener")

		// TODO implement a backlog via a buffered channel and a
		// nonblocking send here
		lp.newPipes <- pipe

		log.Debugf("sending response to %s", Identity(hdr.Sender))

		err := p.sendMessage(hdr.Sender, PIPE_OPENED, hdr.Session, nil)
		if err != nil {
			log.Debugf("Error sending PIPE_OPENED: %s", err)
		}

		log.Debugf("sent response")
	}
}

func (p *Peer) newPipeData(hdr *Header) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if pipe, ok := p.pipes[hdr.Session]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		log.Debugf("inject pipe data: %s", hdr.Session)

		if pipe.closed {
			p.sendMessage(hdr.Sender, PIPE_CLOSE, hdr.Session, nil)
		} else {
			pipe.message <- hdr.Body
		}
	} else {
		log.Debugf("unknown pipe: %d", hdr.Session)
		p.sendMessage(hdr.Sender, PIPE_UNKNOWN, hdr.Session, nil)
	}
}

func (p *Peer) setPipeClosed(hdr *Header) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if pipe, ok := p.pipes[hdr.Session]; ok {
		pipe.lock.Lock()
		defer pipe.lock.Unlock()

		pipe.err = ErrClosed
		pipe.closed = true
	}
}

func (p *Pipe) Send(msg []byte) error {
	if p.closed {
		return p.err
	}

	return p.peer.sendMessage(p.other, PIPE_DATA, p.session, msg)
}

func (p *Pipe) Recv(ctx context.Context) ([]byte, error) {
	if p.closed {
		return nil, p.err
	}

	select {
	case m := <-p.message:
		return m, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Pipe) Close() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.closed {
		return p.err
	}

	p.err = ErrClosed
	p.closed = true

	return p.peer.sendMessage(p.other, PIPE_CLOSE, p.session, nil)
}
