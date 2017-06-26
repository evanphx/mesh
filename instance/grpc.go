package instance

import (
	"context"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/grpc"
	"github.com/evanphx/mesh/log"
)

func (i *Instance) RPCConnect(sel *mesh.PipeSelector) *grpc.ClientConn {
	return grpc.NewOnDemandClientConn(func(ctx context.Context) (grpc.Transport, error) {
		log.Debugf("Connecting on demand to %+v", sel)
		return i.Connect(ctx, sel)
	})
}
