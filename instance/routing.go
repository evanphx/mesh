package instance

import (
	"context"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/grpc"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
)

func (i *Instance) FetchRoutes(ctx context.Context, neigh mesh.Identity, epoch int64) (*pb.RouteUpdate, error) {
	var req pb.RouteRequest

	req.Since = epoch

	pipe, err := i.pipes.LazyConnectPipe(ctx, neigh, ":rpc")
	if err != nil {
		return nil, err
	}

	defer pipe.Close(ctx)

	client := pb.NewRouterClient(grpc.NewClientConn(pipe))

	return client.RoutesSince(ctx, &req)
}

func (i *Instance) SendRoute(ctx context.Context, neigh mesh.Identity, req *pb.RouteUpdate) error {
	pipe, err := i.pipes.ConnectPipe(ctx, neigh, ":rpc")
	if err != nil {
		return err
	}

	defer pipe.Close(ctx)

	client := pb.NewRouterClient(grpc.NewClientConn(pipe))

	_, err = client.NewRoute(ctx, req)

	return err
}

func (i *Instance) listenForRPC(ctx context.Context) error {
	log.Debugf("%s listen for rpc", i.Peer.Desc())
	pb.RegisterRouterServer(i.rpcServer, i.Peer)
	pb.RegisterServicesServer(i.rpcServer, i.Peer)

	lp, err := i.ListenPipe(":rpc")
	if err != nil {
		log.Printf("%s error creating rpc listen pipe: %s", i.Peer.Desc(), err)
		return err
	}

	for {
		pipe, err := lp.Accept(ctx)
		if err != nil {
			return err
		}

		log.Debugf("%s accept rpc pipe", i.Peer.Desc())

		go i.rpcServer.HandleTransport(context.Background(), pipe)
	}
}
