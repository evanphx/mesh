package peer

import (
	"context"
	"time"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
	"github.com/satori/go.uuid"
)

const DefaultTTL = 300

type localAdver struct {
	expiresAt time.Time
	adver     *pb.Advertisement
}

func (l *localAdver) valid(t time.Time) bool {
	if l.expiresAt.IsZero() {
		return true
	}

	return t.Before(l.expiresAt)
}

func (p *Peer) Advertise(ad *pb.Advertisement) error {
	// Don't advertise internal pipes
	if ad.Pipe != "" && ad.Pipe[0] == ':' {
		return nil
	}

	if ad.Owner == nil {
		ad.Owner = p.Identity()
	}

	if ad.Id == "" {
		ad.Id = uuid.NewV4().String()
	}

	if ad.TimeToLive == 0 {
		ad.TimeToLive = DefaultTTL
	}

	p.opChan <- introduceAdver{ad}
	return nil
}

func (p *Peer) RemoveAdvertisement(ad *pb.Advertisement) error {
	if ad.Owner == nil {
		ad.Owner = p.Identity()
	}

	if ad.Id == "" {
		ad.Id = uuid.NewV4().String()
	}

	if ad.TimeToLive == 0 {
		ad.TimeToLive = DefaultTTL
	}

	p.opChan <- removeAdver{ad}
	return nil
}

func (p *Peer) AllAdvertisements() ([]*pb.Advertisement, error) {
	p.adverLock.Lock()
	defer p.adverLock.Unlock()

	var all []*pb.Advertisement

	for _, ad := range p.advers {
		all = append(all, ad.adver)
	}

	return all, nil
}

func (p *Peer) pruneNeighAdvers(neigh mesh.Identity) {
	p.adverLock.Lock()
	defer p.adverLock.Unlock()

	var toDelete []string

	for id, ad := range p.advers {
		if ad.adver.Owner.Equal(neigh) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		delete(p.advers, id)
	}
}

type AdvertisementOps interface {
	GetAllAdvertisements(ctx context.Context, neigh mesh.Identity) (*pb.AdvertisementSet, error)
	SyncAdvertisements(ctx context.Context, neigh mesh.Identity, update *pb.AdvertisementUpdate) error
}

func (p *Peer) getAllAds(ctx context.Context, neigh mesh.Identity) {
	set, err := p.adverOps.GetAllAdvertisements(ctx, neigh)
	if err != nil {
		p.opChan <- rpcError{"GetAllAdvertisements", neigh, err}
		return
	}

	p.opChan <- inputAdvers{set.Advers}
}

func (p *Peer) syncAds(ctx context.Context, neigh mesh.Identity, update *pb.AdvertisementUpdate) {
	err := p.adverOps.SyncAdvertisements(ctx, neigh, update)
	if err != nil {
		p.opChan <- rpcError{"SyncAdvertisements", neigh, err}
		return
	}
}

func (p *Peer) rebroadcastAdvers() error {
	p.adverLock.Lock()

	var update pb.AdvertisementUpdate
	update.Origin = p.Identity()

	for _, ad := range p.selfAdvers {
		update.NewAdvers = append(update.NewAdvers, ad)
	}

	log.Debugf("rebroadcastAdvers: broadcast %d self advers", len(p.selfAdvers))

	p.adverLock.Unlock()

	if len(update.NewAdvers) == 0 {
		return nil
	}

	p.neighLock.Lock()

	for _, neigh := range p.neighbors {
		go p.syncAds(p.lifetime, neigh.Id, &update)
	}

	p.neighLock.Unlock()

	return nil
}

func (p *Peer) floodUpdate(up *pb.AdvertisementUpdate) {
	p.neighLock.Lock()

	var (
		sent int
		skip int
	)

	for _, neigh := range p.neighbors {
		if up.Origin.Equal(neigh.Id) {
			continue
		}

		go p.syncAds(p.lifetime, neigh.Id, up)
	}

	log.Debugf("Flooded advert update to %d neighbors (%d skip)", sent, skip)

	p.neighLock.Unlock()
}

var _ pb.ServicesServer = (*Peer)(nil)

func (p *Peer) SyncAdvertisements(ctx context.Context, update *pb.AdvertisementUpdate) (*pb.AdvertisementChanges, error) {
	resp := make(chan *pb.AdvertisementChanges)

	p.opChan <- syncAdsOp{update: update, resp: resp}

	return <-resp, nil
}

func (p *Peer) RetrieveAdvertisements(ctx context.Context, req *pb.RetrieveAdverRequest) (*pb.AdvertisementSet, error) {
	p.adverLock.Lock()
	defer p.adverLock.Unlock()

	var advers []*pb.Advertisement

	for _, add := range p.advers {
		advers = append(advers, add.adver)
	}

	return &pb.AdvertisementSet{advers}, nil
}
