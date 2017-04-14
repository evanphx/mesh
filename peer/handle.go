package peer

import (
	"context"
	"errors"

	"github.com/evanphx/mesh/log"
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
		log.Printf("Only handling local messages atm. %s != %s", dest, p.Identity())
		return ErrUnroutable
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
