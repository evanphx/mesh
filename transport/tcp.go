package transport

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/crypto"
	"github.com/evanphx/mesh/log"
	"github.com/flynn/noise"
)

var (
	CipherSuite = noise.NewCipherSuite(
		noise.DH25519,
		noise.CipherChaChaPoly,
		noise.HashBLAKE2b)
	RNG      = rand.Reader
	Prologue = []byte("mesh")
)

var (
	ErrInvalidCred = errors.New("invalid credentials")
)

type Peer interface {
	AddSession(sess mesh.Session)
	StaticKey() crypto.DHKey
}

type Validator interface {
	Validate(net string, peer mesh.Identity, cred []byte) bool
	CredsFor(net string) []byte
}

func ListenTCP(pctx context.Context, p Peer, v Validator, addr string) (net.Addr, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return listen(pctx, p, v, l)
}

type closeMonitor struct {
	io.ReadWriteCloser

	mu      sync.Mutex
	onClose []func(context.Context) error
}

func (c *closeMonitor) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var merr error

	for _, f := range c.onClose {
		err := f(ctx)
		if err != nil && merr != nil {
			merr = err
		}
	}

	return merr
}

func (c *closeMonitor) OnClose(f func(context.Context) error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.onClose = append(c.onClose, f)
}

type CloseMonitor interface {
	OnClose(f func(context.Context) error)
}

func listen(pctx context.Context, p Peer, v Validator, l net.Listener) (net.Addr, error) {
	conns := make(chan net.Conn)

	lctx, cancel := context.WithCancel(pctx)

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				cancel()
				return
			}

			conns <- c
		}
	}()

	go func() {
		for {
			select {
			case <-lctx.Done():
				l.Close()
				return
			case c := <-conns:
				mon := &closeMonitor{
					ReadWriteCloser: c,
				}

				go Handshake(pctx, p, v, NewFramer(mon))
			}
		}
	}()

	return l.Addr(), nil
}

type Messenger interface {
	Send(context.Context, []byte) error
	Recv(context.Context, []byte) ([]byte, error)
	Close(context.Context) error
}

func Handshake(ctx context.Context, p Peer, v Validator, tr Messenger) {
	session, err := acceptHS(ctx, p, v, tr)
	if err != nil {
		log.Printf("Error in accept handshake: %s", err)
		return
	}

	p.AddSession(session)
}

func acceptHS(ctx context.Context, p Peer, v Validator, tr Messenger) (mesh.Session, error) {
	hs := crypto.NewXXResponder(p.StaticKey())

	buf, err := tr.Recv(ctx, make([]byte, 256))
	if err != nil {
		return nil, err
	}

	bnetName, err := hs.Start(nil, buf)
	if err != nil {
		return nil, err
	}

	msg := hs.Prime(nil, v.CredsFor(string(bnetName)))

	err = tr.Send(ctx, msg)
	if err != nil {
		return nil, err
	}

	buf, err = tr.Recv(ctx, buf)
	if err != nil {
		return nil, err
	}

	rcred, csW, csR, err := hs.Finish(nil, buf)
	if err != nil {
		return nil, err
	}

	rkey := hs.Peer()

	if !v.Validate(string(bnetName), rkey, rcred) {
		return nil, ErrInvalidCred
	}

	sess := &noiseSession{
		peerIdentity: rkey,
		tr:           tr,
		readCS:       csR,
		writeCS:      csW,
	}

	return sess, nil
}

const RetryTimes = 100

func ConnectTCP(ctx context.Context, l Peer, v Validator, host, netName string) (CloseMonitor, error) {
	var (
		conn net.Conn
		err  error
	)

	for i := 0; i < RetryTimes; i++ {
		conn, err = net.Dial("tcp", host)
		if err == nil {
			break
		}

		log.Printf("Unable to connect to %s, retrying (%s)", host, err)

		time.Sleep(1 * time.Second)
	}

	log.Printf("Connected to %s", host)

	mon := &closeMonitor{
		ReadWriteCloser: conn,
	}

	sess, err := connectHS(ctx, l, netName, v, NewFramer(mon))
	if err != nil {
		conn.Close()
		return nil, err
	}

	l.AddSession(sess)

	return mon, nil
}

func connectHS(ctx context.Context, p Peer, net string, v Validator, tr Messenger) (mesh.Session, error) {
	hs := crypto.NewXXInitiator(p.StaticKey())

	msg := hs.Start(nil, []byte(net))

	err := tr.Send(ctx, msg)
	if err != nil {
		return nil, err
	}

	buf, err := tr.Recv(ctx, make([]byte, 256))
	if err != nil {
		return nil, err
	}

	rcred, err := hs.Prime(nil, buf)
	if err != nil {
		return nil, err
	}

	msg, csR, csW := hs.Finish(nil, v.CredsFor(net))

	err = tr.Send(ctx, msg)
	if err != nil {
		return nil, err
	}

	rkey := hs.Peer()

	if !v.Validate(net, rkey, rcred) {
		return nil, ErrInvalidCred
	}

	sess := &noiseSession{
		peerIdentity: rkey,
		tr:           tr,
		readCS:       csR,
		writeCS:      csW,
	}

	return sess, nil
}
