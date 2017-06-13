package util

import (
	"context"
	"sync"

	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
)

type ReverseHandler struct {
	mu   sync.Mutex
	ctx  context.Context
	hdrs []*pb.Header

	Handler     HandleHeader
	PassThrough bool
}

func (r *ReverseHandler) Handle(ctx context.Context, hdr *pb.Header) error {
	if r.PassThrough {
		return r.Handler.Handle(ctx, hdr)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	log.Debugf("queued hdr for reverse delivery: %p", hdr)

	r.ctx = ctx
	r.hdrs = append(r.hdrs, hdr)
	return nil
}

func (r *ReverseHandler) Deliver() {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Debugf("delivering %d hdrs in reverse order", len(r.hdrs))

	for i := len(r.hdrs) - 1; i >= 0; i-- {
		log.Debugf("reverse handler: delivering %p", r.hdrs[i])
		r.Handler.Handle(r.ctx, r.hdrs[i])
	}

	r.hdrs = nil
	return
}

func (r *ReverseHandler) Swap(i, j int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.hdrs[i], r.hdrs[j] = r.hdrs[j], r.hdrs[i]
}

func (r *ReverseHandler) DeliverAsStored() {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Debugf("delivering %d hdrs in as stored", len(r.hdrs))

	for _, hdr := range r.hdrs {
		log.Debugf("delivering %p", hdr)
		r.Handler.Handle(r.ctx, hdr)
	}

	r.hdrs = nil
	return
}
