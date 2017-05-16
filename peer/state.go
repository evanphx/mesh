package peer

import (
	"context"
	"runtime"
	"time"

	"github.com/evanphx/mesh/grpc"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/router"
)

type inputOperation struct {
	hdr *Header
}

func (p inputOperation) OpType() string {
	return "input"
}

type routeUpdate struct {
	update *RouteUpdate
	prop   bool
}

func (r routeUpdate) OpType() string {
	return "route-update"
}

type routeRetrieve struct {
	req    *RouteRequest
	update chan *RouteUpdate
}

func (r routeRetrieve) OpType() string {
	return "route-retrieve"
}

type neighborAdd struct {
	id Identity
	tr ByteTransport
}

func (n neighborAdd) OpType() string {
	return "neighbor-add"
}

type operation interface {
	OpType() string
}

type rpcError struct {
	loc  string
	peer Identity
	err  error
}

func (n rpcError) OpType() string {
	return "rpc-error"
}

type syncAdsOp struct {
	update *AdvertisementUpdate
	resp   chan *AdvertisementChanges
}

func (o syncAdsOp) OpType() string {
	return "sync-ads"
}

type introduceAdver struct {
	adver *Advertisement
}

func (o introduceAdver) OpType() string {
	return "introduce-adver"
}

type inputAdvers struct {
	advers []*Advertisement
}

func (o inputAdvers) OpType() string {
	return "input-advers"
}

type removeAdver struct {
	adver *Advertisement
}

func (o removeAdver) OpType() string {
	return "remove-adver"
}

type neighborLeft struct {
	neigh Identity
}

func (o neighborLeft) OpType() string {
	return "neighbor-left"
}

func (p *Peer) handleOperations(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case op := <-p.opChan:
			p.processOperation(ctx, op)
		case <-ticker.C:
			p.rebroadcastAdvers()
		case <-ctx.Done():
			return
		}
	}
}

func (p *Peer) WaitTilIdle() {
	for len(p.opChan) > 0 {
		runtime.Gosched()
	}
}

