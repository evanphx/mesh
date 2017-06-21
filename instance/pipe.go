package instance

import (
	"context"

	"github.com/evanphx/mesh"
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
