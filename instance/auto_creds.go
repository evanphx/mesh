package instance

import (
	"os"
	"path/filepath"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/auth"
)

type AutoCreds struct {
	i *Instance
}

func (a *AutoCreds) Validate(net string, peer mesh.Identity, cred []byte) bool {
	return a.i.Peer.Validate(net, peer, cred)
}

func (a *AutoCreds) CredsFor(net string) []byte {
	cred := a.i.Peer.CredsFor(net)
	if cred != nil {
		return cred
	}

	path := filepath.Join(os.Getenv("HOME"), ".config", "mesh", "authorizer", net+".json")

	z, err := auth.ReadFromPath(path)
	if err != nil {
		return nil
	}

	sig, err := a.i.Peer.Authorize(z)
	if err != nil {
		return nil
	}

	return sig
}
