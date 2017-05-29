package util

import (
	"context"

	"github.com/evanphx/mesh"
)

type ByteTrader struct {
	r chan []byte
	w chan []byte
}

func PairByteTraders() (*ByteTrader, *ByteTrader) {
	i := make(chan []byte, 5)
	r := make(chan []byte, 5)

	return &ByteTrader{i, r}, &ByteTrader{r, i}
}

func (b *ByteTrader) Send(_ context.Context, msg []byte) error {
	b.w <- msg
	return nil
}

func (b *ByteTrader) Recv(_ context.Context, msg []byte) ([]byte, error) {
	m, ok := <-b.r
	if !ok {
		return nil, mesh.ErrClosed
	}

	return m, nil
}

func (b *ByteTrader) Close(_ context.Context) error {
	if b.w != nil {
		close(b.w)
		b.w = nil
	}

	return nil
}
