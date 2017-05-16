package transport

import (
	"context"
	"crypto/rand"
	"errors"
	"net"

	"github.com/evanphx/mesh"
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
	StaticKey() noise.DHKey
}

type Validator interface {
	Validate(net string, key, cred []byte) bool
	CredsFor(net string) []byte
}

func ListenTCP(pctx context.Context, p Peer, v Validator, addr string) (net.Addr, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return listen(pctx, p, v, l)
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
				go Handshake(pctx, p, v, NewFramer(c))
			}
		}
	}()

	return l.Addr(), nil
}

type Messenger interface {
	Send(context.Context, []byte) error
	Recv(context.Context, []byte) ([]byte, error)
}

func Handshake(ctx context.Context, p Peer, v Validator, tr Messenger) {
	session, err := acceptHS(ctx, p, v, tr)
	if err != nil {
		log.Debugf("Error in accept handshake: %s", err)
		return
	}

	p.AddSession(session)
}

func acceptHS(ctx context.Context, p Peer, v Validator, tr Messenger) (mesh.Session, error) {
	hs := noise.NewHandshakeState(noise.Config{
		CipherSuite:   CipherSuite,
		Random:        RNG,
		Pattern:       noise.HandshakeXX,
		StaticKeypair: p.StaticKey(),
		Prologue:      Prologue,
	})

	log.Printf("performing accept handshake")

	buf, err := tr.Recv(ctx, make([]byte, 256))
	if err != nil {
		return nil, err
	}

	bnetName, _, _, err := hs.ReadMessage(nil, buf)
	if err != nil {
		return nil, err
	}

	msg, _, _ := hs.WriteMessage(nil, v.CredsFor(string(bnetName)))

	err = tr.Send(ctx, msg)
	if err != nil {
		return nil, err
	}

	buf, err = tr.Recv(ctx, buf)
	if err != nil {
		return nil, err
	}

	rcred, csW, csR, err := hs.ReadMessage(nil, buf)
	if err != nil {
		return nil, err
	}

	rkey := hs.PeerStatic()

	if !v.Validate(string(bnetName), rkey, rcred) {
		return nil, ErrInvalidCred
	}

	sess := &noiseSession{mesh.Identity(rkey), tr, csR, csW}

	return sess, nil
}

func ConnectTCP(ctx context.Context, l Peer, v Validator, host, netName string) error {
	conn, err := net.Dial("tcp", host)
	if err != nil {
		return err
	}

	sess, err := connectHS(ctx, l, netName, v, NewFramer(conn))
	if err != nil {
		conn.Close()
		return err
	}

	l.AddSession(sess)

	return nil
}

func connectHS(ctx context.Context, p Peer, net string, v Validator, tr Messenger) (mesh.Session, error) {
	log.Printf("performing connect handshake")

	hs := noise.NewHandshakeState(noise.Config{
		CipherSuite:   CipherSuite,
		Random:        RNG,
		Pattern:       noise.HandshakeXX,
		Initiator:     true,
		StaticKeypair: p.StaticKey(),
		Prologue:      Prologue,
	})

	msg, _, _ := hs.WriteMessage(nil, []byte(net))

	err := tr.Send(ctx, msg)
	if err != nil {
		return nil, err
	}

	buf, err := tr.Recv(ctx, make([]byte, 256))
	if err != nil {
		return nil, err
	}

	rcred, _, _, err := hs.ReadMessage(nil, buf)
	if err != nil {
		return nil, err
	}

	msg, csR, csW := hs.WriteMessage(nil, v.CredsFor(net))

	err = tr.Send(ctx, msg)
	if err != nil {
		return nil, err
	}

	rkey := hs.PeerStatic()

	if !v.Validate(net, rkey, rcred) {
		return nil, ErrInvalidCred
	}

	sess := &noiseSession{mesh.Identity(rkey), tr, csR, csW}

	return sess, nil
}
