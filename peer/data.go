package peer

import (
	"context"
	"fmt"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/pb"
)

type Marshaler interface {
	Marshal() ([]byte, error)
}

func (p *Peer) SendData(ctx context.Context, dst mesh.Identity, proto int32, body interface{}) error {
	var (
		data []byte
		err  error
	)

	switch st := body.(type) {
	case []byte:
		data = st
	case Marshaler:
		data, err = st.Marshal()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid type for body: %T", st)
	}

	var hdr pb.Header
	hdr.Destination = dst
	hdr.Proto = proto
	hdr.Body = data

	return p.send(ctx, &hdr)
}
