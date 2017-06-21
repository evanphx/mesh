package instance

import (
	"net"

	"github.com/evanphx/mesh/transport"
)

func (i *Instance) ListenTCP(addr string) (*net.TCPAddr, error) {
	a, err := transport.ListenTCP(i.lifetime, i.Peer, i.validator, addr)
	if err != nil {
		return nil, err
	}

	return a.(*net.TCPAddr), nil
}
