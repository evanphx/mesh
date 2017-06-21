package transport

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"sync"

	"github.com/evanphx/mesh/log"
)

type framer struct {
	r   *bufio.Reader
	w   *bufio.Writer
	sz1 []byte
	sz2 []byte

	rmu sync.Mutex
	wmu sync.Mutex
}

func NewFramer(bt io.ReadWriter) Messenger {
	return &framer{
		r: bufio.NewReader(bt),
		w: bufio.NewWriter(bt),
	}
}

const maxSzBuf = 12

func (f *framer) Send(ctx context.Context, msg []byte) error {
	f.wmu.Lock()
	defer f.wmu.Unlock()

	var ary [2]byte

	sz1 := ary[:]

	binary.BigEndian.PutUint16(sz1, uint16(len(msg)))

	n, err := f.w.Write(sz1)
	if err != nil {
		return err
	}

	if n != 2 {
		return io.ErrShortWrite
	}

	n, err = f.w.Write(msg)
	if err != nil {
		return err
	}

	if n != len(msg) {
		return io.ErrShortWrite
	}

	return f.w.Flush()
}

var cnt int

func (f *framer) Recv(ctx context.Context, buf []byte) ([]byte, error) {
	f.rmu.Lock()
	defer f.rmu.Unlock()

	var ary [2]byte

	sz2 := ary[:2]

	_, err := io.ReadFull(f.r, sz2)
	if err != nil {
		return nil, err
	}

	sz := binary.BigEndian.Uint16(sz2)

	if len(buf) < int(sz) {
		buf = make([]byte, sz)
	}

	data := buf[:sz]

	_, err = io.ReadFull(f.r, data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (f *framer) SendCRC(ctx context.Context, msg []byte) error {
	f.wmu.Lock()
	defer f.wmu.Unlock()

	if f.sz1 == nil {
		f.sz1 = make([]byte, 6)
	}

	binary.BigEndian.PutUint16(f.sz1, uint16(len(msg)))

	chk := crc32.ChecksumIEEE(msg)

	binary.BigEndian.PutUint32(f.sz1[2:], chk)

	n, err := f.w.Write(f.sz1)
	if err != nil {
		return err
	}

	if n != 6 {
		panic("nope")
	}

	n, err = f.w.Write(msg)
	if err != nil {
		return err
	}

	if n != len(msg) {
		panic("nope")
	}

	return f.w.Flush()
}

func (f *framer) RecvCRC(ctx context.Context, buf []byte) ([]byte, error) {
	f.rmu.Lock()
	defer f.rmu.Unlock()

	if f.sz2 == nil {
		f.sz2 = make([]byte, 6)
	}

	_, err := io.ReadFull(f.r, f.sz2)
	if err != nil {
		return nil, err
	}

	sz := binary.BigEndian.Uint16(f.sz2)

	chk := binary.BigEndian.Uint32(f.sz2[2:])

	if len(buf) < int(sz) {
		buf = make([]byte, sz)
	}

	_, err = io.ReadFull(f.r, buf[:sz])
	if err != nil {
		return nil, err
	}

	data := buf[:sz]

	exp := crc32.ChecksumIEEE(data)

	if chk != exp {
		return nil, fmt.Errorf("Invalid CRC: %d != %d", chk, exp)
	}

	log.Debugf("! CRC match: %x", exp)

	return data, nil
}
