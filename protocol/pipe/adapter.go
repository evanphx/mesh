package pipe

import (
	"context"
	"io"
)

type ReaderAdapter struct {
	ctx  context.Context
	pipe *Pipe
	rest []byte
}

func (r *ReaderAdapter) Read(b []byte) (int, error) {
	if len(r.rest) > 0 {
		if len(b) >= len(r.rest) {
			copy(b, r.rest)
			r.rest = nil
			return len(r.rest), nil
		}

		copy(b, r.rest[:len(b)])
		r.rest = r.rest[len(b):]
		return len(b), nil
	}

	data, err := r.pipe.Recv(r.ctx)
	if err != nil {
		return 0, err
	}

	if len(b) >= len(data) {
		copy(b, data)
		return len(data), nil
	}

	copy(b, data[:len(b)])
	r.rest = data[len(b):]

	return len(b), nil
}

func Reader(ctx context.Context, p *Pipe) io.Reader {
	return &ReaderAdapter{ctx: ctx, pipe: p}
}

type WriterAdapter struct {
	ctx  context.Context
	pipe *Pipe
}

func (w *WriterAdapter) Write(b []byte) (int, error) {
	out := make([]byte, len(b))
	copy(out, b)

	err := w.pipe.Send(w.ctx, out)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

func Writer(ctx context.Context, p *Pipe) io.Writer {
	return &WriterAdapter{ctx: ctx, pipe: p}
}
