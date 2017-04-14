package peer

import (
	"sync"
	"testing"

	"github.com/evanphx/mesh/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPeer(t *testing.T) {
	n := neko.Start(t)

	n.It("negotates an encrypted session", func() {
		ib, rb := pairByteTraders()

		a, err := auth.NewAuthorizer("test")
		require.NoError(t, err)

		i, err := InitNewPeer()
		require.NoError(t, err)

		i.Authorize(a)

		r, err := InitNewPeer()
		require.NoError(t, err)

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

	n.It("errors out if the peers aren't authorized", func() {
		ib, rb := pairByteTraders()

		a, err := auth.NewAuthorizer("t1")
		require.NoError(t, err)

		b, err := auth.NewAuthorizer("t1")
		require.NoError(t, err)

		i, err := InitNewPeer()
		require.NoError(t, err)

		i.Authorize(a)

		r, err := InitNewPeer()
		require.NoError(t, err)

		r.Authorize(b)

		var wg sync.WaitGroup

		wg.Add(2)

		go func() {
			defer wg.Done()
			defer ib.Close()

			var err error

			_, err = i.BeginHandshake("t1", ib)
			assert.Equal(t, err, ErrInvalidCred)
		}()

		go func() {
			defer wg.Done()
			defer rb.Close()

			var err error

			_, err = r.WaitHandshake(rb)
			assert.Equal(t, err, ErrInvalidCred)
		}()

		wg.Wait()
	})

	n.Meow()
}
