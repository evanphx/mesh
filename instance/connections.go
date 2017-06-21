package instance

import (
	"context"
	"fmt"
	"net/url"

	"github.com/evanphx/mesh/peer"
	"github.com/evanphx/mesh/transport"
)

type DialFunc func(*peer.Peer, *url.URL) error

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

func (i *Instance) ConnectTo(str string) error {
	u, err := url.Parse(str)
	if err != nil {
		return err
	}

	i.lock.Lock()
	df, ok := i.connections.schemes[u.Scheme]
	i.lock.Unlock()

	if !ok {
		return fmt.Errorf("Unknown scheme: %s", u.Scheme)
	}

	return df(i.Peer, u)
}

func TCPDialer(p *peer.Peer, u *url.URL) error {
	return transport.ConnectTCP(context.TODO(), p, p, u.Host, u.Path)
}

func UTPDialer(p *peer.Peer, u *url.URL) error {
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

func (i *Instance) ConnectTCP(ctx context.Context, addr, net string) error {
	return transport.ConnectTCP(ctx, i.Peer, i, addr, net)
}
