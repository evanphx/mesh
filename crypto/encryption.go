package crypto

import (
	"crypto/rand"

	"github.com/evanphx/mesh"
	"github.com/flynn/noise"
)

var (
	CipherSuite = noise.NewCipherSuite(
		noise.DH25519,
		noise.CipherChaChaPoly,
		noise.HashBLAKE2b)
	RNG      = rand.Reader
	Prologue = []byte("mesh")
)

type DHKey noise.DHKey

func GenerateKey() DHKey {
	return DHKey(CipherSuite.GenerateKeypair(RNG))
}

func (k *DHKey) Identity() mesh.Identity {
	return mesh.Identity(k.Public)
}
