package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektra/neko"
)

type testEndpoint struct {
	m *Message
}

func (t *testEndpoint) Inject(m *Message) error {
	t.m = m
	return nil
}

func TestEngine(t *testing.T) {
	n := neko.Start(t)

	n.It("tracks neighbors", func() {
		eng := NewEngine()

		var ep testEndpoint

		eng.AddNeighbor("xxyyzz", &ep)

		assert.Equal(t, &ep, eng.neighbors["xxyyzz"])
	})

	n.It("uses the router to ", func() {
		eng := NewEngine()

		var ep testEndpoint

		eng.AddNeighbor("xxyyzz", &ep)

		assert.Equal(t, &ep, eng.neighbors["xxyyzz"])
	})

	n.Meow()
}
