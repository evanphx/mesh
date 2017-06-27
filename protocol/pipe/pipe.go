package pipe

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/crypto"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
)

type pipeMessage struct {
	hdr  *pb.Header
	msg  *Message
	data []byte
}

const ResendInterval = 500 * time.Millisecond

type unackedMessage struct {
	msg  *Message
	next time.Time
}

type Pipe struct {
	global  context.Context
	handler *DataHandler
	pending chan struct{}
	lazy    bool
	other   mesh.Identity
	session uint64
	closed  bool
	err     error
	service string

	lifetime       context.Context
	lifetimeCancel func()

	message chan pipeMessage

	lock sync.Mutex
	cond sync.Cond

	nextSeqId uint64

	inputThreshold uint64
	recvThreshold  uint64

	window      []pipeMessage
	windowUsed  uint64
	windowStart uint64

	ks       *crypto.KKInitState
	csr, csw crypto.CipherState

	resendInterval  time.Duration
	ackBacklog      int
	unackedMessages []unackedMessage
}

func (p *Pipe) init() {
	p.lifetime, p.lifetimeCancel = context.WithCancel(context.Background())
	p.cond.L = &p.lock
	p.resendInterval = ResendInterval

	go p.resendLoop()
}

func (p *Pipe) PeerIdentity() mesh.Identity {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.other
}

type ListenPipe struct {
	name    string
	handler *DataHandler
	adver   *pb.Advertisement

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

	l.handler.pipeLock.Lock()
	defer l.handler.pipeLock.Unlock()

	delete(l.handler.listening, l.name)

	if l.adver != nil {
		if l.handler.resolver != nil {
			l.handler.resolver.RemoveAdvertisement(l.adver)
		}
	}

	l.err = ErrClosed
	close(l.newPipes)

	return nil
}

func (d *DataHandler) ListenPipe(name string) (*ListenPipe, error) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	lp := &ListenPipe{
		name:     name,
		handler:  d,
		newPipes: make(chan *Pipe, d.PipeBacklog),
	}

	d.listening[name] = lp

	log.Debugf("listen pipe created: %s", name)

	return lp, nil
}

var ErrNoName = errors.New("no pipe name specified")

func (d *DataHandler) Listen(adver *pb.Advertisement) (*ListenPipe, error) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	name := adver.Pipe

	if name == "" {
		return nil, ErrNoName
	}

	lp := &ListenPipe{
		name:     name,
		handler:  d,
		adver:    adver,
		newPipes: make(chan *Pipe, d.PipeBacklog),
	}

	d.listening[name] = lp

	if d.resolver != nil {
		d.resolver.Advertise(adver)
	}

	log.Debugf("listen pipe created: %s", name)

	return lp, nil
}

func mkpipeKey(id mesh.Identity, ses uint64) pipeKey {
	return pipeKey{id.String(), ses}
}

func (d *DataHandler) makePendingPipe(dest mesh.Identity) *Pipe {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	id := d.sessionId(dest)

	pipe := &Pipe{
		other:      dest,
		handler:    d,
		session:    id,
		pending:    make(chan struct{}),
		message:    make(chan pipeMessage, d.PipeBacklog),
		nextSeqId:  1,
		ackBacklog: d.AckBacklog,
	}

	pipe.init()

	d.pipes[mkpipeKey(dest, id)] = pipe

	return pipe
}

func (d *DataHandler) ConnectPipe(ctx context.Context, dst mesh.Identity, name string) (*Pipe, error) {
	pipe := d.makePendingPipe(dst)

	pipe.lock.Lock()

	log.Debugf("%s open pipe to %s:%s", d.identityKey.Identity().Short(), dst.Short(), name)

	var msg Message
	msg.Type = PIPE_OPEN
	msg.Session = pipe.session
	msg.PipeName = name
	msg.Encrypted = true

	pipe.ks = crypto.NewKKInitiator(d.identityKey, dst)

	msg.Data = pipe.ks.Start(nil, nil)

	pipe.lock.Unlock()

	log.Debugf("%s opening sync encrypted pipe", d.desc())

	err := d.sender.SendData(ctx, dst, d.peerProto, &msg)
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

func (d *DataHandler) LazyConnectPipe(ctx context.Context, dst mesh.Identity, name string) (*Pipe, error) {
	d.pipeLock.Lock()
	defer d.pipeLock.Unlock()

	log.Debugf("%s lazy pipe to %s:%s", d.desc(), dst.Short(), name)

	id := d.sessionId(dst)

	pipe := &Pipe{
		other:      dst,
		handler:    d,
		session:    id,
		service:    name,
		lazy:       true,
		message:    make(chan pipeMessage, d.PipeBacklog),
		nextSeqId:  1,
		ackBacklog: d.AckBacklog,
	}

	pipe.init()

	d.pipes[mkpipeKey(dst, id)] = pipe

	return pipe, nil
}

func (d *DataHandler) Connect(ctx context.Context, sel *mesh.PipeSelector) (*Pipe, error) {
	peer, name, err := d.resolver.LookupSelector(sel)
	if err != nil {
		return nil, err
	}

	return d.ConnectPipe(ctx, peer, name)
}

var (
	ErrUnknownPipe = errors.New("unknown pipe endpoint")
	ErrClosed      = errors.New("closed pipe")
)

func (p *Pipe) resendLoop() {
	tick := time.NewTicker(p.resendInterval / 2)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			p.resendUnacked(p.lifetime)
		case <-p.lifetime.Done():
			return
		}
	}
}

