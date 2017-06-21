package instance

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/pb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestInstance(t *testing.T) {
	n := neko.Modern(t)

	n.It("starts a configured peer", func(t *testing.T) {
		a, err := InitNew()
		require.NoError(t, err)

		defer a.Shutdown()

		b, err := InitNew()
		require.NoError(t, err)

		defer b.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		addr, err := a.ListenTCP(":0")
		require.NoError(t, err)

		err = b.ConnectTCP(ctx, addr.String(), "test")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		dur, err := a.Ping(ctx, b.Identity())
		require.NoError(t, err)

		assert.True(t, dur > 0)

		t.Log(dur)
	})

	n.It("can listen on pipes and find them via adverts", func(t *testing.T) {
		a, err := InitNew()
		require.NoError(t, err)

		defer a.Shutdown()

		b, err := InitNew()
		require.NoError(t, err)

		defer b.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		addr, err := a.ListenTCP(":0")
		require.NoError(t, err)

		err = b.ConnectTCP(ctx, addr.String(), "test")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		var ad pb.Advertisement
		ad.Pipe = "hello"

		lc, err := a.Listen(&ad)
		require.NoError(t, err)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p, err := lc.Accept(ctx)
			if err != nil {
				return
			}

			p.Send(ctx, []byte("hello"))
		}()

		time.Sleep(100 * time.Millisecond)

		var sel mesh.PipeSelector
		sel.Pipe = "hello"

		pipe, err := b.Connect(ctx, &sel)
		require.NoError(t, err)

		msg, err := pipe.Recv(ctx)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(msg))
	})

	n.Meow()
}
