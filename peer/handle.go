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

func (p *Peer) drive(ctx context.Context, tr ByteTransport) error {
	msg := make([]byte, 1024)
	var err error

	for {
		msg, err = tr.Recv(msg)
		if err != nil {
			return err
		}

		err = p.handleMessage(ctx, msg)
		if err != nil {
			return err
		}
	}
}

func (p *Peer) Monitor(ctx context.Context, tr ByteTransport) {
	err := p.drive(ctx, tr)
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
		return p.forward(&hdr, buf)
	}

	p.opChan <- inputOperation{&hdr}

	return nil

	/*

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
	*/
}

func (p *Peer) forward(hdr *Header, buf []byte) error {
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

	return neigh.tr.Send(buf)
}
