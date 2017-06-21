package instance

import (
	"context"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/grpc"
	"github.com/evanphx/mesh/pb"
)

func (i *Instance) GetAllAdvertisements(ctx context.Context, neigh mesh.Identity) (*pb.AdvertisementSet, error) {
	pipe, err := i.ConnectPipe(ctx, neigh, ":rpc")
	if err != nil {
		return nil, err
	}

	defer pipe.Close(ctx)

	client := pb.NewServicesClient(grpc.NewClientConn(pipe))

	return client.RetrieveAdvertisements(ctx, &pb.RetrieveAdverRequest{})
}

func (i *Instance) SyncAdvertisements(ctx context.Context, neigh mesh.Identity, update *pb.AdvertisementUpdate) error {
	pipe, err := i.ConnectPipe(ctx, neigh, ":rpc")
	if err != nil {
		return err
	}

	defer pipe.Close(ctx)

	client := pb.NewServicesClient(grpc.NewClientConn(pipe))

	_, err = client.SyncAdvertisements(ctx, update)
	return err
}
