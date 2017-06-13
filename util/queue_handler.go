package util

import (
	"context"
	"sync"

	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
)

type QueueHandler struct {
	mu   sync.Mutex
	ctx  context.Context
	hdrs []*pb.Header

	PassX       int
	PassThrough bool
	Handler     HandleHeader
}

func (r *QueueHandler) SetPassthrough(val bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.PassThrough = val
}

func (r *QueueHandler) SetPassX(val int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.PassX = val
}

func (r *QueueHandler) Handle(ctx context.Context, hdr *pb.Header) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.PassThrough {
		return r.Handler.Handle(ctx, hdr)
	}

	if r.PassX > 0 {
		r.PassX--
		return r.Handler.Handle(ctx, hdr)
	}

	log.Debugf("queued hdr for delivery: %p", hdr)

	r.ctx = ctx
	r.hdrs = append(r.hdrs, hdr)
	return nil
}

func (r *QueueHandler) Swap(i, j int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.hdrs[i], r.hdrs[j] = r.hdrs[j], r.hdrs[i]
}

func (r *QueueHandler) Deliver(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Debugf("delivering %d hdrs in as stored", len(r.hdrs))

	for i, hdr := range r.hdrs {

		if n > 0 && n == i {
			r.hdrs = r.hdrs[i:]
			return
		}

		log.Debugf("delivering %p", hdr)
		r.Handler.Handle(r.ctx, hdr)
	}

	r.hdrs = nil
	return
}

func (r *QueueHandler) Size() int {
	return len(r.hdrs)
}
