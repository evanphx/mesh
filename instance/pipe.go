package instance

import (
	"context"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/grpc"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
	"github.com/evanphx/mesh/protocol/pipe"
)

func (i *Instance) ListenPipe(name string) (*pipe.ListenPipe, error) {
	return i.pipes.ListenPipe(name)
}

func (i *Instance) Listen(adver *pb.Advertisement) (*pipe.ListenPipe, error) {
	return i.pipes.Listen(adver)
}

func (i *Instance) ConnectPipe(ctx context.Context, dst mesh.Identity, name string) (*pipe.Pipe, error) {
	return i.pipes.ConnectPipe(ctx, dst, name)
}

func (i *Instance) Connect(ctx context.Context, sel *mesh.PipeSelector) (*pipe.Pipe, error) {
	return i.pipes.Connect(ctx, sel)
}

type RPCHandler interface {
	HandleTransport(ctx context.Context, tr grpc.Transport)
}

func (i *Instance) HandleRPC(ctx context.Context, adver *pb.Advertisement, h RPCHandler) error {
	lp, err := i.Listen(adver)
	if err != nil {
		return err
	}

	for {
		pipe, err := lp.Accept(ctx)
		if err != nil {
			return err
		}

		log.Debugf("%s accept rpc pipe", i.Peer.Desc())

		go h.HandleTransport(ctx, pipe)
	}

	return nil
}
