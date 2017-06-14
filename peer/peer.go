package peer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ed25519"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/auth"
	"github.com/evanphx/mesh/crypto"
	"github.com/evanphx/mesh/grpc"
	"github.com/evanphx/mesh/pb"
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
	Id mesh.Identity
	tr ByteTransport
}

const DefaultPipeBacklog = 10

type Peer struct {
	Name        string
	PipeBacklog int

	identityKey crypto.DHKey

	networks map[string]*Network

	lock   sync.Mutex
	router *router.Router

	neighLock sync.Mutex
	neighbors map[string]*Neighbor

	rpcServer *grpc.Server

	lifetime context.Context
	shutdown func()
	opChan   chan operation

	adverLock  sync.Mutex
	advers     map[string]*localAdver
	selfAdvers map[string]*pb.Advertisement
	adverOps   AdvertisementOps
	routeOps   RouteOps

	protoLock sync.RWMutex
	protocols map[int32]Protocol
}

func (p *Peer) Desc() string {
	if p.Name == "" {
		return p.Identity().Short()
	}

	return fmt.Sprintf("%s:%s", p.Name, p.Identity().Short())
}

func InitNewPeer() (*Peer, error) {
	id := crypto.GenerateKey()

	peer := &Peer{
		PipeBacklog: DefaultPipeBacklog,
		identityKey: id,
		router:      router.NewRouter(),
		neighbors:   make(map[string]*Neighbor),
		networks:    make(map[string]*Network),
		rpcServer:   grpc.NewServer(),
		opChan:      make(chan operation),
		advers:      make(map[string]*localAdver),
		selfAdvers:  make(map[string]*pb.Advertisement),
		protocols:   make(map[int32]Protocol),
	}

	peer.run()

	return peer, nil
}

type PeerConfig struct {
	AdvertisementOps AdvertisementOps
	RouteOps         RouteOps
}

func InitNew(cfg PeerConfig) (*Peer, error) {
	id := crypto.GenerateKey()

	peer := &Peer{
		PipeBacklog: DefaultPipeBacklog,
		identityKey: id,
		router:      router.NewRouter(),
		neighbors:   make(map[string]*Neighbor),
		networks:    make(map[string]*Network),
		rpcServer:   grpc.NewServer(),
		opChan:      make(chan operation),
		advers:      make(map[string]*localAdver),
		selfAdvers:  make(map[string]*pb.Advertisement),
		protocols:   make(map[int32]Protocol),
		adverOps:    cfg.AdvertisementOps,
		routeOps:    cfg.RouteOps,
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

func (p *Peer) AddNeighbor(id mesh.Identity, tr ByteTransport) {
	p.opChan <- neighborAdd{id, tr}
}

func (p *Peer) AddSession(sess mesh.Session) {
	p.opChan <- neighborAdd{sess.PeerIdentity(), sess}
}

func (p *Peer) AddRoute(neigh, dest mesh.Identity) {
	var update pb.RouteUpdate
	update.Neighbor = neigh
	update.Routes = append(update.Routes, &pb.Route{
		Destination: dest,
		Weight:      1,
	})

	p.opChan <- routeUpdate{update: &update, prop: true}
}

func (p *Peer) Identity() mesh.Identity {
	return mesh.Identity(p.identityKey.Public)
}

func (p *Peer) StaticKey() crypto.DHKey {
	return p.identityKey
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

func (p *Peer) CredsFor(bnet string) []byte {
	net, ok := p.networks[bnet]
	if !ok {
		return nil
	}

	return net.cred
}

func (p *Peer) Validate(bnet string, key, rcred []byte) bool {
	net, ok := p.networks[bnet]
	if !ok {
		// If we don't have this network and the remote side presented
		// no creds, assume this is an anonymous network
		if len(rcred) == 0 {
			return true
		}

		return false
	}

	return ed25519.Verify(net.authPub, key, rcred)
}

type ByteTransport interface {
	Send(context.Context, []byte) error
	Recv(context.Context, []byte) ([]byte, error)
}

type Session interface {
	PeerIdentity() mesh.Identity

	Send(ctx context.Context, msg []byte) error
	Recv(ctx context.Context, out []byte) ([]byte, error)

	Encrypt(msg, out []byte) []byte
	Decrypt(msg, out []byte) ([]byte, error)
}

type noiseSession struct {
	peerIdentity mesh.Identity

	tr ByteTransport

	readCS  *noise.CipherState
	writeCS *noise.CipherState
}

func (n *noiseSession) PeerIdentity() mesh.Identity {
	return n.peerIdentity
}

func (n *noiseSession) Encrypt(msg, out []byte) []byte {
	return n.writeCS.Encrypt(out, nil, msg)
}

func (n *noiseSession) Decrypt(msg, out []byte) ([]byte, error) {
	return n.readCS.Decrypt(out, nil, msg)
}

func (n *noiseSession) Send(ctx context.Context, msg []byte) error {
	return n.tr.Send(ctx, n.writeCS.Encrypt(nil, nil, msg))
}

func (n *noiseSession) Recv(ctx context.Context, out []byte) ([]byte, error) {
	buf, err := n.tr.Recv(ctx, out)
	if err != nil {
		return nil, err
	}

	return n.readCS.Decrypt(out, nil, buf)
}

/*
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

	err := tr.Send(context.TODO(), msg)
	if err != nil {
		return nil, err
	}

	buf, err := tr.Recv(context.TODO(), make([]byte, 256))
	if err != nil {
		return nil, err
	}

	rcred, _, _, err := hs.ReadMessage(nil, buf)
	if err != nil {
		return nil, err
	}

	msg, csR, csW := hs.WriteMessage(nil, net.cred)

	err = tr.Send(context.TODO(), msg)
	if err != nil {
		return nil, err
	}

	rkey := hs.PeerStatic()

	if !net.validateCred(rkey, rcred) {
		return nil, ErrInvalidCred
	}

	sess := &noiseSession{mesh.Identity(rkey), tr, csR, csW}

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

	buf, err := tr.Recv(context.TODO(), make([]byte, 256))
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

	err = tr.Send(context.TODO(), msg)
	if err != nil {
		return nil, err
	}

	buf, err = tr.Recv(context.TODO(), buf)
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

	sess := &noiseSession{mesh.Identity(rkey), tr, csR, csW}

	p.AddNeighbor(sess.peerIdentity, sess)

	return sess, nil
}
*/

func (p *Peer) send(ctx context.Context, hdr *pb.Header) error {
	hdr.Sender = p.identityKey.Public

	data, err := hdr.Marshal()
	if err != nil {
		return err
	}

	return p.forward(ctx, hdr, data)
}

func (p *Peer) run() {
	p.lifetime, p.shutdown = context.WithCancel(context.Background())

	go p.handleOperations(p.lifetime)
}

func (p *Peer) Shutdown() {
	p.shutdown()
}

func (p *Peer) AttachPeer(id mesh.Identity, tr ByteTransport) {
	go p.Monitor(p.lifetime, id, tr)

	p.AddNeighbor(id, tr)
}
