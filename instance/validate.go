package instance

import "github.com/evanphx/mesh"

func (i *Instance) Validate(name string, peer mesh.Identity, b []byte) bool {
	return true
}
