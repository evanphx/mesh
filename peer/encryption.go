package peer

import (
	"crypto/rand"

	"github.com/flynn/noise"
)

var (
	CipherSuite = noise.NewCipherSuite(
		noise.DH25519,
		noise.CipherAESGCM,
		noise.HashBLAKE2b)
	RNG      = rand.Reader
	Prologue = []byte("mesh")
)
