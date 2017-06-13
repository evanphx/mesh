package peer

import (
	"context"

	"github.com/evanphx/mesh/pb"
	"github.com/pkg/errors"
)

type Protocol interface {
	Handle(ctx context.Context, hdr *pb.Header) error
}

var ErrExistingProtocol = errors.New("existing protocol registered")

func (p *Peer) AddProtocol(num int32, proto Protocol) error {
	p.protoLock.Lock()
	defer p.protoLock.Unlock()

	if h, ok := p.protocols[num]; ok {
		return errors.Wrapf(ErrExistingProtocol, "handler: %T", h)
	}

	p.protocols[num] = proto
	return nil
}
