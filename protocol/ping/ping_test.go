package ping

import (
	"context"
	"testing"
	"time"

	"github.com/evanphx/mesh/pb"
	"github.com/evanphx/mesh/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestPing(t *testing.T) {
	n := neko.Modern(t)

	n.It("send ping messages", func(t *testing.T) {
		var (
			h  Handler
			ss util.SavingSender
		)

		h.Setup(1, &ss)

		dest := util.RandomIdentity()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			h.Ping(ctx, dest)
		}()

		time.Sleep(100 * time.Millisecond)

		require.Equal(t, 1, len(ss.Messages))

		msg := ss.Messages[0]

		assert.Equal(t, dest, msg.Dest)
		assert.Equal(t, int32(1), msg.Proto)

		pm, ok := msg.Message.(*Message)
		require.True(t, ok)

		assert.Equal(t, h.nextSess-1, pm.Session)
	})

	n.It("send pong messages", func(t *testing.T) {
		var (
			h  Handler
			ss util.SavingSender
		)

		h.Setup(1, &ss)

		src := util.RandomIdentity()
		dest := util.RandomIdentity()

		var msg Message
		msg.Session = 47

		data, err := msg.Marshal()
		require.NoError(t, err)

		var hdr pb.Header

		hdr.Destination = dest
		hdr.Sender = src
		hdr.Proto = 1
		hdr.Body = data

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = h.Handle(ctx, &hdr)
		require.NoError(t, err)

		require.Equal(t, 1, len(ss.Messages))

		sent := ss.Messages[0]

		pm, ok := sent.Message.(*Message)
		require.True(t, ok)

		assert.True(t, pm.Reply)
	})

	n.It("handles pong responses", func(t *testing.T) {
		var (
			h1 Handler
			s1 util.SendAdapter
			h2 Handler
			s2 util.SendAdapter
		)

		h1.Setup(1, &s1)
		h2.Setup(1, &s2)

		src := util.RandomIdentity()
		dest := util.RandomIdentity()

		s1.Sender = src
		s1.Handler = &h2

		s2.Sender = dest
		s2.Handler = &h1

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dur, err := h1.Ping(ctx, dest)

		require.NoError(t, err)
		assert.True(t, dur > 0)
	})

	n.Meow()
}
