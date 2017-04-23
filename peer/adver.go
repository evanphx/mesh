package peer

import (
	"context"

	"github.com/evanphx/mesh/grpc"
)

func (p *Peer) Advertise(ad *Advertisement) error {
	if ad.Owner == nil {
		ad.Owner = p.Identity()
	}

	p.opChan <- introduceAdver{ad}
	return nil
}

func (p *Peer) AllAdvertisements() ([]*Advertisement, error) {
	p.adverLock.Lock()
	defer p.adverLock.Unlock()

	return p.allAdvers, nil
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

var _ ServicesServer = (*Peer)(nil)

func (p *Peer) SyncAdvertisements(ctx context.Context, update *AdvertisementUpdate) (*AdvertisementChanges, error) {
	resp := make(chan *AdvertisementChanges)

	p.opChan <- syncAdsOp{update: update, resp: resp}

	return <-resp, nil
}

func (p *Peer) RetrieveAdvertisements(ctx context.Context, req *RetrieveAdverRequest) (*AdvertisementSet, error) {
	p.adverLock.Lock()
	defer p.adverLock.Unlock()

	return &AdvertisementSet{p.allAdvers}, nil
}
