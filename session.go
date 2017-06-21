package mesh

import "context"

type Session interface {
	PeerIdentity() Identity

	Send(ctx context.Context, msg []byte) error
	Recv(ctx context.Context, out []byte) ([]byte, error)

	Encrypt(msg, out []byte) []byte
	Decrypt(msg, out []byte) ([]byte, error)

	Close(ctx context.Context) error
}
