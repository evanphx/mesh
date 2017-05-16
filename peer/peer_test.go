package peer

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/evanphx/mesh/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPeer(t *testing.T) {
	n := neko.Start(t)

	n.NIt("negotates an encrypted session", func() {
		ib, rb := pairByteTraders()

		a, err := auth.NewAuthorizer("test")
		require.NoError(t, err)

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		i.Authorize(a)

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		r.Authorize(a)

		var (
			is Session
			rs Session

			ipeer Identity
			rpeer Identity
		)

		var wg sync.WaitGroup

		wg.Add(2)

		go func() {
			defer wg.Done()

			var err error

			is, err = i.BeginHandshake("test", ib)
			require.NoError(t, err)

			ipeer = is.PeerIdentity()
		}()

		go func() {
			defer wg.Done()

			var err error

			rs, err = r.WaitHandshake(rb)
			require.NoError(t, err)

			rpeer = rs.PeerIdentity()
		}()

		wg.Wait()

		ct := is.Encrypt([]byte("hello"), nil)
		pt, err := rs.Decrypt(ct, nil)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(pt))

		assert.True(t, ipeer.Equal(r.Identity()))
		assert.True(t, rpeer.Equal(i.Identity()))
	})

	n.NIt("errors out if the peers aren't authorized", func() {
		ib, rb := pairByteTraders()

		a, err := auth.NewAuthorizer("t1")
		require.NoError(t, err)

		b, err := auth.NewAuthorizer("t1")
		require.NoError(t, err)

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		i.Authorize(a)

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		r.Authorize(b)

		var wg sync.WaitGroup

		wg.Add(2)

		go func() {
			defer wg.Done()
			defer ib.Close(context.TODO())

			var err error

			_, err = i.BeginHandshake("t1", ib)
			assert.Equal(t, err, ErrInvalidCred)
		}()

		go func() {
			defer wg.Done()
			defer rb.Close(context.TODO())

			var err error

			_, err = r.WaitHandshake(rb)
			assert.Equal(t, err, ErrInvalidCred)
		}()

		wg.Wait()
	})

	n.It("exchanges routes on full start (forced order)", func() {
		ab, ba := pairByteTraders()

		a, err := InitNewPeer()
		require.NoError(t, err)

		defer a.Shutdown()

		b, err := InitNewPeer()
		require.NoError(t, err)

		defer b.Shutdown()

		bc, cb := pairByteTraders()

		c, err := InitNewPeer()
		require.NoError(t, err)

		defer c.Shutdown()

		cd, dc := pairByteTraders()

		d, err := InitNewPeer()
		require.NoError(t, err)

		defer d.Shutdown()

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

	n.It("sends out route updates on new attachments", func() {
		ab, ba := pairByteTraders()

		a, err := InitNewPeer()
		require.NoError(t, err)
		a.Name = "a"

		defer a.Shutdown()

		b, err := InitNewPeer()
		require.NoError(t, err)
		b.Name = "b"

		defer b.Shutdown()

		bc, cb := pairByteTraders()

		c, err := InitNewPeer()
		require.NoError(t, err)
		c.Name = "c"

		defer c.Shutdown()

		cd, dc := pairByteTraders()

		d, err := InitNewPeer()
		require.NoError(t, err)
		d.Name = "d"

		defer d.Shutdown()

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

	n.Meow()
}
