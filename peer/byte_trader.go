package peer

import "context"

type byteTrader struct {
	r chan []byte
	w chan []byte
}

func pairByteTraders() (*byteTrader, *byteTrader) {
	i := make(chan []byte, 5)
	r := make(chan []byte, 5)

	return &byteTrader{i, r}, &byteTrader{r, i}
}

func (b *byteTrader) Send(_ context.Context, msg []byte) error {
	b.w <- msg
	return nil
}

func (b *byteTrader) Recv(_ context.Context, msg []byte) ([]byte, error) {
	m, ok := <-b.r
	if !ok {
		return nil, ErrClosed
	}

	return m, nil
}

func (b *byteTrader) Close(_ context.Context) error {
	if b.w != nil {
		close(b.w)
		b.w = nil
	}

	return nil
}
