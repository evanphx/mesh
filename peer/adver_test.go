package peer

import (
	"context"
	"testing"
	"time"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/auth"
	"github.com/evanphx/mesh/pb"
	"github.com/evanphx/mesh/protocol/pipe"
	"github.com/evanphx/mesh/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPeerAdver(t *testing.T) {
	n := neko.Modern(t)

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

	n.It("send out advertisements", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ib, rb := util.PairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.adverOps = ld.AddNode(i.Identity(), i)
		r.adverOps = ld.AddNode(r.Identity(), r)

		i.routeOps = lr.AddNode(i.Identity(), i)
		r.routeOps = lr.AddNode(r.Identity(), r)

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		ad := &pb.Advertisement{
			Pipe: "test",
			Tags: map[string]string{
				"env": "prod",
			},
			Resources: map[string]string{
				"subnet": "10.181.0.0/16",
			},
		}

		err = i.Advertise(ad)
		require.NoError(t, err)

		i.WaitTilIdle()
		r.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		ads, err := r.AllAdvertisements()
		require.NoError(t, err)

		var found *pb.Advertisement

		for _, a := range ads {
			if a.Pipe == "test" {
				found = a
				break
			}
		}

		require.NotNil(t, found)

		assert.True(t, len(ad.Owner) != 0)

		assert.Equal(t, ad, found)
	})

	n.It("retrieves neighbors advers on start", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ib, rb := util.PairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		ad := &pb.Advertisement{
			Pipe: "test",
			Tags: map[string]string{
				"env": "prod",
			},
			Resources: map[string]string{
				"subnet": "10.181.0.0/16",
			},
		}

		err = i.Advertise(ad)
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.adverOps = ld.AddNode(i.Identity(), i)
		r.adverOps = ld.AddNode(r.Identity(), r)

		i.routeOps = lr.AddNode(i.Identity(), i)
		r.routeOps = lr.AddNode(r.Identity(), r)

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		i.WaitTilIdle()
		r.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		ads, err := r.AllAdvertisements()
		require.NoError(t, err)

		var found *pb.Advertisement

		for _, a := range ads {
			if a.Pipe == "test" {
				found = a
				break
			}
		}

		require.NotNil(t, found)

		assert.True(t, len(ad.Owner) != 0)

		assert.Equal(t, ad, found)
	})

	n.It("advertises complete advers and selects on tags", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ib, rb := util.PairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.adverOps = ld.AddNode(i.Identity(), i)
		r.adverOps = ld.AddNode(r.Identity(), r)

		i.routeOps = lr.AddNode(i.Identity(), i)
		r.routeOps = lr.AddNode(r.Identity(), r)

		var pi pipe.DataHandler
		var pr pipe.DataHandler

		pi.Setup(2, i, i.identityKey)
		pr.Setup(2, r, r.identityKey)

		pi.SetResolver(i)
		pr.SetResolver(r)

		i.AddProtocol(2, &pi)
		r.AddProtocol(2, &pr)

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := pr.Listen(&pb.Advertisement{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		defer l.Close()

		r.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		_, _, err = i.LookupSelector(&mesh.PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "development",
			},
		})

		require.Error(t, err)

		peer, name, err := i.LookupSelector(&mesh.PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		assert.Equal(t, "test", name)
		assert.Equal(t, r.Identity(), peer)
	})

	n.It("clears advers for owners that go offline", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ib, rb := util.PairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.adverOps = ld.AddNode(i.Identity(), i)
		r.adverOps = ld.AddNode(r.Identity(), r)

		i.routeOps = lr.AddNode(i.Identity(), i)
		r.routeOps = lr.AddNode(r.Identity(), r)

		var pi pipe.DataHandler
		var pr pipe.DataHandler

		pi.Setup(2, i, i.identityKey)
		pr.Setup(2, r, r.identityKey)

		i.AddProtocol(2, &pi)
		r.AddProtocol(2, &pr)

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := pr.Listen(&pb.Advertisement{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		defer l.Close()

		r.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		rb.Close(context.TODO())

		time.Sleep(100 * time.Millisecond)

		_, _, err = i.LookupSelector(&mesh.PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.Error(t, err)
	})

	n.It("ignores advers that have their TTL passed", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ib, rb := util.PairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.adverOps = ld.AddNode(i.Identity(), i)
		r.adverOps = ld.AddNode(r.Identity(), r)

		i.routeOps = lr.AddNode(i.Identity(), i)
		r.routeOps = lr.AddNode(r.Identity(), r)

		var pi pipe.DataHandler
		var pr pipe.DataHandler

		pi.Setup(2, i, i.identityKey)
		pr.Setup(2, r, r.identityKey)

		i.AddProtocol(2, &pi)
		r.AddProtocol(2, &pr)

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := pr.Listen(&pb.Advertisement{
			TimeToLive: 1,
			Pipe:       "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		defer l.Close()

		r.WaitTilIdle()

		time.Sleep(1500 * time.Millisecond)

		_, _, err = i.LookupSelector(&mesh.PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.Error(t, err)
	})

	n.It("refreshes TTLs by rebroadcasting advers", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ib, rb := util.PairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.adverOps = ld.AddNode(i.Identity(), i)
		r.adverOps = ld.AddNode(r.Identity(), r)

		i.routeOps = lr.AddNode(i.Identity(), i)
		r.routeOps = lr.AddNode(r.Identity(), r)

		var pi pipe.DataHandler
		var pr pipe.DataHandler

		pi.Setup(2, i, i.identityKey)
		pr.Setup(2, r, r.identityKey)

		pi.SetResolver(i)
		pr.SetResolver(r)

		i.AddProtocol(2, &pi)
		r.AddProtocol(2, &pr)

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := pr.Listen(&pb.Advertisement{
			TimeToLive: 300,
			Pipe:       "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		defer l.Close()

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		ad, err := i.lookupAdver(&mesh.PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		ex1 := ad.expiresAt

		time.Sleep(1 * time.Second)

		err = r.rebroadcastAdvers()
		require.NoError(t, err)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		ad, err = i.lookupAdver(&mesh.PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		assert.True(t, ad.expiresAt.After(ex1))
	})

	n.It("clears advers when a listener is closed", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ib, rb := util.PairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.adverOps = ld.AddNode(i.Identity(), i)
		r.adverOps = ld.AddNode(r.Identity(), r)

		i.routeOps = lr.AddNode(i.Identity(), i)
		r.routeOps = lr.AddNode(r.Identity(), r)

		var pi pipe.DataHandler
		var pr pipe.DataHandler

		pi.Setup(2, i, i.identityKey)
		pr.Setup(2, r, r.identityKey)

		i.AddProtocol(2, &pi)
		r.AddProtocol(2, &pr)

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := pr.Listen(&pb.Advertisement{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l.Close()

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		_, _, err = i.LookupSelector(&mesh.PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.Error(t, err)
	})

	n.It("can be signed and verified", func(t *testing.T) {
		auth, err := auth.NewAuthorizer("test")
		require.NoError(t, err)

		adver := &pb.Advertisement{
			Pipe: "vpn",
		}

		adver.Sign(auth)

		assert.NotEqual(t, 0, len(adver.Signer))
		assert.NotEqual(t, 0, len(adver.Signature))

		assert.True(t, adver.Verify(auth))

		adver2 := &pb.Advertisement{
			Pipe:      "blah",
			Signer:    adver.Signer,
			Signature: adver.Signature,
		}

		assert.False(t, adver2.Verify(auth))
	})

	n.It("verifies the owner", func(t *testing.T) {
		id1 := CipherSuite.GenerateKeypair(RNG)
		id2 := CipherSuite.GenerateKeypair(RNG)

		auth, err := auth.NewAuthorizer("test")
		require.NoError(t, err)

		adver := &pb.Advertisement{
			Owner: mesh.Identity(id1.Public),
			Pipe:  "vpn",
		}

		adver.Sign(auth)

		assert.NotEqual(t, 0, len(adver.Signer))
		assert.NotEqual(t, 0, len(adver.Signature))

		assert.True(t, adver.Verify(auth))

		adver2 := &pb.Advertisement{
			Owner:     mesh.Identity(id2.Public),
			Pipe:      "vpn",
			Signer:    adver.Signer,
			Signature: adver.Signature,
		}

		assert.False(t, adver2.Verify(auth))
	})

	n.It("removes very stale advers", func(t *testing.T) {
		ld := util.NewLocalAdvertisements()
		lr := util.NewLocalRoutes()

		ib, rb := util.PairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.adverOps = ld.AddNode(i.Identity(), i)
		r.adverOps = ld.AddNode(r.Identity(), r)

		i.routeOps = lr.AddNode(i.Identity(), i)
		r.routeOps = lr.AddNode(r.Identity(), r)

		var pi pipe.DataHandler
		var pr pipe.DataHandler

		pi.Setup(2, i, i.identityKey)
		pr.Setup(2, r, r.identityKey)

		i.AddProtocol(2, &pi)
		r.AddProtocol(2, &pr)

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := pr.Listen(&pb.Advertisement{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l.Close()

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		i.adverLock.Lock()
		for _, ad := range i.advers {
			ad.expiresAt = time.Now().Add(-6 * time.Minute)
		}
		i.adverLock.Unlock()

		i.pruneStaleAdvers()

		i.adverLock.Lock()
		cnt := len(i.advers)
		i.adverLock.Unlock()

		assert.Equal(t, 0, cnt)

	})

	n.Meow()
}
