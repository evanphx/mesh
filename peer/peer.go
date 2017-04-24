package peer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ed25519"

	"github.com/evanphx/mesh/auth"
	"github.com/evanphx/mesh/grpc"
	"github.com/evanphx/mesh/router"
	"github.com/flynn/noise"
	"github.com/pkg/errors"
)

var (
	ErrInvalidCred    = errors.New("invalid credentials provided")
	ErrUnknownNetwork = errors.New("unknown network requested")
)

type Network struct {
	authPub ed25519.PublicKey
	cred    []byte
}

type Neighbor struct {
	Id Identity
	tr ByteTransport
}

const DefaultPipeBacklog = 10

type pipeKey struct {
	ep string
	id uint64
}

type Peer struct {
	Name        string
	PipeBacklog int

	identityKey noise.DHKey

	networks map[string]*Network

	lock   sync.Mutex
	router *router.Router

	pipeLock  sync.Mutex
	listening map[string]*ListenPipe
	pipes     map[pipeKey]*Pipe

	neighLock sync.Mutex
	neighbors map[string]*Neighbor

	nextSession *uint64

	rpcServer *grpc.Server

	lifetime context.Context
	shutdown func()
	opChan   chan operation

	pingLock sync.Mutex
	pings    map[uint64]chan struct{}

	adverLock  sync.Mutex
	advers     map[string]*localAdver
	selfAdvers map[string]*Advertisement
}

func (p *Peer) Desc() string {
	if p.Name == "" {
		return p.Identity().Short()
	}

	return fmt.Sprintf("%s:%s", p.Name, p.Identity().Short())
}

func InitNewPeer() (*Peer, error) {
	id := CipherSuite.GenerateKeypair(RNG)

	peer := &Peer{
		PipeBacklog: DefaultPipeBacklog,
		identityKey: id,
		router:      router.NewRouter(),
		neighbors:   make(map[string]*Neighbor),
		networks:    make(map[string]*Network),
		listening:   make(map[string]*ListenPipe),
		pipes:       make(map[pipeKey]*Pipe),
		rpcServer:   grpc.NewServer(),
		opChan:      make(chan operation),
		pings:       make(map[uint64]chan struct{}),
		nextSession: new(uint64),
		advers:      make(map[string]*localAdver),
		selfAdvers:  make(map[string]*Advertisement),
	}

	peer.run()

	return peer, nil
}

type dhKey struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

type networkState struct {
	PublicKey  string `json:"public_key"`
	Credential string `json:"credential"`
}

type peerState struct {
	IdentityKey dhKey                    `json:"identity_key"`
	Networks    map[string]*networkState `json:"networks"`
}

const ConfigFile = "mesh.json"

func InitPeerFromDir(dir string) (*Peer, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}

	var ps peerState

	err = json.NewDecoder(f).Decode(&ps)
	if err != nil {
		return nil, err
	}

	peer := &Peer{
		PipeBacklog: DefaultPipeBacklog,
		router:      router.NewRouter(),
		neighbors:   make(map[string]*Neighbor),
		networks:    make(map[string]*Network),
		listening:   make(map[string]*ListenPipe),
		pipes:       make(map[pipeKey]*Pipe),
		rpcServer:   grpc.NewServer(),
		opChan:      make(chan operation),
	}

	peer.identityKey.Public, err = hex.DecodeString(ps.IdentityKey.Public)
	if err != nil {
		return nil, err
	}

	peer.identityKey.Private, err = hex.DecodeString(ps.IdentityKey.Private)
	if err != nil {
		return nil, err
	}

	peer.networks = make(map[string]*Network)

	for name, ns := range ps.Networks {
		authPub, err := hex.DecodeString(ns.PublicKey)
		if err != nil {
			return nil, err
		}

		cred, err := hex.DecodeString(ns.Credential)
		if err != nil {
			return nil, err
		}

		peer.networks[name] = &Network{
			authPub: ed25519.PublicKey(authPub),
			cred:    cred,
		}
	}

	return peer, nil
}

