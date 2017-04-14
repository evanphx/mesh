package peer

import (
	"bytes"
	"encoding/hex"

	"github.com/flynn/noise"
)

type Identity []byte

func (i Identity) String() string {
	return hex.EncodeToString(i)
}

func (i Identity) Equal(j Identity) bool {
	return bytes.Equal(i, j)
}

func (i Identity) ForKey(key noise.DHKey) bool {
	if bytes.Equal(i, key.Public) {
		return true
	}

	return false
}
