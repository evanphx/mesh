package peer

import (
	"context"
	"time"

	"github.com/evanphx/mesh/grpc"
	"github.com/satori/go.uuid"
)

const DefaultTTL = 300

type localAdver struct {
	expiresAt time.Time
	adver     *Advertisement
}

func (l *localAdver) valid(t time.Time) bool {
	if l.expiresAt.IsZero() {
		return true
	}

	return t.Before(l.expiresAt)
}

func (a *Advertisement) ExpiresAt() time.Time {
	ttl := a.TimeToLive
	if a.TimeToLive == 0 {
		ttl = DefaultTTL
	}

	return time.Now().Add(time.Duration(ttl) * time.Second)
}

func (a *Advertisement) Sign(s Signer) {
	a.Signer = nil
	a.Signature = nil

	data, err := a.Marshal()
	if err != nil {
		panic(err)
	}

	a.Signer, a.Signature = s.Sign(data)
}

func (a *Advertisement) Verify(v Verifier) bool {
	var cp Advertisement = *a

	cp.Signer = nil
	cp.Signature = nil

	data, err := cp.Marshal()
	if err != nil {
		return false
	}

	return v.VerifySigner(a.Signer, a.Signature, data)
}

func (p *Peer) Advertise(ad *Advertisement) error {
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

func (p *Peer) AllAdvertisements() ([]*Advertisement, error) {
	p.adverLock.Lock()
	defer p.adverLock.Unlock()

	var all []*Advertisement

	for _, ad := range p.advers {
		all = append(all, ad.adver)
	}

	return all, nil
}

func (p *Peer) pruneNeighAdvers(neigh Identity) {
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

func (p *Peer) getAllAds(ctx context.Context, neigh Identity) {
	pipe, err := p.ConnectPipe(ctx, neigh, ":rpc")
	if err != nil {
		p.opChan <- rpcError{neigh, err}
	}

	defer pipe.Close()

	client := NewServicesClient(grpc.NewClientConn(pipe))

	set, err := client.RetrieveAdvertisements(ctx, &RetrieveAdverRequest{})
	if err != nil {
		p.opChan <- rpcError{neigh, err}
	}

	p.opChan <- inputAdvers{set.Advers}
}

func (p *Peer) syncAds(ctx context.Context, neigh Identity, update *AdvertisementUpdate) {
	pipe, err := p.ConnectPipe(ctx, neigh, ":rpc")
	if err != nil {
		p.opChan <- rpcError{neigh, err}
	}

	defer pipe.Close()

	client := NewServicesClient(grpc.NewClientConn(pipe))

	_, err = client.SyncAdvertisements(ctx, update)
	if err != nil {
		p.opChan <- rpcError{neigh, err}
	}
}

func (p *Peer) rebroadcastAdvers() error {
	p.adverLock.Lock()

	var update AdvertisementUpdate

	for _, ad := range p.selfAdvers {
		update.NewAdvers = append(update.NewAdvers, ad)
	}

	p.adverLock.Unlock()
	p.neighLock.Lock()

	for _, neigh := range p.neighbors {
		go p.syncAds(p.lifetime, neigh.Id, &update)
	}

	p.neighLock.Unlock()

	return nil
}

var _ ServicesServer = (*Peer)(nil)

func (p *Peer) SyncAdvertisements(ctx context.Context, update *AdvertisementUpdate) (*AdvertisementChanges, error) {
	resp := make(chan *AdvertisementChanges)

	p.opChan <- syncAdsOp{update: update, resp: resp}

	return <-resp, nil
}

func (p *Peer) RetrieveAdvertisements(ctx context.Context, req *RetrieveAdverRequest) (*AdvertisementSet, error) {
	p.adverLock.Lock()
	defer p.adverLock.Unlock()

	var advers []*Advertisement

	for _, add := range p.advers {
		advers = append(advers, add.adver)
	}

	return &AdvertisementSet{advers}, nil
}
