package peer

type byteTrader struct {
	r chan []byte
	w chan []byte
}

func pairByteTraders() (*byteTrader, *byteTrader) {
	i := make(chan []byte, 5)
	r := make(chan []byte, 5)

	return &byteTrader{i, r}, &byteTrader{r, i}
}

func (b *byteTrader) Send(msg []byte) error {
	b.w <- msg
	return nil
}

func (b *byteTrader) Recv(msg []byte) ([]byte, error) {
	m, ok := <-b.r
	if !ok {
		return nil, ErrClosed
	}

	return m, nil
}

func (b *byteTrader) Close() error {
	if b.w != nil {
		close(b.w)
		b.w = nil
	}

	return nil
}
