package peer

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ed25519"

	"github.com/evanphx/mesh/auth"
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

type Peer struct {
	PipeBacklog int

	identityKey noise.DHKey

	networks map[string]*Network

	lock      sync.Mutex
	router    *router.Router
	neighbors map[string]*Neighbor
	listening map[string]*ListenPipe
	pipes     map[int64]*Pipe

	nextSession int64
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
		pipes:       make(map[int64]*Pipe),
	}

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
		pipes:       make(map[int64]*Pipe),
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
	p.lock.Lock()
	defer p.lock.Unlock()

	// Add a direct routing entry for the neighbor
	p.router.Update(router.Update{
		Neighbor:    id.String(),
		Destination: id.String(),
	})

	p.neighbors[id.String()] = &Neighbor{
		Id: id,
		tr: tr,
	}
}

func (p *Peer) Identity() Identity {
	return Identity(p.identityKey.Public)
}

func (p *Peer) Authorize(a *auth.Authorizer) error {
	p.networks[a.Name()] = &Network{
		authPub: a.PublicKey(),
		cred:    a.Sign(p.identityKey.Public),
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

func (p *Peer) sendMessage(dst Identity, t Header_Type, s int64, body []byte) error {
	hop, err := p.router.Lookup(dst.String())
	if err != nil {
		return err
	}

	neigh, ok := p.neighbors[hop.Neighbor]
	if !ok {
		return errors.Wrapf(ErrUnroutable, "unknown neighbor: %s", hop.Neighbor)
	}

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

	return neigh.tr.Send(data)
}
