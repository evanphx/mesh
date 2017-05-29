package util

import (
	"context"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/pb"
)

type HandleHeader interface {
	Handle(context.Context, *pb.Header) error
}

type SendDataer interface {
	SendData(context.Context, mesh.Identity, int32, interface{}) error
}

type SendAdapter struct {
	Sender  mesh.Identity
	Handler HandleHeader
}

func (s SendAdapter) SendData(ctx context.Context, dst mesh.Identity, proto int32, msg interface{}) error {
	var hdr pb.Header
	hdr.Sender = s.Sender
	hdr.Destination = dst
	hdr.Proto = proto

	data, err := msg.(mesh.Marshaler).Marshal()
	if err != nil {
		return err
	}

	hdr.Body = data

	return s.Handler.Handle(ctx, &hdr)
}
