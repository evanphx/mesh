package transport

import (
	"context"
	"sync"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/crypto"
)

type noiseSession struct {
	rmu sync.Mutex
	wmu sync.Mutex

	peerIdentity mesh.Identity

	tr       Messenger
	remoteId string

	readCS  crypto.CipherState
	writeCS crypto.CipherState
}

func (n *noiseSession) RemoteID() string {
	return n.remoteId
}

func (n *noiseSession) PeerIdentity() mesh.Identity {
	return n.peerIdentity
}

func (n *noiseSession) Encrypt(out, msg []byte) []byte {
	n.wmu.Lock()
	defer n.wmu.Unlock()

	return n.writeCS.Encrypt(out, nil, msg)
}

func (n *noiseSession) Decrypt(out, msg []byte) ([]byte, error) {
	n.rmu.Lock()
	defer n.rmu.Unlock()

	return n.readCS.Decrypt(out, nil, msg)
}

func (n *noiseSession) Send(ctx context.Context, msg []byte) error {
	n.wmu.Lock()
	defer n.wmu.Unlock()

	return n.tr.Send(ctx, n.writeCS.Encrypt(nil, nil, msg))
}

func (n *noiseSession) Recv(ctx context.Context, out []byte) ([]byte, error) {
	n.rmu.Lock()
	defer n.rmu.Unlock()

	buf, err := n.tr.Recv(ctx, out)
	if err != nil {
		return nil, err
	}

	msg, err := n.readCS.Decrypt(nil, nil, buf)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (n *noiseSession) Close(ctx context.Context) error {
	return n.tr.Close(ctx)
}
