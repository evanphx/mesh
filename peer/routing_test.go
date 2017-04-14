package peer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPeerRouting(t *testing.T) {
	n := neko.Start(t)

	n.It("routes messages to neighbors based on the routing table", func() {
		ab, ba := pairByteTraders()

		a, err := InitNewPeer()
		require.NoError(t, err)

		b, err := InitNewPeer()
		require.NoError(t, err)

		a.AddNeighbor(b.Identity(), ab)
		b.AddNeighbor(a.Identity(), ba)

		ac, ca := pairByteTraders()

		c, err := InitNewPeer()
		require.NoError(t, err)

		a.AddNeighbor(c.Identity(), ac)
		c.AddNeighbor(a.Identity(), ca)

		b.AddRoute(a.Identity(), c.Identity())

		var req Header
		req.Destination = c.Identity()
		req.Sender = b.Identity()
		req.Type = PING
		req.Session = 1

		data, err := req.Marshal()
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = b.handleMessage(ctx, data)
		require.NoError(t, err)

		msg, err := ab.Recv(nil)
		require.NoError(t, err)

		err = a.handleMessage(ctx, msg)
		require.NoError(t, err)

		msg, err = ca.Recv(nil)
		require.NoError(t, err)

		err = c.handleMessage(ctx, msg)
		require.NoError(t, err)
	})

	n.Meow()
}
