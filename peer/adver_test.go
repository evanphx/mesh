package peer

import (
	"testing"
	"time"

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

		require.Equal(t, 1, len(ads))

		assert.True(t, len(ads[0].Owner) != 0)

		assert.Equal(t, ad, ads[0])
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

		require.Equal(t, 1, len(ads))

		assert.True(t, len(ads[0].Owner) != 0)

		assert.Equal(t, ad, ads[0])
	})

	n.Meow()
}
