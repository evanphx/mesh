package peer

import (
	"bytes"
	"encoding/hex"

	"github.com/flynn/noise"
)

type Identity []byte

var NilIdentity Identity

func (i Identity) String() string {
	return hex.EncodeToString(i)
}

func (i Identity) GoString() string {
	return hex.EncodeToString(i)
}

func (i Identity) Short() string {
	return i.String()[:7]
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

func ToIdentity(str string) Identity {
	data, err := hex.DecodeString(str)
	if err != nil {
		panic(err)
	}

	return Identity(data)
}