func (p *Peer) processOperation(ctx context.Context, val operation) {
	log.Debugf("%s op %T", p.Desc(), val)

	switch op := val.(type) {
	case inputOperation:
		hdr := op.hdr

		switch hdr.Type {
		case PIPE_OPEN:
			p.newPipeRequest(ctx, hdr)
		case PIPE_OPENED:
			p.setPipeOpened(ctx, hdr)
		case PIPE_DATA:
			p.newPipeData(ctx, hdr)
		case PIPE_CLOSE:
			p.setPipeClosed(ctx, hdr)
		case PIPE_UNKNOWN:
			p.setPipeUnknown(ctx, hdr)
		case PING:
			var out Header
			out.Sender = p.Identity()
			out.Destination = hdr.Sender
			out.Type = PONG
			out.Session = hdr.Session

			go p.send(ctx, &out)
		case PONG:
			p.processPong(hdr)
		default:
			log.Debugf("Unknown pipe operation: %s", hdr.Type)
		}
	case neighborAdd:
		p.neighbors[op.id.String()] = &Neighbor{
			Id: op.id,
			tr: op.tr,
		}

		// Add a direct routing entry for the neighbor
		p.router.Update(router.Update{
			Neighbor:    op.id.String(),
			Destination: op.id.String(),
			Weight:      1,
		})

		log.Debugf("%s: route add %s => %d", p.Desc(), op.id.Short(), 1)

		// Get the neighbors routes
		go p.getRoutes(p.lifetime, op.id)
		go p.getAllAds(p.lifetime, op.id)

		var req RouteUpdate

		req.Neighbor = p.Identity()
		req.Routes = append(req.Routes, &Route{
			Destination: op.id,
			Weight:      1,
		})

		for _, neigh := range p.neighbors {
			if neigh.Id.Equal(op.id) {
				continue
			}

			go p.sendNewRoute(p.lifetime, neigh.Id, &req)
		}

	case neighborLeft:
		delete(p.neighbors, op.neigh.String())

		p.router.PruneByHop(op.neigh.String())
		// p.pruneNeighAdvers(op.neigh)
	case routeUpdate:
		for _, route := range op.update.Routes {
			// Skip routes advertise for us, we know where we are
			if Identity(route.Destination).Equal(p.Identity()) {
				continue
			}

			log.Debugf("%s ROUTE update from %s: %s => %d",
				p.Desc(),
				Identity(op.update.Neighbor).Short(),
				Identity(route.Destination).Short(),
				int(route.Weight)+1,
			)

			p.router.Update(router.Update{
				Neighbor:    Identity(op.update.Neighbor).String(),
				Destination: Identity(route.Destination).String(),
				Weight:      int(route.Weight) + 1,
			})
		}

		if op.prop {
			from := op.update.Neighbor

			op.update.Neighbor = p.Identity()

			for _, neigh := range p.neighbors {
				if neigh.Id.Equal(from) {
					continue
				}

				go p.sendNewRoute(p.lifetime, neigh.Id, op.update)
			}
		}
	case routeRetrieve:
		var update RouteUpdate

		update.Neighbor = p.Identity()

		for _, hop := range p.router.RoutesSince(op.req.Since) {
			log.Debugf("%s ROUTE advertise: %s => %d",
				p.Desc(), ToIdentity(hop.Destination).Short(),
				int32(hop.Weight),
			)

			update.Routes = append(update.Routes, &Route{
				Destination: ToIdentity(hop.Destination),
				Weight:      int32(hop.Weight),
			})
		}

		op.update <- &update
	case introduceAdver:
		update := &AdvertisementUpdate{}
		update.NewAdvers = append(update.NewAdvers, op.adver)

		p.adverLock.Lock()
		p.selfAdvers[op.adver.Id] = op.adver

		p.advers[op.adver.Id] = &localAdver{
			adver: op.adver,
		}
		p.adverLock.Unlock()

		p.neighLock.Lock()

		for _, neigh := range p.neighbors {
			go p.syncAds(p.lifetime, neigh.Id, update)
		}

		p.neighLock.Unlock()
	case removeAdver:
		update := &AdvertisementUpdate{}
		update.RemoveAdvers = append(update.RemoveAdvers, op.adver.Id)

		p.adverLock.Lock()
		delete(p.selfAdvers, op.adver.Id)
		delete(p.advers, op.adver.Id)
		p.adverLock.Unlock()

		p.neighLock.Lock()
		for _, neigh := range p.neighbors {
			go p.syncAds(p.lifetime, neigh.Id, update)
		}
		p.neighLock.Unlock()
	case syncAdsOp:
		p.adverLock.Lock()

		log.Debugf("%s adding %d new advers", p.Desc(), len(op.update.NewAdvers))

		var changes int32

		for _, ad := range op.update.NewAdvers {
			changes++

			p.advers[ad.Id] = &localAdver{
				expiresAt: ad.ExpiresAt(),
				adver:     ad,
			}
		}

		for _, id := range op.update.RemoveAdvers {
			changes++

			delete(p.advers, id)
		}

		p.adverLock.Unlock()

		if op.resp != nil {
			op.resp <- &AdvertisementChanges{changes}
		}

	case inputAdvers:
		p.adverLock.Lock()

		for _, ad := range op.advers {
			p.advers[ad.Id] = &localAdver{
				expiresAt: ad.ExpiresAt(),
				adver:     ad,
			}
		}

		p.adverLock.Unlock()
	case rpcError:
		log.Debugf("%s rpc error detected on %s: %s: %s", p.Desc(), Identity(op.peer).Short(), op.loc, op.err)
	default:
		log.Debugf("Unknown operation: %T", val)
	}
}

func (p *Peer) getRoutes(ctx context.Context, id Identity) {
	var req RouteRequest

	req.Since = 1

	pipe, err := p.LazyConnectPipe(ctx, id, ":rpc")
	if err != nil {
		p.opChan <- rpcError{"LazyConnectPipe", id, err}
	}

	defer pipe.Close(ctx)

	client := NewRouterClient(grpc.NewClientConn(pipe))

	update, err := client.RoutesSince(ctx, &req)
	if err != nil {
		p.opChan <- rpcError{"client.RoutesSince", id, err}
		return
	}

	p.opChan <- routeUpdate{update: update, prop: false}
}

func (p *Peer) sendNewRoute(ctx context.Context, id Identity, req *RouteUpdate) {
	for _, route := range req.Routes {
		log.Debugf("%s sending route to %s: %s", p.Desc(), id.Short(), Identity(route.Destination).Short())
	}

	pipe, err := p.LazyConnectPipe(ctx, id, ":rpc")
	if err != nil {
		p.opChan <- rpcError{"LazyConnectPipe", id, err}
		return
	}

	log.Debugf("%s rpc pipe opened to %s", p.Desc(), id.Short())

	defer pipe.Close(ctx)

	client := NewRouterClient(grpc.NewClientConn(pipe))

	_, err = client.NewRoute(ctx, req)
	if err != nil {
		p.opChan <- rpcError{"client.NewRoute", id, err}
		return
	}
}