func (p *Peer) SaveToDir(dir string) error {
	var ps peerState

	ps.IdentityKey.Public = hex.EncodeToString(p.identityKey.Public)
	ps.IdentityKey.Private = hex.EncodeToString(p.identityKey.Private)

	ps.Networks = make(map[string]*networkState)

	for name, net := range p.networks {
		ps.Networks[name] = &networkState{
			PublicKey:  hex.EncodeToString(net.authPub),
			Credential: hex.EncodeToString(net.cred),
		}
	}

	f, err := os.Create(filepath.Join(dir, ConfigFile))
	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewEncoder(f).Encode(&ps)
}

func (p *Peer) AddNeighbor(id Identity, tr ByteTransport) {
	p.opChan <- neighborAdd{id, tr}
}

func (p *Peer) AddRoute(neigh, dest Identity) {
	var update RouteUpdate
	update.Neighbor = neigh
	update.Routes = append(update.Routes, &Route{
		Destination: dest,
		Weight:      1,
	})

	p.opChan <- routeUpdate{update: &update, prop: true}
}

func (p *Peer) Identity() Identity {
	return Identity(p.identityKey.Public)
}

func (p *Peer) Authorize(a *auth.Authorizer) error {
	_, sig := a.Sign(p.identityKey.Public)
	p.networks[a.Name()] = &Network{
		authPub: a.PublicKey(),
		cred:    sig,
	}
	return nil
}

func (n *Network) validateCred(key, rcred []byte) bool {
	return ed25519.Verify(n.authPub, key, rcred)
}

type ByteTransport interface {
	Send([]byte) error
	Recv([]byte) ([]byte, error)
}

type Session interface {
	PeerIdentity() Identity

	Send(msg []byte) error
	Recv(out []byte) ([]byte, error)

	Encrypt(msg, out []byte) []byte
	Decrypt(msg, out []byte) ([]byte, error)
}

type noiseSession struct {
	peerIdentity Identity

	tr ByteTransport

	readCS  *noise.CipherState
	writeCS *noise.CipherState
}

func (n *noiseSession) PeerIdentity() Identity {
	return n.peerIdentity
}

func (n *noiseSession) Encrypt(msg, out []byte) []byte {
	return n.writeCS.Encrypt(out, nil, msg)
}

func (n *noiseSession) Decrypt(msg, out []byte) ([]byte, error) {
	return n.readCS.Decrypt(out, nil, msg)
}

func (n *noiseSession) Send(msg []byte) error {
	return n.tr.Send(n.writeCS.Encrypt(nil, nil, msg))
}

func (n *noiseSession) Recv(out []byte) ([]byte, error) {
	buf, err := n.tr.Recv(out)
	if err != nil {
		return nil, err
	}

	return n.readCS.Decrypt(out, nil, buf)
}

func (p *Peer) BeginHandshake(netName string, tr ByteTransport) (Session, error) {
	hs := noise.NewHandshakeState(noise.Config{
		CipherSuite:   CipherSuite,
		Random:        RNG,
		Pattern:       noise.HandshakeXX,
		Initiator:     true,
		StaticKeypair: p.identityKey,
		Prologue:      Prologue,
	})

	net, ok := p.networks[netName]
	if !ok {
		return nil, ErrUnknownNetwork
	}

	msg, _, _ := hs.WriteMessage(nil, []byte(netName))

	err := tr.Send(msg)
	if err != nil {
		return nil, err
	}

	buf, err := tr.Recv(make([]byte, 256))
	if err != nil {
		return nil, err
	}

	rcred, _, _, err := hs.ReadMessage(nil, buf)
	if err != nil {
		return nil, err
	}

	msg, csR, csW := hs.WriteMessage(nil, net.cred)

	err = tr.Send(msg)
	if err != nil {
		return nil, err
	}

	rkey := hs.PeerStatic()

	if !net.validateCred(rkey, rcred) {
		return nil, ErrInvalidCred
	}

	sess := &noiseSession{Identity(rkey), tr, csR, csW}

	p.AddNeighbor(sess.peerIdentity, sess)

	return sess, nil
}

