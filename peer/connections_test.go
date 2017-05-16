package peer

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/evanphx/mesh/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPeerConnections(t *testing.T) {
	n := neko.Modern(t)

	n.It("allows peers to directly connect to eachother", func(t *testing.T) {

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		cm, err := NewConnections()
		require.NoError(t, err)

		cm.Register("inmem", func(p *Peer, u *url.URL) error {
			switch u.Host {
			case "i":
				ib, rb := pairByteTraders()
				p.AttachPeer(i.Identity(), ib)
				i.AttachPeer(p.Identity(), rb)
				return nil
			default:
				return fmt.Errorf("unknown host: %s", u.Host)
			}
		})

		r.connections = cm

		err = r.ConnectTo("inmem://i")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		hop, err := i.router.Lookup(r.Identity().String())
		require.NoError(t, err)

		assert.Equal(t, hop.Neighbor, r.Identity().String())
	})

	n.It("has a default tcp protocol", func(t *testing.T) {
		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		addr, err := transport.ListenTCP(ctx, i, i, ":0")
		require.NoError(t, err)

		t.Logf("connect to: %s", addr.String())

		err = r.ConnectTo("mesh://" + addr.String())
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		hop, err := i.router.Lookup(r.Identity().String())
		require.NoError(t, err)

		assert.Equal(t, hop.Neighbor, r.Identity().String())
	})

	n.Only("has a default utp protocol", func(t *testing.T) {
		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		addr, err := transport.ListenUTP(ctx, i, i, "[::1]:0")
		require.NoError(t, err)

		t.Logf("connect to: %s", addr.String())

		err = r.ConnectTo("mesh+utp://" + addr.String())
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		hop, err := i.router.Lookup(r.Identity().String())
		require.NoError(t, err)

		assert.Equal(t, hop.Neighbor, r.Identity().String())
	})

	n.Meow()
}
