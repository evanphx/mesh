package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestRouter(t *testing.T) {
	n := neko.Start(t)

	n.It("updates the routing table using updates", func() {
		router := NewRouter()

		id := "xxyyzz"
		neigh := "aabbcc"

		_, err := router.Lookup(id)
		require.Error(t, err)

		err = router.Update(Update{
			Neighbor:    neigh,
			Destination: id,
		})

		require.NoError(t, err)

		hop, err := router.Lookup(id)
		require.NoError(t, err)

		assert.Equal(t, neigh, hop.Neighbor)
	})

	n.Meow()
}
