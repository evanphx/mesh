package transport

import (
	"context"

	"github.com/evanphx/mesh"
	"github.com/flynn/noise"
)

type noiseSession struct {
	peerIdentity mesh.Identity

	tr Messenger

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
