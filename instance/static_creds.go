package instance

import (
	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/log"
)

type StaticCreds struct {
	Network string `json:"network"`
	Token   string `json:"token"`
}

func (s *StaticCreds) CredsFor(network string) []byte {
	if network == s.Network {
		return []byte(s.Token)
	}

	return nil
}

func (s *StaticCreds) Validate(network string, peer mesh.Identity, token []byte) bool {
	log.Debugf("checking creds %s/%s against %v", network, string(token), s)
	if network != s.Network {
		return false
	}

	return s.Token == string(token)
}
