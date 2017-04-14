package peer

import (
	"context"

	"github.com/evanphx/mesh/log"
	"github.com/pkg/errors"
)

var (
	ErrShortRead  = errors.New("short read")
	ErrUnroutable = errors.New("unroutable")
)

func (p *Peer) handleMessage(ctx context.Context, buf []byte) error {
	var hdr Header

	err := hdr.Unmarshal(buf)
	if err != nil {
		return err
	}

	dest := Identity(hdr.Destination)

	if !dest.Equal(p.Identity()) {
		return p.forward(&hdr, buf)
	}

	log.Debugf("request: %s", hdr.Type)

	switch hdr.Type {
	case PIPE_OPEN:
		p.newPipeRequest(&hdr)
	case PIPE_OPENED:
		p.setPipeOpened(&hdr)
	case PIPE_DATA:
		p.newPipeData(&hdr)
	case PIPE_CLOSE:
		p.setPipeClosed(&hdr)
	case PIPE_UNKNOWN:
		p.setPipeUnknown(&hdr)
	}

	return nil
}

func (p *Peer) forward(hdr *Header, buf []byte) error {
	dst := Identity(hdr.Destination)

	hop, err := p.router.Lookup(dst.String())
	if err != nil {
		return err
	}

	neigh, ok := p.neighbors[hop.Neighbor]
	if !ok {
		return errors.Wrapf(ErrUnroutable, "unknown neighbor: %s", hop.Neighbor)
	}

	return neigh.tr.Send(buf)
}
