package pipe

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/evanphx/mesh/crypto"
	"github.com/evanphx/mesh/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPipe(t *testing.T) {
	n := neko.Modern(t)

	n.It("perfoms a handshake to connect a pipe", func(t *testing.T) {
		var (
			h1 DataHandler
			s1 util.SendAdapter
			h2 DataHandler
			s2 util.SendAdapter

			k1 = crypto.GenerateKey()
			k2 = crypto.GenerateKey()
		)

		h1.Setup(2, &s1, k1)
		h2.Setup(2, &s2, k2)

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			require.NoError(t, err)

			err = p1.Send(ctx, []byte("hello"))
			require.NoError(t, err)
		}()

		p2, err := h2.ConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		recv, err := p2.Recv(ctx)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(recv))

		wg.Wait()
	})

	n.It("can handshake on first use", func(t *testing.T) {
		var (
			h1 DataHandler
			s1 util.SendAdapter
			h2 DataHandler
			s2 util.SendAdapter

			k1 = crypto.GenerateKey()
			k2 = crypto.GenerateKey()
		)

		h1.Setup(2, &s1, k1)
		h2.Setup(2, &s2, k2)

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			require.NoError(t, err)

			recv, err := p1.Recv(ctx)
			require.NoError(t, err)
			assert.Equal(t, "hello", string(recv))
		}()

		p2, err := h2.LazyConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		wg.Wait()
	})

	n.It("acks payloads to maintain a window", func(t *testing.T) {
		var (
			h1 DataHandler
			s1 util.SendAdapter
			h2 DataHandler
			s2 util.SendAdapter

			k1 = crypto.GenerateKey()
			k2 = crypto.GenerateKey()
		)

		h1.Setup(2, &s1, k1)
		h2.Setup(2, &s2, k2)

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			require.NoError(t, err)

			recv, err := p1.Recv(ctx)
			require.NoError(t, err)
			assert.Equal(t, "hello", string(recv))
		}()

		p2, err := h2.LazyConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		wg.Wait()

		require.True(t, len(s1.Messages) > 0)

		msg := s1.Messages[len(s1.Messages)-1].Message.(*Message)

		assert.Equal(t, PIPE_DATA_ACK, msg.Type)

		// To allow the ack to fire on h2
		time.Sleep(100 * time.Millisecond)

		assert.True(t, msg.SeqId > 0)

		assert.Equal(t, p2.recvThreshold, msg.SeqId)
	})

	n.It("ignores duplicate messages receieved", func(t *testing.T) {
		var (
			h1 DataHandler
			s1 util.SendAdapter
			h2 DataHandler
			s2 util.SendAdapter

			k1 = crypto.GenerateKey()
			k2 = crypto.GenerateKey()
		)

		h1.Setup(2, &s1, k1)
		h2.Setup(2, &s2, k2)

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			wg sync.WaitGroup
			p1 *Pipe
		)

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err = lp.Accept(ctx)
			require.NoError(t, err)

			recv, err := p1.Recv(ctx)
			require.NoError(t, err)
			assert.Equal(t, "hello", string(recv))
		}()

		p2, err := h2.ConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		wg.Wait()

		max := len(s1.Messages)

		// Replay the last message p2 sent to see that p1 ignores it

		h1.Handle(ctx, s2.Messages[len(s2.Messages)-1].ToHeader(s2.Sender))

		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, max, len(s1.Messages))
		assert.Equal(t, 0, len(p1.message))
	})

	n.It("reorders messages before delivering them", func(t *testing.T) {
		var (
			h1 DataHandler
			s1 util.SendAdapter
			h2 DataHandler
			s2 util.SendAdapter

			k1 = crypto.GenerateKey()
			k2 = crypto.GenerateKey()

			rh util.ReverseHandler
		)

		h1.Setup(2, &s1, k1)
		h2.Setup(2, &s2, k2)

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()
		s2.Sync = true

		s1.Handler = &h2
		rh.Handler = &h1
		s2.Handler = &rh

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			wg sync.WaitGroup
			p1 *Pipe
		)

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err = lp.Accept(ctx)
			require.NoError(t, err)

			recv, err := p1.Recv(ctx)
			require.NoError(t, err)
			assert.Equal(t, "hello", string(recv))

			recv, err = p1.Recv(ctx)
			require.NoError(t, err)
			assert.Equal(t, "world", string(recv))
		}()

		rh.PassThrough = true

		p2, err := h2.ConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		rh.PassThrough = false

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		err = p2.Send(ctx, []byte("world"))
		require.NoError(t, err)

		rh.Deliver()

		wg.Wait()
	})

	n.It("can deal with an out-of-order 0-rrt open", func(t *testing.T) {
		var (
			h1 DataHandler
			s1 util.SendAdapter
			h2 DataHandler
			s2 util.SendAdapter

			k1 = crypto.GenerateKey()
			k2 = crypto.GenerateKey()

			rh util.ReverseHandler
		)

		h1.Setup(2, &s1, k1)
		h2.Setup(2, &s2, k2)

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()
		s2.Sync = true

		s1.Handler = &h2
		rh.Handler = &h1
		s2.Handler = &rh

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			wg sync.WaitGroup
			p1 *Pipe
		)

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err = lp.Accept(ctx)
			require.NoError(t, err)

			recv, err := p1.Recv(ctx)
			require.NoError(t, err)
			assert.Equal(t, "hello", string(recv))

			recv, err = p1.Recv(ctx)
			require.NoError(t, err)
			assert.Equal(t, "world", string(recv))
		}()

		p2, err := h2.LazyConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		err = p2.Send(ctx, []byte("world"))
		require.NoError(t, err)

		rh.Deliver()

		wg.Wait()
	})

	n.It("sends an error back on an unknown session", func(t *testing.T) {
		var (
			h1 DataHandler
			s1 util.SendAdapter
			h2 DataHandler
			s2 util.SendAdapter

			k1 = crypto.GenerateKey()
			k2 = crypto.GenerateKey()
		)

		h1.Setup(2, &s1, k1)
		h2.Setup(2, &s2, k2)

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()
		s2.Sync = true

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			wg sync.WaitGroup
			p1 *Pipe
		)

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err = lp.Accept(ctx)
			require.NoError(t, err)
		}()

		p2, err := h2.LazyConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// "forget" about any open pipes
		h1.pipes = make(map[pipeKey]*Pipe)

		time.Sleep(2 * time.Second)

		err = p2.Send(ctx, []byte("world"))
		require.NoError(t, err)

		time.Sleep(2 * time.Second)

		err = p2.Send(ctx, []byte("again"))
		require.Error(t, err)
		wg.Wait()
	})

	n.Meow()
}
