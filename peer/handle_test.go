package peer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/evanphx/mesh/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPeerHandle(t *testing.T) {
	n := neko.Start(t)

	n.It("can accept and send on a pipe", func() {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.AddNeighbor(r.Identity(), ib)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		time.Sleep(1 * time.Second)

		listen, err := i.ListenPipe("test")
		require.NoError(t, err)

		var req Header
		req.Destination = i.Identity()
		req.Sender = r.Identity()
		req.Type = PIPE_OPEN
		req.Session = r.sessionId(req.Destination)
		req.PipeName = "test"

		data, err := req.Marshal()
		require.NoError(t, err)

		err = i.handleMessage(ctx, data)
		require.NoError(t, err)

		i.WaitTilIdle()

		log.Debugf("accepting...")

		rp, err := listen.Accept(ctx)
		require.NoError(t, err)

		assert.Equal(t, req.Session, rp.session)

		// rpc request, discard
		_, err = rb.Recv(nil)
		require.NoError(t, err)

		nxt, err := rb.Recv(nil)
		require.NoError(t, err)

		var resp Header

		err = resp.Unmarshal(nxt)
		require.NoError(t, err)

		assert.Equal(t, PIPE_OPENED, resp.Type)

		var req2 Header
		req2.Destination = i.Identity()
		req2.Sender = r.Identity()
		req2.Type = PIPE_DATA
		req2.Session = req.Session
		req2.Body = []byte("hello")

		data, err = req2.Marshal()
		require.NoError(t, err)

		err = i.handleMessage(ctx, data)
		require.NoError(t, err)

		i.WaitTilIdle()

		msg, err := rp.Recv(ctx)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(msg))
	})

	n.Only("can open a new pipe", func() {
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

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			wg      sync.WaitGroup
			pipe    *Pipe
			pipeErr error
		)

		accept, err := i.ListenPipe("test")
		require.NoError(t, err)

		wg.Add(1)
		go func() {
			defer wg.Done()

			pipe, pipeErr = r.ConnectPipe(ctx, i.Identity(), "test")
		}()

		l, err := accept.Accept(ctx)
		require.NoError(t, err)

		wg.Wait()

		require.NoError(t, pipeErr)

		err = pipe.Send([]byte("hello"))
		require.NoError(t, err)

		msg, err := l.Recv(ctx)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(msg))

	})

	n.It("detects an unknown pipe endpoint", func() {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.AttachPeer(r.Identity(), ib)
		r.AttachPeer(i.Identity(), rb)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		i.WaitTilIdle()
		r.WaitTilIdle()

		time.Sleep(1 * time.Second)

		_, err = r.ConnectPipe(ctx, i.Identity(), "test")

		assert.Equal(t, ErrUnknownPipe, err)
	})

	n.It("can close a remote pipe by the connector", func() {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.AttachPeer(r.Identity(), ib)
		r.AttachPeer(i.Identity(), rb)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			wg      sync.WaitGroup
			pipe    *Pipe
			pipeErr error
		)

		time.Sleep(100 * time.Millisecond)

		accept, err := i.ListenPipe("test")
		require.NoError(t, err)

		wg.Add(1)
		go func() {
			defer wg.Done()

			pipe, pipeErr = r.ConnectPipe(ctx, i.Identity(), "test")
		}()

		l, err := accept.Accept(ctx)
		require.NoError(t, err)

		wg.Wait()

		require.NoError(t, pipeErr)

		err = pipe.Close()
		require.NoError(t, err)

		_, err = l.Recv(ctx)
		require.Error(t, err)
	})

	n.It("can close a remote pipe by the acceptor", func() {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		defer i.Shutdown()

		r, err := InitNewPeer()
		require.NoError(t, err)

		defer r.Shutdown()

		i.AttachPeer(r.Identity(), ib)
		r.AttachPeer(i.Identity(), rb)

		i.WaitTilIdle()
		r.WaitTilIdle()

		time.Sleep(100 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			wg      sync.WaitGroup
			pipe    *Pipe
			pipeErr error
		)

		accept, err := i.ListenPipe("test")
		require.NoError(t, err)

		wg.Add(1)
		go func() {
			defer wg.Done()

			pipe, pipeErr = r.ConnectPipe(ctx, i.Identity(), "test")
		}()

		l, err := accept.Accept(ctx)
		require.NoError(t, err)

		wg.Wait()

		require.NoError(t, pipeErr)

		err = l.Close()
		require.NoError(t, err)

		_, err = pipe.Recv(ctx)
		require.Error(t, err)
	})

	n.Meow()
}
