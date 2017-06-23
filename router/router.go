package router

import (
	"errors"
	"sync"
	"time"

	"github.com/evanphx/mesh/log"
)

type routeEntry struct {
	Neighbor  string
	Weight    int
	UpdatedAt int64
}

type Router struct {
	lock   sync.Mutex
	routes map[string]map[string]routeEntry
}

func NewRouter() *Router {
	return &Router{
		routes: make(map[string]map[string]routeEntry),
	}
}

type Hop struct {
	Neighbor    string
	Destination string
	Weight      int
}

var ErrNoRoute = errors.New("no route available")

func (r *Router) Lookup(dest string) (Hop, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	neighbors, ok := r.routes[dest]
	if !ok {
		return Hop{}, ErrNoRoute
	}

	var found routeEntry

	found.Weight = -1

	for _, ent := range neighbors {
		if found.Weight == -1 || ent.Weight < found.Weight {
			found = ent
		}
	}

	if len(neighbors) > 1 {
		log.Debugf("picked from %d routes", len(neighbors))
		for _, ent := range neighbors {
			log.Debugf("route: => %s (%d)", ent.Neighbor, ent.Weight)
		}
	}

	if found.Neighbor == "" {
		return Hop{}, ErrNoRoute
	}

	return Hop{
		Neighbor:    found.Neighbor,
		Weight:      found.Weight,
		Destination: dest,
	}, nil
}

type Update struct {
	Neighbor    string
	Destination string
	Weight      int
	Prune       bool
}

func (r *Router) Update(u Update) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if u.Prune {
		neighbors, ok := r.routes[u.Destination]
		if !ok {
			return nil
		}

		delete(neighbors, u.Neighbor)
		return nil
	}

	neighbors, ok := r.routes[u.Destination]
	if !ok {
		neighbors = make(map[string]routeEntry)
		r.routes[u.Destination] = neighbors
	}

	neighbors[u.Neighbor] = routeEntry{
		Neighbor:  u.Neighbor,
		Weight:    u.Weight,
		UpdatedAt: time.Now().UnixNano(),
	}

	return nil
}

func (r *Router) RoutesSince(offset int64) []Hop {
	r.lock.Lock()
	defer r.lock.Unlock()

	var hops []Hop

	for dest, neighbors := range r.routes {
		for _, ent := range neighbors {
			if ent.UpdatedAt > offset {
				hops = append(hops, Hop{
					Neighbor:    ent.Neighbor,
					Weight:      ent.Weight,
					Destination: dest,
				})
			}
		}
	}

	return hops
}

func (r *Router) PruneByHop(target string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	for _, neighbors := range r.routes {
		delete(neighbors, target)
	}
}
