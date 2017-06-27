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
	_, err := transport.ConnectTCP(context.TODO(), p, p, u.Host, u.Path)
	return err
}

var DefaultConnections *Connections

func init() {
	conn, err := NewConnections()
	if err != nil {
		panic(err)
	}

	conn.Register("mesh", TCPDialer)

	DefaultConnections = conn
}

func (i *Instance) ConnectTCP(ctx context.Context, addr, net string) error {
	mon, err := transport.ConnectTCP(ctx, i.Peer, i.validator, addr, net)
	if err != nil {
		return err
	}

	mon.OnClose(func(sctx context.Context) error {
		go i.ConnectTCP(ctx, addr, net)
		return nil
	})

	return nil
}
