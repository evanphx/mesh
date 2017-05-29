package util

import (
	"encoding/binary"
	"math/rand"

	"github.com/evanphx/mesh"
)

func RandomIdentity() mesh.Identity {
	id := make(mesh.Identity, 8)
	binary.BigEndian.PutUint64(id, rand.Uint64())
	return id
}
