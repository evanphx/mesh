package transport

import (
	"context"
	"sync"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/crypto"
	"github.com/evanphx/mesh/log"
)

type noiseSession struct {
	mu sync.Mutex

	peerIdentity mesh.Identity

	tr Messenger

	readCS  crypto.CipherState
	writeCS crypto.CipherState
}

func (n *noiseSession) PeerIdentity() mesh.Identity {
	return n.peerIdentity
}

func (n *noiseSession) Encrypt(out, msg []byte) []byte {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.writeCS.Encrypt(out, nil, msg)
}

func (n *noiseSession) Decrypt(out, msg []byte) ([]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.readCS.Decrypt(out, nil, msg)
}

func (n *noiseSession) Send(ctx context.Context, msg []byte) error {
	return n.tr.Send(ctx, n.Encrypt(nil, msg))
}

func (n *noiseSession) Recv(ctx context.Context, out []byte) ([]byte, error) {
	buf, err := n.tr.Recv(ctx, out)
	if err != nil {
		return nil, err
	}

	msg, err := n.Decrypt(nil, buf)
	if err != nil {
		log.Printf("! decryption error: %s (%d)", err, len(msg))
		return nil, err
	}

	return msg, nil
}

func (n *noiseSession) Close(ctx context.Context) error {
	return n.tr.Close(ctx)
}
