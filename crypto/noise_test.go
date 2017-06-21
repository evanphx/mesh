package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestNoise(t *testing.T) {
	n := neko.Modern(t)

	n.It("can handshake", func(t *testing.T) {
		var (
			ik = GenerateKey()
			rk = GenerateKey()
			i  = NewXXInitiator(ik)
			r  = NewXXResponder(rk)
		)

		m1 := i.Start(nil, nil)

		_, err := r.Start(nil, m1)
		require.NoError(t, err)
		m3 := r.Prime(nil, nil)

		_, err = i.Prime(nil, m3)
		require.NoError(t, err)

		m5, icr, icw := i.Finish(nil, nil)

		_, rcw, rcr, err := r.Finish(nil, m5)
		require.NoError(t, err)

		ct := icw.Encrypt(nil, nil, []byte("hello"))
		pt, err := rcr.Decrypt(nil, nil, ct)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(pt))

		ct2 := rcw.Encrypt(nil, nil, []byte("hello"))
		pt2, err := icr.Decrypt(nil, nil, ct2)
		require.NoError(t, err)

		assert.Equal(t, "hello", string(pt2))
	})

	n.Meow()
}
