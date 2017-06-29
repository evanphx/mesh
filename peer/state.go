package peer

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
	"github.com/evanphx/mesh/router"
)

type inputOperation struct {
	hdr *pb.Header
}

func (p inputOperation) OpType() string {
	return "input"
}

type routeUpdate struct {
	update *pb.RouteUpdate
	prop   bool
}

func (r routeUpdate) OpType() string {
	return "route-update"
}

type routeRetrieve struct {
	req    *pb.RouteRequest
	update chan *pb.RouteUpdate
}

func (r routeRetrieve) OpType() string {
	return "route-retrieve"
}

type neighborAdd struct {
	id mesh.Identity
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
	peer mesh.Identity
	err  error
}

func (n rpcError) OpType() string {
	return "rpc-error"
}

type syncAdsOp struct {
	update *pb.AdvertisementUpdate
	resp   chan *pb.AdvertisementChanges
	from   mesh.Identity
}

func (o syncAdsOp) OpType() string {
	return "sync-ads"
}

type introduceAdver struct {
	adver *pb.Advertisement
}

func (o introduceAdver) OpType() string {
	return "introduce-adver"
}

type inputAdvers struct {
	advers []*pb.Advertisement
}

func (o inputAdvers) OpType() string {
	return "input-advers"
}

type removeAdver struct {
	adver *pb.Advertisement
}

func (o removeAdver) OpType() string {
	return "remove-adver"
}

type neighborLeft struct {
	neigh mesh.Identity
}

func (o neighborLeft) OpType() string {
	return "neighbor-left"
}

type printStatus struct{}

func (p printStatus) OpType() string {
	return "print-status"
}

func (p *Peer) PrintStatus() {
	p.opChan <- printStatus{}
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
			p.pruneStaleAdvers()
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

		p.protoLock.RLock()

		ph, ok := p.protocols[hdr.Proto]

		p.protoLock.RUnlock()

		if !ok {
			log.Debugf("Unknown protocol requested: %d", hdr.Proto)
			// TODO send back some kind of "huh?"
			return
		}

		err := ph.Handle(ctx, hdr)
		if err != nil {
			log.Printf("Error in protocol handler %T: %s", ph, err)
			// TODO send back an error message?
		}

	case printStatus:
		fmt.Printf("==== Status of %s", p.Identity())

		p.neighLock.Lock()

		if len(p.neighbors) == 0 {
			fmt.Printf("No neighbors")
		} else {
			for _, neigh := range p.neighbors {
				fmt.Printf("Neighbor: %s => %T\n", neigh.Id.Short(), neigh.tr)
			}
		}

		p.neighLock.Unlock()

		p.adverLock.Lock()

		fmt.Printf("Advertisements:\n")
		for _, adver := range p.advers {
			fmt.Printf("ID: %s\n", adver.adver.Id)
			fmt.Printf("  Pipe: %s\n", adver.adver.Pipe)
			fmt.Printf("  Owner: %s\n", adver.adver.Owner.Short())
			fmt.Printf("  Tags: %#v\n", adver.adver.Tags)
			fmt.Printf("  TTL: %d (%s, %s)\n", adver.adver.TimeToLive, adver.expiresAt.Sub(time.Now()), adver.expiresAt)
		}

		p.adverLock.Unlock()

	case neighborAdd:
		p.neighLock.Lock()

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

		var req pb.RouteUpdate

		req.Neighbor = p.Identity()
		req.Routes = append(req.Routes, &pb.Route{
			Destination: op.id,
			Weight:      1,
		})

		for _, neigh := range p.neighbors {
			if neigh.Id.Equal(op.id) {
				continue
			}

			go p.sendNewRoute(p.lifetime, neigh.Id, &req)
		}

		p.neighLock.Unlock()

	case neighborLeft:
		p.neighLock.Lock()

		delete(p.neighbors, op.neigh.String())

		p.router.PruneByHop(op.neigh.String())
		log.Debugf("pruning neighbor advers: %s", op.neigh)
		p.pruneNeighAdvers(op.neigh)

		log.Debugf("broadcasting neighbor route lost")

		var req pb.RouteUpdate

		req.Neighbor = p.Identity()
		req.Routes = append(req.Routes, &pb.Route{
			Destination: op.neigh,
			Prune:       true,
		})

		for _, neigh := range p.neighbors {
			go p.sendNewRoute(p.lifetime, neigh.Id, &req)
		}

		p.neighLock.Unlock()
	case routeUpdate:
		for _, route := range op.update.Routes {
			// Skip routes advertise for us, we know where we are
			if route.Destination.Equal(p.Identity()) {
				continue
			}

			if route.Prune {
				log.Debugf("%s prune that %s has %s", p.Desc(),
					op.update.Neighbor.Short(),
					route.Destination.Short(),
				)

				p.router.Update(router.Update{
					Neighbor:    op.update.Neighbor.String(),
					Destination: route.Destination.String(),
					Prune:       true,
				})

				_, err := p.router.Lookup(route.Destination.String())
				if err != nil {
					for _, proto := range p.protocols {
						proto.Unroutable(route.Destination)
					}
				}
			} else {
				log.Debugf("%s ROUTE update from %s: %s => %d",
					p.Desc(),
					op.update.Neighbor.Short(),
					route.Destination.Short(),
					int(route.Weight)+1,
				)

				p.router.Update(router.Update{
					Neighbor:    op.update.Neighbor.String(),
					Destination: route.Destination.String(),
					Weight:      int(route.Weight) + 1,
				})
			}
		}

		if op.prop {
			from := op.update.Neighbor

			op.update.Neighbor = p.Identity()

			p.neighLock.Lock()

			for _, neigh := range p.neighbors {
				if neigh.Id.Equal(from) {
					continue
				}

				go p.sendNewRoute(p.lifetime, neigh.Id, op.update)
			}

			p.neighLock.Unlock()
		}
	case routeRetrieve:
		var update pb.RouteUpdate

		update.Neighbor = p.Identity()

		for _, hop := range p.router.RoutesSince(op.req.Since) {
			log.Debugf("%s ROUTE advertise: %s => %d",
				p.Desc(), mesh.ToIdentity(hop.Destination).Short(),
				int32(hop.Weight),
			)

			update.Routes = append(update.Routes, &pb.Route{
				Destination: mesh.ToIdentity(hop.Destination),
				Weight:      int32(hop.Weight),
			})
		}

		op.update <- &update
	case introduceAdver:
		update := &pb.AdvertisementUpdate{}
		update.Origin = p.Identity()
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
		update := &pb.AdvertisementUpdate{}
		update.Origin = p.Identity()
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
			op.resp <- &pb.AdvertisementChanges{changes}
		}

		// Flood to neighbors
		p.floodUpdate(op.update, op.from)

	case inputAdvers:
		p.adverLock.Lock()

		for _, ad := range op.advers {
			p.advers[ad.Id] = &localAdver{
				expiresAt: ad.ExpiresAt(),
				adver:     ad,
			}
		}

		log.Debugf("inputAdvers: %d new ads", len(op.advers))

		p.adverLock.Unlock()
	case rpcError:
		log.Debugf("%s rpc error detected on %s: %s: %s", p.Desc(), op.peer.Short(), op.loc, op.err)
	default:
		log.Debugf("Unknown operation: %T", val)
	}
}