func (p *Pipe) resendUnacked(ctx context.Context) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if len(p.unackedMessages) == 0 {
		return nil
	}

	var sent int

	log.Debugf("%s considering %d messages for resend to %s", p.handler.desc(), len(p.unackedMessages), p.other.Short())
	defer func() {
		log.Debugf("%s finished with resends, sent %d", p.handler.desc(), sent)
	}()

	now := time.Now()

	for _, uk := range p.unackedMessages {
		if uk.next.Before(now) {
			sent++
			err := p.handler.sender.SendData(ctx, p.other, p.handler.peerProto, uk.msg)
			if err != nil {
				if err.Error() == "no route available" {
					log.Debugf("%s closing unroutable pipe to %s", p.handler.desc(), p.other.Short())
					p.unackedMessages = nil
					go p.handler.closeUnroutablePipe(p)
					return nil
				}

				log.Debugf("%s resend error: %s", p.handler.desc(), err)
				return err
			}

			uk.next = now.Add(p.resendInterval)
		}
	}

	return nil
}

func (p *Pipe) Send(ctx context.Context, data []byte) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.closed {
		return p.err
	}

	for p.blockForAcks() {
		log.Debugf("blocking for ack space")
		p.cond.Wait()
	}

	var msg Message
	msg.Session = p.session
	msg.Data = data
	msg.SeqId = p.nextSeqId

	p.nextSeqId++

	if p.lazy {
		p.ks = crypto.NewKKInitiator(p.handler.identityKey, p.other)

		msg.Data = p.ks.Start(nil, data)
		msg.Encrypted = true
		msg.Type = PIPE_OPEN
		msg.PipeName = p.service

		log.Debugf("%s initialing with encrypted pipe", p.handler.desc())

		p.lazy = false
	} else {
		if p.csw != nil {
			msg.Encrypted = true
			msg.Data = p.csw.Encrypt(nil, nil, data)
		}

		msg.Type = PIPE_DATA
	}

	log.Debugf("%s adding %d to unacked", p.handler.desc(), msg.SeqId)
	p.unackedMessages = append(p.unackedMessages, unackedMessage{&msg, time.Now().Add(p.resendInterval)})

	return p.handler.sender.SendData(ctx, p.other, p.handler.peerProto, &msg)
}

func (p *Pipe) SendFinal(ctx context.Context, data []byte) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.closed {
		return p.err
	}

	close(p.message)

	p.closed = true
	p.err = ErrClosed

	var msg Message
	msg.Type = PIPE_CLOSE
	msg.Session = p.session
	msg.Data = data
	msg.SeqId = p.nextSeqId

	p.nextSeqId++

	if p.lazy {
		p.lazy = false
		msg.PipeName = p.service

		ks := crypto.NewKKInitiator(p.handler.identityKey, p.other)

		out := ks.Start(nil, nil)

		msg.Data = out
		msg.Encrypted = true

		p.ks = ks
	}

	return p.handler.sender.SendData(ctx, p.other, p.handler.peerProto, &msg)
}

var ErrInvalidState = errors.New("encrypted message but no decryption context")

func (p *Pipe) Recv(ctx context.Context) ([]byte, error) {
	select {
	case m, ok := <-p.message:
		if !ok {
			p.lock.Lock()
			err := p.err
			p.lock.Unlock()

			return nil, err
		}

		if m.data != nil {
			return m.data, nil
		} else if m.msg.Encrypted {
			if p.csr == nil {
				return nil, ErrInvalidState
			}

			data, err := p.csr.Decrypt(nil, nil, m.msg.Data)
			if err != nil {
				log.Printf("%s error decrypting pipe data: %s", err)
				return nil, err
			}

			return data, nil
		} else {
			return m.msg.Data, nil
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Pipe) Close(ctx context.Context) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.closed {
		return nil
	}

	if len(p.unackedMessages) == 0 {
		p.handler.pipeLock.Lock()
		delete(p.handler.pipes, mkpipeKey(p.other, p.session))
		p.handler.pipeLock.Unlock()
		p.lifetimeCancel()
	}

	close(p.message)

	p.err = ErrClosed
	p.closed = true

	var msg Message
	msg.Type = PIPE_CLOSE
	msg.Session = p.session

	return p.handler.sender.SendData(ctx, p.other, p.handler.peerProto, &msg)
}

func (p *Pipe) blockForAcks() bool {
	return p.nextSeqId > p.recvThreshold+uint64(p.ackBacklog)
}
