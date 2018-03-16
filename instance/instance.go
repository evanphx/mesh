package instance

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/grpc"
	"github.com/evanphx/mesh/peer"
	"github.com/evanphx/mesh/protocol/ping"
	"github.com/evanphx/mesh/protocol/pipe"
	"github.com/evanphx/mesh/transport"
)

type Options struct {
	AdvertiseMDNS bool
}

type Instance struct {
	Peer *peer.Peer

	lock sync.Mutex

	lifetime       context.Context
	lifetimeCancel func()

	pings ping.Handler
	pipes pipe.DataHandler

	rpcServer *grpc.Server

	connections *Connections

	validator transport.Validator

	hasListeners bool

	options Options
}

func InitNew() (*Instance, error) {
	return Init(Options{})
}

func Init(opts Options) (*Instance, error) {
	ctx, cancel := context.WithCancel(context.Background())

	i := &Instance{
		lifetime:       ctx,
		lifetimeCancel: cancel,
		options:        opts,
	}

	i.validator = &AutoCreds{i}

	p, err := peer.InitNew(peer.PeerConfig{
		AdvertisementOps: i,
		RouteOps:         i,
	})

	if err != nil {
		return nil, err
	}

	i.Peer = p

	i.initProtocols()

	i.rpcServer = grpc.NewServer()
	go i.listenForRPC(i.lifetime)

	i.ProvideInfo()

	return i, nil
}

func (i *Instance) initProtocols() {
	i.pings.Setup(1, i.Peer)
	i.pipes.Setup(2, i.Peer, i.Peer.StaticKey())
	i.pipes.SetResolver(i.Peer)

	i.Peer.AddProtocol(1, &i.pings)
	i.Peer.AddProtocol(2, &i.pipes)
}

func (i *Instance) Identity() mesh.Identity {
	return i.Peer.Identity()
}

func (i *Instance) Shutdown() {
	i.Peer.Shutdown()
}

func (i *Instance) StaticTokenAuth(network, token string) {
	i.validator = &StaticCreds{network, token}
}

func (i *Instance) ProvideInfo() {
	ch := make(chan os.Signal, 1)

	signal.Notify(ch, syscall.SIGUSR1)

	go func() {
		for {
			<-ch

			i.Peer.PrintStatus()
		}
	}()
}
