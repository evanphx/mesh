package router

import "errors"

type routeEntry struct {
	Neighbor string
}

type Router struct {
	routes map[string]routeEntry
}

func NewRouter() *Router {
	return &Router{
		routes: make(map[string]routeEntry),
	}
}

type Hop struct {
	Neighbor    string
	Destination string
}

var ErrNoRoute = errors.New("no route available")

func (r *Router) Lookup(dest string) (Hop, error) {
	if ent, ok := r.routes[dest]; ok {
		return Hop{
			Neighbor:    ent.Neighbor,
			Destination: dest,
		}, nil
	}

	return Hop{}, ErrNoRoute
}

type Update struct {
	Neighbor    string
	Destination string
}

func (r *Router) Update(u Update) error {
	r.routes[u.Destination] = routeEntry{
		Neighbor: u.Neighbor,
	}

	return nil
}
