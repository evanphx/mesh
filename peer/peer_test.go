package peer

import (
	"context"
	"testing"
	"time"

	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
	"github.com/evanphx/mesh/protocol/ping"
	"github.com/evanphx/mesh/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPeer(t *testing.T) {
	n := neko.Modern(t)

	n.It("exchanges routes on full start (forced order)", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ab, ba := util.PairByteTraders()

		a, err := InitNewPeer()
		require.NoError(t, err)

		defer a.Shutdown()

		b, err := InitNewPeer()
		require.NoError(t, err)

		defer b.Shutdown()

		bc, cb := util.PairByteTraders()

		c, err := InitNewPeer()
		require.NoError(t, err)

		defer c.Shutdown()

		cd, dc := util.PairByteTraders()

		d, err := InitNewPeer()
		require.NoError(t, err)

		defer d.Shutdown()

		a.adverOps = ld.AddNode(a.Identity(), a)
		b.adverOps = ld.AddNode(b.Identity(), b)
		c.adverOps = ld.AddNode(c.Identity(), c)
		d.adverOps = ld.AddNode(d.Identity(), d)

		a.routeOps = lr.AddNode(a.Identity(), a)
		b.routeOps = lr.AddNode(b.Identity(), b)
		c.routeOps = lr.AddNode(c.Identity(), c)
		d.routeOps = lr.AddNode(d.Identity(), d)

		a.AttachPeer(b.Identity(), ab)
		time.Sleep(100 * time.Millisecond)
		b.AttachPeer(a.Identity(), ba)
		time.Sleep(100 * time.Millisecond)
		b.AttachPeer(c.Identity(), bc)
		time.Sleep(100 * time.Millisecond)
		c.AttachPeer(b.Identity(), cb)
		time.Sleep(100 * time.Millisecond)
		c.AttachPeer(d.Identity(), cd)
		time.Sleep(100 * time.Millisecond)
		d.AttachPeer(c.Identity(), dc)
		time.Sleep(100 * time.Millisecond)

		hop, err := d.router.Lookup(a.Identity().String())
		require.NoError(t, err)

		assert.Equal(t, hop.Neighbor, c.Identity().String())
	})

	n.It("sends out route updates on new attachments", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ab, ba := util.PairByteTraders()

		a, err := InitNewPeer()
		require.NoError(t, err)
		a.Name = "a"

		defer a.Shutdown()

		b, err := InitNewPeer()
		require.NoError(t, err)
		b.Name = "b"

		defer b.Shutdown()

		bc, cb := util.PairByteTraders()

		c, err := InitNewPeer()
		require.NoError(t, err)
		c.Name = "c"

		defer c.Shutdown()

		cd, dc := util.PairByteTraders()

		d, err := InitNewPeer()
		require.NoError(t, err)
		d.Name = "d"

		defer d.Shutdown()

		a.adverOps = ld.AddNode(a.Identity(), a)
		b.adverOps = ld.AddNode(b.Identity(), b)
		c.adverOps = ld.AddNode(c.Identity(), c)
		d.adverOps = ld.AddNode(d.Identity(), d)

		a.routeOps = lr.AddNode(a.Identity(), a)
		b.routeOps = lr.AddNode(b.Identity(), b)
		c.routeOps = lr.AddNode(c.Identity(), c)
		d.routeOps = lr.AddNode(d.Identity(), d)

		b.AttachPeer(c.Identity(), bc)
		c.AttachPeer(b.Identity(), cb)
		c.AttachPeer(d.Identity(), cd)
		d.AttachPeer(c.Identity(), dc)

		time.Sleep(1000 * time.Millisecond)

		log.Printf("==== ATTACH %s to %s =====", b.Desc(), a.Desc())
		log.Printf("==== Track the flow to %s", d.Desc())

		a.AttachPeer(b.Identity(), ab)
		b.AttachPeer(a.Identity(), ba)

		time.Sleep(2000 * time.Millisecond)

		log.Printf("a: %s", a.Identity().Short())
		log.Printf("b: %s", b.Identity().Short())
		log.Printf("c: %s", c.Identity().Short())
		log.Printf("d: %s", d.Identity().Short())

		a.WaitTilIdle()
		b.WaitTilIdle()
		c.WaitTilIdle()
		d.WaitTilIdle()

		hop, err := d.router.Lookup(a.Identity().String())
		require.NoError(t, err)

		assert.Equal(t, hop.Neighbor, c.Identity().String())
	})

	n.It("routes messages between peers", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ab, ba := util.PairByteTraders()

		a, err := InitNewPeer()
		require.NoError(t, err)
		a.Name = "a"

		defer a.Shutdown()

		b, err := InitNewPeer()
		require.NoError(t, err)
		b.Name = "b"

		a.adverOps = ld.AddNode(a.Identity(), a)
		b.adverOps = ld.AddNode(b.Identity(), b)

		a.routeOps = lr.AddNode(a.Identity(), a)
		b.routeOps = lr.AddNode(b.Identity(), b)

		var pa ping.Handler
		var pb ping.Handler

		pa.Setup(1, a)
		pb.Setup(1, b)

		a.AddProtocol(1, &pa)
		b.AddProtocol(1, &pb)

		defer b.Shutdown()

		a.AttachPeer(b.Identity(), ab)
		time.Sleep(100 * time.Millisecond)
		b.AttachPeer(a.Identity(), ba)
		time.Sleep(100 * time.Millisecond)

		ctx := context.Background()

		dur, err := pa.Ping(ctx, b.Identity())
		require.NoError(t, err)

		t.Log(dur)

		assert.True(t, dur > 0)
	})

	n.It("broadcasts advertisments", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ab, ba := util.PairByteTraders()

		a, err := InitNewPeer()
		require.NoError(t, err)
		a.Name = "a"

		defer a.Shutdown()

		b, err := InitNewPeer()
		require.NoError(t, err)
		b.Name = "b"

		a.adverOps = ld.AddNode(a.Identity(), a)
		b.adverOps = ld.AddNode(b.Identity(), b)

		a.routeOps = lr.AddNode(a.Identity(), a)
		b.routeOps = lr.AddNode(b.Identity(), b)

		defer b.Shutdown()

		a.AttachPeer(b.Identity(), ab)
		time.Sleep(100 * time.Millisecond)
		b.AttachPeer(a.Identity(), ba)
		time.Sleep(100 * time.Millisecond)

		ad := &pb.Advertisement{}
		ad.Pipe = "test"

		err = a.Advertise(ad)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		ads, err := b.AllAdvertisements()
		require.NoError(t, err)

		require.True(t, len(ads) > 0)

		assert.Equal(t, "test", ads[0].Pipe)
	})

	n.It("drops adverts when a connection drops", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ab, ba := util.PairByteTraders()

		a, err := InitNewPeer()
		require.NoError(t, err)
		a.Name = "a"

		defer a.Shutdown()

		b, err := InitNewPeer()
		require.NoError(t, err)
		b.Name = "b"

		a.adverOps = ld.AddNode(a.Identity(), a)
		b.adverOps = ld.AddNode(b.Identity(), b)

		a.routeOps = lr.AddNode(a.Identity(), a)
		b.routeOps = lr.AddNode(b.Identity(), b)

		defer b.Shutdown()

		a.AttachPeer(b.Identity(), ab)
		time.Sleep(100 * time.Millisecond)
		b.AttachPeer(a.Identity(), ba)
		time.Sleep(100 * time.Millisecond)

		ad := &pb.Advertisement{}
		ad.Pipe = "test"

		err = a.Advertise(ad)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		ads, err := b.AllAdvertisements()
		require.NoError(t, err)

		require.True(t, len(ads) > 0)

		assert.Equal(t, "test", ads[0].Pipe)

		ctx := context.Background()

		ab.Close(ctx)

		time.Sleep(100 * time.Millisecond)

		ads, err = b.AllAdvertisements()
		require.NoError(t, err)
		require.True(t, len(ads) == 0)
	})

	n.Meow()
}
