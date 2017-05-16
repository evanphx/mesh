package peer

import (
	"context"
	"fmt"
	"net/url"

	"github.com/evanphx/mesh/transport"
)

type DialFunc func(*Peer, *url.URL) error

type Connections struct {
	schemes map[string]DialFunc
}

func NewConnections() (*Connections, error) {
	return &Connections{
		make(map[string]DialFunc),
	}, nil
}

func (c *Connections) Register(scheme string, f DialFunc) {
	c.schemes[scheme] = f
}

func (p *Peer) ConnectTo(str string) error {
	u, err := url.Parse(str)
	if err != nil {
		return err
	}

	p.lock.Lock()
	df, ok := p.connections.schemes[u.Scheme]
	p.lock.Unlock()

	if !ok {
		return fmt.Errorf("Unknown scheme: %s", u.Scheme)
	}

	return df(p, u)
}

func TCPDialer(p *Peer, u *url.URL) error {
	return transport.ConnectTCP(context.TODO(), p, p, u.Host, u.Path)
}

func UTPDialer(p *Peer, u *url.URL) error {
	return transport.ConnectUTP(context.TODO(), p, p, u.Host, u.Path)
}

var DefaultConnections *Connections

func init() {
	conn, err := NewConnections()
	if err != nil {
		panic(err)
	}

	conn.Register("mesh", TCPDialer)
	conn.Register("mesh+utp", UTPDialer)

	DefaultConnections = conn
}
