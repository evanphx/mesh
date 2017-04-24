package peer

import (
	"testing"
	"time"

	"github.com/evanphx/mesh/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPeerAdver(t *testing.T) {
	n := neko.Modern(t)

	n.It("send out advertisements", func(t *testing.T) {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		ad := &Advertisement{
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

		var found *Advertisement

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
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		ad := &Advertisement{
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

		var found *Advertisement

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
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := r.Listen(&Advertisement{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		defer l.Close()

		r.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		_, _, err = i.lookupSelector(&PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "development",
			},
		})

		require.Error(t, err)

		peer, name, err := i.lookupSelector(&PipeSelector{
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
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := r.Listen(&Advertisement{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		defer l.Close()

		r.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		rb.Close()

		time.Sleep(100 * time.Millisecond)

		_, _, err = i.lookupSelector(&PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.Error(t, err)
	})

	n.It("ignores advers that have their TTL passed", func(t *testing.T) {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := r.Listen(&Advertisement{
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

		_, _, err = i.lookupSelector(&PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.Error(t, err)
	})

	n.It("refreshes TTLs by rebroadcasting advers", func(t *testing.T) {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := r.Listen(&Advertisement{
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

		ad, err := i.lookupAdver(&PipeSelector{
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

		ad, err = i.lookupAdver(&PipeSelector{
			Pipe: "test",
			Tags: map[string]string{
				"env": "production",
			},
		})

		require.NoError(t, err)

		assert.True(t, ad.expiresAt.After(ex1))
	})

	n.It("clears advers when a listener is closed", func(t *testing.T) {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		r.AttachPeer(i.Identity(), rb)
		i.AttachPeer(r.Identity(), ib)

		r.WaitTilIdle()
		i.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		l, err := r.Listen(&Advertisement{
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

		_, _, err = i.lookupSelector(&PipeSelector{
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

		adver := &Advertisement{
			Pipe: "vpn",
		}

		adver.Sign(auth)

		assert.NotEqual(t, 0, len(adver.Signer))
		assert.NotEqual(t, 0, len(adver.Signature))

		assert.True(t, adver.Verify(auth))

		adver2 := &Advertisement{
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

		adver := &Advertisement{
			Owner: Identity(id1.Public),
			Pipe:  "vpn",
		}

		adver.Sign(auth)

		assert.NotEqual(t, 0, len(adver.Signer))
		assert.NotEqual(t, 0, len(adver.Signature))

		assert.True(t, adver.Verify(auth))

		adver2 := &Advertisement{
			Owner:     Identity(id2.Public),
			Pipe:      "vpn",
			Signer:    adver.Signer,
			Signature: adver.Signature,
		}

		assert.False(t, adver2.Verify(auth))
	})
	n.Meow()
}
