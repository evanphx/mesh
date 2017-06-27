package instance

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/evanphx/mesh/peer"
	"github.com/evanphx/mesh/transport"
	"github.com/evanphx/mesh/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func noTestPeerConnections(t *testing.T) {
	n := neko.Modern(t)

	n.It("allows peers to directly connect to eachother", func(t *testing.T) {

		i, err := InitNew()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNew()
		require.NoError(t, err)

		defer r.Shutdown()

		cm, err := NewConnections()
		require.NoError(t, err)

		cm.Register("inmem", func(p *peer.Peer, u *url.URL) error {
			switch u.Host {
			case "i":
				ib, rb := util.PairByteTraders()
				p.AttachPeer(i.Identity(), ib)
				i.Peer.AttachPeer(p.Identity(), rb)
				return nil
			default:
				return fmt.Errorf("unknown host: %s", u.Host)
			}
		})

		r.connections = cm

		err = r.ConnectTo("inmem://i")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		hop, err := i.Peer.LookupRoute(r.Identity())
		require.NoError(t, err)

		assert.Equal(t, hop.Neighbor, r.Identity().String())
	})

	n.It("has a default tcp protocol", func(t *testing.T) {
		i, err := InitNew()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNew()
		require.NoError(t, err)

		defer r.Shutdown()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		addr, err := transport.ListenTCP(ctx, i.Peer, i.Peer, ":0")
		require.NoError(t, err)

		t.Logf("connect to: %s", addr.String())

		err = r.ConnectTo("mesh://" + addr.String())
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		hop, err := i.Peer.LookupRoute(r.Identity())
		require.NoError(t, err)

		assert.Equal(t, hop.Neighbor, r.Identity().String())
	})

	n.Meow()
}
