package peer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPeerRouting(t *testing.T) {
	n := neko.Start(t)

	n.It("routes messages to neighbors based on the routing table", func() {
		ab, ba := pairByteTraders()

		a, err := InitNewPeer()
		require.NoError(t, err)

		a.Name = "a"

		defer a.Shutdown()

		b, err := InitNewPeer()
		require.NoError(t, err)

		b.Name = "b"

		defer b.Shutdown()

		a.AttachPeer(b.Identity(), ab)
		b.AttachPeer(a.Identity(), ba)

		ac, ca := pairByteTraders()

		c, err := InitNewPeer()
		require.NoError(t, err)

		c.Name = "c"

		a.AttachPeer(c.Identity(), ac)
		c.AttachPeer(a.Identity(), ca)

		b.WaitTilIdle()
		a.WaitTilIdle()
		c.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dur, err := b.Ping(ctx, c.Identity())
		require.NoError(t, err)

		assert.True(t, dur > 0)
	})

	n.It("can get route updates from a peer", func() {
		ab, ba := pairByteTraders()

		a, err := InitNewPeer()
		require.NoError(t, err)

		defer a.Shutdown()

		b, err := InitNewPeer()
		require.NoError(t, err)

		defer b.Shutdown()

		a.AttachPeer(b.Identity(), ab)
		b.AttachPeer(a.Identity(), ba)

		ac, ca := pairByteTraders()

		c, err := InitNewPeer()
		require.NoError(t, err)

		defer c.Shutdown()

		a.AttachPeer(c.Identity(), ac)
		c.AttachPeer(a.Identity(), ca)

		time.Sleep(100 * time.Millisecond)

		a.WaitTilIdle()
		b.WaitTilIdle()
		c.WaitTilIdle()

		hop, err := b.router.Lookup(c.Identity().String())
		require.NoError(t, err)

		assert.Equal(t, hop.Neighbor, a.Identity().String())

	})

	n.Meow()
}
