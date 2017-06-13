package util

import (
	"context"
	"fmt"
	"sync"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/pb"
)

type AdvertisementOps interface {
	SyncAdvertisements(ctx context.Context, update *pb.AdvertisementUpdate) (*pb.AdvertisementChanges, error)
	RetrieveAdvertisements(ctx context.Context, req *pb.RetrieveAdverRequest) (*pb.AdvertisementSet, error)
}

type AdNode struct {
	l  *LocalAdvertisements
	id mesh.Identity
	op AdvertisementOps
}

type LocalAdvertisements struct {
	mu sync.Mutex

	nodes map[string]*AdNode
}

func NewLocalAdvertisements() *LocalAdvertisements {
	return &LocalAdvertisements{
		nodes: make(map[string]*AdNode),
	}
}

func (l *LocalAdvertisements) AddNode(n mesh.Identity, op AdvertisementOps) *AdNode {
	l.mu.Lock()
	defer l.mu.Unlock()

	node, ok := l.nodes[n.String()]
	if !ok {
		node = &AdNode{
			l:  l,
			id: n,
			op: op,
		}

		l.nodes[n.String()] = node
	}

	return node
}

func (l *AdNode) GetAllAdvertisements(ctx context.Context, neigh mesh.Identity) (*pb.AdvertisementSet, error) {
	l.l.mu.Lock()
	defer l.l.mu.Unlock()

	node, ok := l.l.nodes[neigh.String()]
	if !ok {
		return nil, fmt.Errorf("Unknown node: %s", neigh)
	}

	req := &pb.RetrieveAdverRequest{}

	return node.op.RetrieveAdvertisements(ctx, req)
}

func (l *AdNode) SyncAdvertisements(ctx context.Context, neigh mesh.Identity, update *pb.AdvertisementUpdate) error {
	l.l.mu.Lock()
	defer l.l.mu.Unlock()

	node, ok := l.l.nodes[neigh.String()]
	if !ok {
		return fmt.Errorf("Unknown node: %s", neigh)
	}

	_, err := node.op.SyncAdvertisements(ctx, update)
	return err
}
