package peer

import (
	"context"

	"github.com/evanphx/mesh/grpc"
	"github.com/evanphx/mesh/log"
)

func (p *Peer) reachable(id Identity) bool {
	_, err := p.router.Lookup(id.String())
	if err != nil {
		return false
	}

	return true
}

func (p *Peer) UpdateRoutes(ctx context.Context, neigh Identity) error {
	var req RouteRequest

	req.Since = 1

	pipe, err := p.ConnectPipe(ctx, neigh, ":rpc")
	if err != nil {
		return err
	}

	defer pipe.Close()

	client := NewRouterClient(grpc.NewClientConn(pipe))

	updates, err := client.RoutesSince(ctx, &req)
	if err != nil {
		return err
	}

	for _, req := range updates.Routes {
		p.AddRoute(neigh, Identity(req.Destination))
	}

	return nil
}

func (p *Peer) UpdateNeighbors(ctx context.Context, req *RouteUpdate, skip Identity) error {
	p.neighLock.Lock()
	defer p.neighLock.Unlock()

	req.Neighbor = p.Identity()

	for _, neigh := range p.neighbors {
		if neigh.Id.Equal(skip) {
			continue
		}

		pipe, err := p.ConnectPipe(ctx, neigh.Id, ":rpc")
		if err != nil {
			return err
		}

		client := NewRouterClient(grpc.NewClientConn(pipe))

		_, err = client.NewRoute(ctx, req)

		pipe.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Peer) ListenForRPC(ctx context.Context) error {
	log.Debugf("%s listen for rpc", p.Desc())
	RegisterRouterServer(p.rpcServer, p)
	RegisterServicesServer(p.rpcServer, p)

	lp, err := p.ListenPipe(":rpc")
	if err != nil {
		log.Printf("%s error creating rpc listen pipe: %s", p.Desc(), err)
		return err
	}

	for {
		pipe, err := lp.Accept(ctx)
		if err != nil {
			return err
		}

		log.Debugf("%s accept rpc pipe", p.Desc())

		go p.rpcServer.HandleTransport(context.Background(), pipe)
	}
}

var _ RouterServer = (*Peer)(nil)

func (p *Peer) RoutesSince(ctx context.Context, req *RouteRequest) (*RouteUpdate, error) {
	resp := make(chan *RouteUpdate)

	p.opChan <- routeRetrieve{req: req, update: resp}

	return <-resp, nil
}

func (p *Peer) NewRoute(ctx context.Context, req *RouteUpdate) (*NoResponse, error) {
	log.Debugf("%s new route received", p.Desc())
	p.opChan <- routeUpdate{update: req, prop: true}
	return &NoResponse{}, nil
}
