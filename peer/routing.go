package peer

import (
	"context"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
	"github.com/evanphx/mesh/router"
)

func (p *Peer) reachable(id mesh.Identity) bool {
	_, err := p.router.Lookup(id.String())
	if err != nil {
		return false
	}

	return true
}

func (p *Peer) LookupRoute(dest mesh.Identity) (router.Hop, error) {
	return p.router.Lookup(dest.String())
}

type RouteOps interface {
	FetchRoutes(ctx context.Context, neigh mesh.Identity, epoch int64) (*pb.RouteUpdate, error)
	SendRoute(ctx context.Context, neigh mesh.Identity, req *pb.RouteUpdate) error
}

func (p *Peer) getRoutes(ctx context.Context, id mesh.Identity) {
	update, err := p.routeOps.FetchRoutes(ctx, id, 1)
	if err != nil {
		p.opChan <- rpcError{"FetchRoutes", id, err}
		return
	}

	p.opChan <- routeUpdate{update: update, prop: false}
}

func (p *Peer) sendNewRoute(ctx context.Context, id mesh.Identity, req *pb.RouteUpdate) {
	for _, route := range req.Routes {
		log.Debugf("%s sending route to %s: %s", p.Desc(), id.Short(), route.Destination.Short())
	}

	err := p.routeOps.SendRoute(ctx, id, req)
	if err != nil {
		p.opChan <- rpcError{"SendRoute", id, err}
		return
	}
}

var _ pb.RouterServer = (*Peer)(nil)

func (p *Peer) RoutesSince(ctx context.Context, req *pb.RouteRequest) (*pb.RouteUpdate, error) {
	resp := make(chan *pb.RouteUpdate)

	p.opChan <- routeRetrieve{req: req, update: resp}

	return <-resp, nil
}

func (p *Peer) NewRoute(ctx context.Context, req *pb.RouteUpdate) (*pb.NoResponse, error) {
	log.Debugf("%s new route received", p.Desc())
	p.opChan <- routeUpdate{update: req, prop: true}
	return &pb.NoResponse{}, nil
}
