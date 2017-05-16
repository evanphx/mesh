package transport

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
)

type framer struct {
	r   *bufio.Reader
	w   *bufio.Writer
	sz1 []byte
	sz2 []byte
}

func NewFramer(bt io.ReadWriter) Messenger {
	return &framer{
		r: bufio.NewReader(bt),
		w: bufio.NewWriter(bt),
	}
}

const maxSzBuf = 12

func (f *framer) Send(ctx context.Context, msg []byte) error {
	if f.sz1 == nil {
		f.sz1 = make([]byte, 2)
	}

	binary.BigEndian.PutUint16(f.sz1, uint16(len(msg)))

	_, err := f.w.Write(f.sz1)
	if err != nil {
		return err
	}

	_, err = f.w.Write(msg)
	if err != nil {
		return err
	}

	return f.w.Flush()
}

func (f *framer) Recv(ctx context.Context, buf []byte) ([]byte, error) {
	if f.sz2 == nil {
		f.sz2 = make([]byte, 2)
	}

	_, err := io.ReadFull(f.r, f.sz2)
	if err != nil {
		return nil, err
	}

	sz := binary.BigEndian.Uint16(f.sz2)

	if len(buf) < int(sz) {
		buf = make([]byte, sz)
	}

	_, err = io.ReadFull(f.r, buf[:sz])
	if err != nil {
		return nil, err
	}

	return buf[:sz], nil
}