func (p *Peer) WaitHandshake(tr ByteTransport) (Session, error) {
	hs := noise.NewHandshakeState(noise.Config{
		CipherSuite:   CipherSuite,
		Random:        RNG,
		Pattern:       noise.HandshakeXX,
		StaticKeypair: p.identityKey,
		Prologue:      Prologue,
	})

	buf, err := tr.Recv(make([]byte, 256))
	if err != nil {
		return nil, err
	}

	bnetName, _, _, err := hs.ReadMessage(nil, buf)
	if err != nil {
		return nil, err
	}

	net, ok := p.networks[string(bnetName)]
	if !ok {
		return nil, ErrUnknownNetwork
	}

	msg, _, _ := hs.WriteMessage(nil, net.cred)

	err = tr.Send(msg)
	if err != nil {
		return nil, err
	}

	buf, err = tr.Recv(buf)
	if err != nil {
		return nil, err
	}

	rcred, csW, csR, err := hs.ReadMessage(nil, buf)
	if err != nil {
		return nil, err
	}

	rkey := hs.PeerStatic()

	if !net.validateCred(rkey, rcred) {
		return nil, ErrInvalidCred
	}

	sess := &noiseSession{Identity(rkey), tr, csR, csW}

	p.AddNeighbor(sess.peerIdentity, sess)

	return sess, nil
}

func (p *Peer) sendMessage(dst Identity, t Header_Type, s uint64, body []byte) error {
	var hdr Header

	hdr.Destination = dst
	hdr.Sender = p.identityKey.Public
	hdr.Type = t
	hdr.Session = s
	hdr.Body = body

	data, err := hdr.Marshal()
	if err != nil {
		return err
	}

	return p.forward(&hdr, data)
}

func (p *Peer) send(hdr *Header) error {
	hdr.Sender = p.identityKey.Public

	data, err := hdr.Marshal()
	if err != nil {
		return err
	}

	return p.forward(hdr, data)
}

func (p *Peer) run() {
	p.lifetime, p.shutdown = context.WithCancel(context.Background())

	go p.ListenForRPC(p.lifetime)
	go p.handleOperations(p.lifetime)
}

func (p *Peer) Shutdown() {
	p.shutdown()
}

func (p *Peer) AttachPeer(id Identity, tr ByteTransport) {
	go p.Monitor(p.lifetime, id, tr)

	p.AddNeighbor(id, tr)
}

func (p *Peer) Ping(ctx context.Context, id Identity) (time.Duration, error) {
	sess := p.sessionId(id)

	c := make(chan struct{})

	p.pingLock.Lock()
	p.pings[sess] = c
	p.pingLock.Unlock()

	var hdr Header

	hdr.Sender = p.Identity()
	hdr.Destination = id
	hdr.Type = PING
	hdr.Session = sess

	sent := time.Now()

	err := p.send(&hdr)
	if err != nil {
		return 0, err
	}

	var ret time.Duration

	select {
	case <-c:
		ret = time.Since(sent)
		err = nil
	case <-ctx.Done():
		err = ctx.Err()
	}

	p.pingLock.Lock()
	delete(p.pings, sess)
	p.pingLock.Unlock()

	// Do a quick drain of c in case the ping came in after we
	// timed out but before we got to the lock.
	select {
	case <-c:
	default:
	}

	return ret, err
}

func (p *Peer) processPong(hdr *Header) {
	log.Printf("%s process pong", p.Desc())

	p.pingLock.Lock()

	c, ok := p.pings[hdr.Session]

	p.pingLock.Unlock()

	if ok {
		select {
		case c <- struct{}{}:
			// all good
		default:
			// omg super fast response
			go func() {
				c <- struct{}{}
			}()
		}
	}
}
