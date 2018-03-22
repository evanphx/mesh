package pipe

import (
	bytes "bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/evanphx/mesh/crypto"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPipe(t *testing.T) {
	n := neko.Modern(t)

	n.It("performs a handshake to connect a pipe", func(t *testing.T) {
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

		defer h1.Cleanup()
		defer h2.Cleanup()

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			if err != nil {
				return
			}

			p1.Send(ctx, []byte("hello"))
		}()

		p2, err := h2.ConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		log.Debugf("receiving")
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

		defer h1.Cleanup()
		defer h2.Cleanup()

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		recvs := make(chan string, 1)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			if err != nil {
				recvs <- ""
				return
			}

			recv, err := p1.Recv(ctx)
			recvs <- string(recv)
		}()

		p2, err := h2.LazyConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		wg.Wait()

		assert.Equal(t, "hello", <-recvs)
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

		defer h1.Cleanup()
		defer h2.Cleanup()

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

		recvs := make(chan string, 1)

		wg.Add(1)
		go func() {
			defer wg.Done()

			var err error
			p1, err = lp.Accept(ctx)
			if err != nil {
				recvs <- ""
				return
			}

			recv, _ := p1.Recv(ctx)
			recvs <- string(recv)
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

		assert.Equal(t, "hello", <-recvs)
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

		defer h1.Cleanup()
		defer h2.Cleanup()

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

		recvs := make(chan string, 2)

		wg.Add(1)
		go func() {
			defer wg.Done()

			var err error

			p1, err = lp.Accept(ctx)
			if err != nil {
				recvs <- ""
				recvs <- ""
				return
			}

			recv, _ := p1.Recv(ctx)
			recvs <- string(recv)

			recv, _ = p1.Recv(ctx)
			recvs <- string(recv)
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

		assert.Equal(t, "hello", <-recvs)
		assert.Equal(t, "world", <-recvs)
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

		defer h1.Cleanup()
		defer h2.Cleanup()

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

		defer h1.Cleanup()
		defer h2.Cleanup()

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()
		s2.Sync = true

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var (
			wg sync.WaitGroup
			p1 *Pipe
		)

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, _ = lp.Accept(ctx)
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

	n.It("processes only used sequences of the window", func(t *testing.T) {
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

		defer h1.Cleanup()
		defer h2.Cleanup()

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

		recvs := make(chan string, 4)

		wg.Add(1)
		go func() {
			defer wg.Done()

			var err error

			p1, err = lp.Accept(ctx)
			if err != nil {
				recvs <- ""
				recvs <- ""
				recvs <- ""
				recvs <- ""
				return
			}

			recv, _ := p1.Recv(ctx)
			recvs <- string(recv)

			recv, _ = p1.Recv(ctx)
			recvs <- string(recv)

			recv, _ = p1.Recv(ctx)
			recvs <- string(recv)

			recv, _ = p1.Recv(ctx)
			recvs <- string(recv)
		}()

		rh.PassThrough = true

		p2, err := h2.ConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		rh.PassThrough = false

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("world"))
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("again"))
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("friends"))
		require.NoError(t, err)

		rh.Swap(0, 3)
		rh.Swap(1, 3)

		rh.DeliverAsStored()

		wg.Wait()

		assert.Equal(t, "hello", <-recvs)
		assert.Equal(t, "world", <-recvs)
		assert.Equal(t, "again", <-recvs)
		assert.Equal(t, "friends", <-recvs)
	})

	n.Meow()
}

