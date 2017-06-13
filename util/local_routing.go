package util

import (
	"context"
	"fmt"
	"sync"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/pb"
)

type RouteOps interface {
	RoutesSince(ctx context.Context, req *pb.RouteRequest) (*pb.RouteUpdate, error)
	NewRoute(ctx context.Context, req *pb.RouteUpdate) (*pb.NoResponse, error)
}

type RouteNode struct {
	l  *LocalRoutes
	id mesh.Identity
	op RouteOps
}

type LocalRoutes struct {
	mu sync.Mutex

	nodes map[string]*RouteNode
}

func NewLocalRoutes() *LocalRoutes {
	return &LocalRoutes{
		nodes: make(map[string]*RouteNode),
	}
}

func (l *LocalRoutes) AddNode(n mesh.Identity, op RouteOps) *RouteNode {
	l.mu.Lock()
	defer l.mu.Unlock()

	node, ok := l.nodes[n.String()]
	if !ok {
		node = &RouteNode{
			l:  l,
			id: n,
			op: op,
		}

		l.nodes[n.String()] = node
	}

	return node
}

func (l *RouteNode) FetchRoutes(ctx context.Context, neigh mesh.Identity, epoch int64) (*pb.RouteUpdate, error) {
	l.l.mu.Lock()
	defer l.l.mu.Unlock()

	node, ok := l.l.nodes[neigh.String()]
	if !ok {
		return nil, fmt.Errorf("Unknown node: %s", neigh)
	}

	req := &pb.RouteRequest{}
	req.Since = epoch

	return node.op.RoutesSince(ctx, req)
}

func (l *RouteNode) SendRoute(ctx context.Context, neigh mesh.Identity, req *pb.RouteUpdate) error {
	l.l.mu.Lock()
	defer l.l.mu.Unlock()

	node, ok := l.l.nodes[neigh.String()]
	if !ok {
		return fmt.Errorf("Unknown node: %s", neigh)
	}

	_, err := node.op.NewRoute(ctx, req)
	return err
}
