package reliable

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestReliable(t *testing.T) {
	n := neko.Modern(t)

	cfg := Config{
		WindowSlots: 10,
		WindowBytes: OneMB,
	}

	n.It("takes a byte slice and returns a seq number for it", func(t *testing.T) {
		r := NewReliable(cfg)

		n, ok := r.Queue([]byte("foo"))
		require.True(t, ok)

		assert.Equal(t, "foo", string(r.slots[r.slotNumber(n)]))
	})

	n.It("indicates that it can not queue more data", func(t *testing.T) {
		r := NewReliable(cfg)

		for i := 0; i < QueueSlots; i++ {
			_, ok := r.Queue([]byte("foo"))
			require.True(t, ok)
		}

		_, ok := r.Queue([]byte("foo"))
		assert.False(t, ok)
	})

	n.It("clears slots when ack'd", func(t *testing.T) {
		r := NewReliable(cfg)

		n, ok := r.Queue([]byte("foo"))
		assert.True(t, ok)

		assert.NotNil(t, r.slots[0])

		err := r.Ack(n)
		require.NoError(t, err)

		assert.Nil(t, r.slots[0])

		assert.Equal(t, SeqNum(1), r.threshold)

		x, ok := r.Queue([]byte("bar"))
		require.True(t, ok)

		assert.True(t, x.After(n))

		assert.NotNil(t, r.slots[0])
	})

	n.It("treats acks cummulatively", func(t *testing.T) {
		r := NewReliable(cfg)

		_, ok := r.Queue([]byte("foo"))
		assert.True(t, ok)

		assert.NotNil(t, r.slots[0])

		m, ok := r.Queue([]byte("foo"))
		assert.True(t, ok)

		assert.NotNil(t, r.slots[1])

		err := r.Ack(m)
		require.NoError(t, err)

		assert.Nil(t, r.slots[0])
		assert.Nil(t, r.slots[1])

		assert.Equal(t, SeqNum(2), r.threshold)
	})

	n.It("ignores acks for message already seen", func(t *testing.T) {
		r := NewReliable(cfg)

		n, ok := r.Queue([]byte("foo"))
		require.True(t, ok)

		m, ok := r.Queue([]byte("foo"))
		require.True(t, ok)

		err := r.Ack(m)
		require.NoError(t, err)

		err = r.Ack(n)
		require.NoError(t, err)

		assert.Equal(t, m, r.threshold)
	})

	n.It("tracks the bytes outstanding in a window to track queueing", func(t *testing.T) {
		cfg := Config{
			WindowSlots: 100,
			WindowBytes: 10,
		}

		r := NewReliable(cfg)

		n, ok := r.Queue([]byte("foo"))
		assert.True(t, ok)

		assert.NotNil(t, r.slots[0])
		assert.Equal(t, 7, r.curWindow)

		large := []byte("12345678")

		_, ok = r.Queue(large)
		assert.False(t, ok)

		r.Ack(n)

		_, ok = r.Queue(large)
		require.True(t, ok)

		assert.Equal(t, 2, r.curWindow)
	})

	n.It("can retrieve the data for a seq number", func(t *testing.T) {
		r := NewReliable(cfg)

		data := []byte("foo")

		n, ok := r.Queue(data)
		require.True(t, ok)

		d2, ok := r.Retrieve(n)
		require.True(t, ok)

		assert.Equal(t, data, d2)
	})

	n.It("will not retrieve data for ack'd seq numbers", func(t *testing.T) {
		r := NewReliable(cfg)

		data := []byte("foo")

		n, ok := r.Queue(data)
		require.True(t, ok)

		err := r.Ack(n)
		require.NoError(t, err)

		_, ok = r.Retrieve(n)
		require.False(t, ok)
	})

	n.It("returns the range out outstanding ack's", func(t *testing.T) {
		r := NewReliable(cfg)

		_, _, ok := r.Outstanding()
		require.False(t, ok)

		n, ok := r.Queue([]byte("foo"))

		s, f, ok := r.Outstanding()
		require.True(t, ok)

		assert.Equal(t, n, s)
		assert.Equal(t, n, f)

		m, ok := r.Queue([]byte("bar"))
		require.True(t, ok)

		s, f, ok = r.Outstanding()
		require.True(t, ok)

		assert.Equal(t, n, s)
		assert.Equal(t, m, f)

		err := r.Ack(n)
		require.NoError(t, err)

		s, f, ok = r.Outstanding()
		require.True(t, ok)

		assert.Equal(t, m, s)
		assert.Equal(t, m, f)

		err = r.Ack(m)
		require.NoError(t, err)

		_, _, ok = r.Outstanding()
		require.False(t, ok)
	})

	n.Meow()
}