func TestPipeAck(t *testing.T) {
	n := neko.Modern(t)

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

		defer h1.Cleanup()
		defer h2.Cleanup()

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		recvs := make(chan string, 1)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			if err != nil {
				recvs <- ""
				return
			}

			recv, _ := p1.Recv(ctx)
			recvs <- string(recv)
		}()

		p2, err := h2.LazyConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		wg.Wait()

		time.Sleep(100 * time.Millisecond)

		sent := s1.At(-1)
		require.NotNil(t, sent)

		msg := sent.Message.(*Message)

		assert.Equal(t, PIPE_DATA_ACK, msg.Type)

		// To allow the ack to fire on h2
		time.Sleep(100 * time.Millisecond)

		assert.True(t, msg.AckId > 0)

		p2.lock.Lock()
		assert.Equal(t, p2.recvThreshold, msg.AckId)
		p2.lock.Unlock()
	})

	n.It("suspends sending until acks are received", func(t *testing.T) {
		var (
			h1 DataHandler
			s1 util.SendAdapter
			h2 DataHandler
			s2 util.SendAdapter

			k1 = crypto.GenerateKey()
			k2 = crypto.GenerateKey()

			qh util.QueueHandler
		)

		h1.Setup(2, &s1, k1)
		h2.Setup(2, &s2, k2)

		defer h1.Cleanup()
		defer h2.Cleanup()

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		qh.Handler = &h2
		s1.Handler = &qh
		s2.Handler = &h1

		qh.SetPassX(1)

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

			time.Sleep(100 * time.Millisecond)

			p1.Recv(ctx)
		}()

		h2.AckBacklog = 1

		p2, err := h2.ConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		p2.lock.Lock()
		assert.True(t, p2.blockForAcks())
		p2.lock.Unlock()

		time.Sleep(time.Second)

		go qh.Deliver(1)

		time.Sleep(100 * time.Millisecond)

		p2.lock.Lock()
		assert.False(t, p2.blockForAcks())
		p2.lock.Unlock()

		wg.Wait()
	})

	n.It("unblocks a send when an ack arrives", func(t *testing.T) {
		var (
			h1 DataHandler
			s1 util.SendAdapter
			h2 DataHandler
			s2 util.SendAdapter

			k1 = crypto.GenerateKey()
			k2 = crypto.GenerateKey()

			qh util.QueueHandler
		)

		h1.Setup(2, &s1, k1)
		h2.Setup(2, &s2, k2)

		defer h1.Cleanup()
		defer h2.Cleanup()

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		qh.Handler = &h2
		s1.Handler = &qh
		s2.Handler = &h1

		qh.SetPassX(1)

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		recvs := make(chan string, 2)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			if err != nil {
				recvs <- ""
				recvs <- ""
				return
			}

			time.Sleep(100 * time.Millisecond)

			log.Debugf("begin recv phase")

			recv, _ := p1.Recv(ctx)
			recvs <- string(recv)

			log.Debugf("waiting on 2")
			recv, _ = p1.Recv(ctx)
			recvs <- string(recv)
		}()

		h2.AckBacklog = 1

		p2, err := h2.ConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		var (
			serr error
			diff time.Duration
		)

		var wg2 sync.WaitGroup

		wg2.Add(1)
		go func() {
			log.Debugf("trying send that requires ack first")
			s := time.Now()
			serr = p2.Send(ctx, []byte("2"))
			diff = time.Since(s)
			log.Debugf("send: %s", serr)
		}()

		// qh.PassThrough = true

		time.Sleep(100 * time.Millisecond)
		log.Debugf("delivering acks")
		go qh.Deliver(2)

		wg.Wait()

		assert.NoError(t, serr)
		assert.True(t, diff >= 100*time.Millisecond)

		assert.Equal(t, "hello", <-recvs)
		assert.Equal(t, "2", <-recvs)
	})

	n.It("retransmits lost messages eventually", func(t *testing.T) {
		var (
			h1 DataHandler
			s1 util.SendAdapter
			h2 DataHandler
			s2 util.SendAdapter

			k1 = crypto.GenerateKey()
			k2 = crypto.GenerateKey()

			qh util.QueueHandler
		)

		h1.Setup(2, &s1, k1)
		h2.Setup(2, &s2, k2)

		defer h1.Cleanup()
		defer h2.Cleanup()

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		qh.Handler = &h2
		s1.Handler = &qh
		s2.Handler = &h1

		qh.SetPassX(1)

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		recvs := make(chan string, 2)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			if err != nil {
				recvs <- ""
				recvs <- ""
				return
			}

			time.Sleep(100 * time.Millisecond)

			log.Debugf("begin recv phase")

			recv, _ := p1.Recv(ctx)
			recvs <- string(recv)

			log.Debugf("waiting on 2")
			recv, _ = p1.Recv(ctx)
			recvs <- string(recv)
		}()

		h2.AckBacklog = 1

		p2, err := h2.LazyConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		before := s2.Size()

		time.Sleep(ResendInterval * 2)

		log.Debugf("sending 2")
		err = p2.Send(ctx, []byte("2"))
		require.NoError(t, err)

		log.Debugf("chceking messages")

		assert.Equal(t, before+1, s2.Size())

		qh.Deliver(3)

		time.Sleep(ResendInterval * 2)

		assert.Equal(t, before+1, s2.Size())
	})

	n.It("can open a subpipe", func(t *testing.T) {
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

		defer h1.Cleanup()
		defer h2.Cleanup()

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			if err != nil {
				return
			}
			log.Debugf("accepted p1")

			time.Sleep(1 * time.Second)

			log.Debugf("open sub")
			sub, err := p1.OpenSub()
			if err != nil {
				return
			}

			sub.Send(ctx, []byte("hello"))
		}()

		p2, err := h2.ConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		log.Debugf("accept sub")
		sub, err := p2.AcceptSub(ctx)
		require.NoError(t, err)

		log.Debugf("receiving")
		recv, err := sub.Recv(ctx)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(recv))

		wg.Wait()
	})

	n.It("can open a subpipe after the parent sends data", func(t *testing.T) {
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

		defer h1.Cleanup()
		defer h2.Cleanup()

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			if err != nil {
				t.Logf("failed to send: %s", err)
				return
			}

			err = p1.Send(ctx, []byte("hello"))
			if err != nil {
				t.Logf("failed to send: %s", err)
				return
			}

			log.Debugf("accepted p1")

			time.Sleep(1 * time.Second)

			log.Debugf("open sub")
			sub, err := p1.OpenSub()
			if err != nil {
				t.Logf("failed to send: %s", err)
				return
			}

			sub.Send(ctx, []byte("hello"))
		}()

		p2, err := h2.ConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		m1, err := p2.Recv(ctx)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(m1))

		log.Debugf("accept sub")
		sub, err := p2.AcceptSub(ctx)
		require.NoError(t, err)

		log.Debugf("receiving")
		recv, err := sub.Recv(ctx)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(recv))

		wg.Wait()
	})

	n.It("can open a subpipe after the parent recvs data", func(t *testing.T) {
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

		defer h1.Cleanup()
		defer h2.Cleanup()

		s1.Sender = k1.Identity()
		s2.Sender = k2.Identity()

		s1.Handler = &h2
		s2.Handler = &h1

		lp, err := h1.ListenPipe("test")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			p1, err := lp.Accept(ctx)
			if err != nil {
				t.Logf("failed to send: %s", err)
				return
			}

			m1, err := p1.Recv(ctx)
			if err != nil {
				t.Logf("failed to send: %s", err)
				return
			}

			if !bytes.Equal(m1, []byte("hello")) {
				t.Log("message not hello")
				return
			}

			log.Debugf("accepted p1")

			time.Sleep(1 * time.Second)

			log.Debugf("open sub")
			sub, err := p1.OpenSub()
			if err != nil {
				t.Logf("failed to send: %s", err)
				return
			}

			sub.Send(ctx, []byte("hello"))
		}()

		p2, err := h2.ConnectPipe(ctx, k1.Identity(), "test")
		require.NoError(t, err)

		err = p2.Send(ctx, []byte("hello"))
		require.NoError(t, err)

		log.Debugf("accept sub")
		sub, err := p2.AcceptSub(ctx)
		require.NoError(t, err)

		log.Debugf("receiving")
		recv, err := sub.Recv(ctx)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(recv))

		wg.Wait()
	})

	n.Meow()
}
