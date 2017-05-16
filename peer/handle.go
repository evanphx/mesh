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

func (p *Peer) drive(ctx context.Context, neigh Identity, tr ByteTransport) error {
	msg := make([]byte, 1024)
	var err error

	for {
		msg, err = tr.Recv(ctx, msg)
		if err != nil {
			if err == ErrClosed {
				p.opChan <- neighborLeft{neigh}
			}

			return nil
		}

		err = p.handleMessage(ctx, msg)
		if err != nil {
			return err
		}
	}
}

func (p *Peer) Monitor(ctx context.Context, id Identity, tr ByteTransport) {
	err := p.drive(ctx, id, tr)
	if err != nil {
		log.Printf("Error monitoring transport: %s", err)
	}
}

func (p *Peer) handleMessage(ctx context.Context, buf []byte) error {
	var hdr Header

	err := hdr.Unmarshal(buf)
	if err != nil {
		return err
	}

	dest := Identity(hdr.Destination)

	if !dest.Equal(p.Identity()) {
		log.Debugf("%s forward to %s", p.Desc(), dest.Short())
		return p.forward(ctx, &hdr, buf)
	}

	select {
	case p.opChan <- inputOperation{&hdr}:
		// ok
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (p *Peer) forward(ctx context.Context, hdr *Header, buf []byte) error {
	var err error

	if buf == nil {
		buf, err = hdr.Marshal()
		if err != nil {
			return err
		}
	}

	dst := Identity(hdr.Destination)

	hop, err := p.router.Lookup(dst.String())
	if err != nil {
		return err
	}

	neigh, ok := p.neighbors[hop.Neighbor]
	if !ok {
		return errors.Wrapf(ErrUnroutable, "unknown neighbor: %s", hop.Neighbor)
	}

	return neigh.tr.Send(ctx, buf)
}
