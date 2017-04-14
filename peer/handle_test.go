package peer

import (
	"context"
	"sync"
	"testing"
	"time"

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

		r, err := InitNewPeer()
		require.NoError(t, err)

		i.AddNeighbor(r.Identity(), ib)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		listen, err := i.ListenPipe("test")
		require.NoError(t, err)

		var req Header
		req.Destination = i.Identity()
		req.Sender = r.Identity()
		req.Type = PIPE_OPEN
		req.Session = 1
		req.PipeName = "test"

		data, err := req.Marshal()
		require.NoError(t, err)

		err = i.handleMessage(ctx, data)
		require.NoError(t, err)

		rp, err := listen.Accept(ctx)
		require.NoError(t, err)

		assert.Equal(t, int64(1), rp.session)

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
		req2.Session = 1
		req2.Body = []byte("hello")

		data, err = req2.Marshal()
		require.NoError(t, err)

		err = i.handleMessage(ctx, data)
		require.NoError(t, err)

		msg, err := rp.Recv(ctx)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(msg))
	})

	n.It("can open a new pipe", func() {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		r, err := InitNewPeer()
		require.NoError(t, err)

		i.AddNeighbor(r.Identity(), ib)
		r.AddNeighbor(i.Identity(), rb)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			wg      sync.WaitGroup
			pipe    *Pipe
			pipeErr error
		)

		wg.Add(1)
		go func() {
			defer wg.Done()

			pipe, pipeErr = r.ConnectPipe(ctx, i.Identity(), "test")
		}()

		data, err := ib.Recv(nil)
		require.NoError(t, err)

		var req Header

		err = req.Unmarshal(data)
		require.NoError(t, err)

		assert.Equal(t, PIPE_OPEN, req.Type)

		var resp Header
		resp.Destination = req.Sender
		resp.Sender = req.Destination
		resp.Type = PIPE_OPENED
		resp.Session = req.Session

		data, err = resp.Marshal()
		require.NoError(t, err)

		err = r.handleMessage(ctx, data)
		require.NoError(t, err)

		wg.Wait()

		require.NoError(t, pipeErr)

		err = pipe.Send([]byte("hello"))
		require.NoError(t, err)

		data, err = ib.Recv(nil)
		require.NoError(t, err)

		err = req.Unmarshal(data)
		require.NoError(t, err)

		assert.Equal(t, PIPE_DATA, req.Type)
		assert.Equal(t, "hello", string(req.Body))
	})

	n.It("detects an unknown pipe endpoint", func() {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		r, err := InitNewPeer()
		require.NoError(t, err)

		i.AddNeighbor(r.Identity(), ib)
		r.AddNeighbor(i.Identity(), rb)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			wg      sync.WaitGroup
			pipe    *Pipe
			pipeErr error
		)

		wg.Add(1)
		go func() {
			defer wg.Done()

			pipe, pipeErr = r.ConnectPipe(ctx, i.Identity(), "test")
		}()

		data, err := ib.Recv(nil)
		require.NoError(t, err)

		var req Header

		err = req.Unmarshal(data)
		require.NoError(t, err)

		assert.Equal(t, PIPE_OPEN, req.Type)

		var resp Header
		resp.Destination = req.Sender
		resp.Sender = req.Destination
		resp.Type = PIPE_UNKNOWN
		resp.Session = req.Session

		data, err = resp.Marshal()
		require.NoError(t, err)

		err = r.handleMessage(ctx, data)
		require.NoError(t, err)

		wg.Wait()

		assert.Equal(t, ErrUnknownPipe, pipeErr)
	})

	n.It("can close a remote pipe by the connector", func() {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		r, err := InitNewPeer()
		require.NoError(t, err)

		i.AddNeighbor(r.Identity(), ib)
		r.AddNeighbor(i.Identity(), rb)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			wg      sync.WaitGroup
			pipe    *Pipe
			pipeErr error
		)

		wg.Add(1)
		go func() {
			defer wg.Done()

			pipe, pipeErr = r.ConnectPipe(ctx, i.Identity(), "test")
		}()

		data, err := ib.Recv(nil)
		require.NoError(t, err)

		var req Header

		err = req.Unmarshal(data)
		require.NoError(t, err)

		assert.Equal(t, PIPE_OPEN, req.Type)

		var resp Header
		resp.Destination = req.Sender
		resp.Sender = req.Destination
		resp.Type = PIPE_OPENED
		resp.Session = req.Session

		data, err = resp.Marshal()
		require.NoError(t, err)

		err = r.handleMessage(ctx, data)
		require.NoError(t, err)

		wg.Wait()

		require.NoError(t, pipeErr)

		err = pipe.Close()
		require.NoError(t, err)

		data, err = ib.Recv(nil)
		require.NoError(t, err)

		err = req.Unmarshal(data)
		require.NoError(t, err)

		assert.Equal(t, PIPE_CLOSE, req.Type)
		assert.Equal(t, resp.Session, req.Session)
	})

	n.It("can detect a pipe close by the acceptor", func() {
		ib, rb := pairByteTraders()

		i, err := InitNewPeer()
		require.NoError(t, err)

		r, err := InitNewPeer()
		require.NoError(t, err)

		i.AddNeighbor(r.Identity(), ib)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		listen, err := i.ListenPipe("test")
		require.NoError(t, err)

		var req Header
		req.Destination = i.Identity()
		req.Sender = r.Identity()
		req.Type = PIPE_OPEN
		req.Session = 1
		req.PipeName = "test"

		data, err := req.Marshal()
		require.NoError(t, err)

		err = i.handleMessage(ctx, data)
		require.NoError(t, err)

		rp, err := listen.Accept(ctx)
		require.NoError(t, err)

		assert.Equal(t, int64(1), rp.session)

		nxt, err := rb.Recv(nil)
		require.NoError(t, err)

		var resp Header

		err = resp.Unmarshal(nxt)
		require.NoError(t, err)

		assert.Equal(t, PIPE_OPENED, resp.Type)

		err = rp.Close()
		require.NoError(t, err)

		nxt, err = rb.Recv(nil)
		require.NoError(t, err)

		err = resp.Unmarshal(nxt)
		require.NoError(t, err)

		assert.Equal(t, PIPE_CLOSE, resp.Type)
		assert.Equal(t, int64(1), resp.Session)
	})

	n.Meow()
}
