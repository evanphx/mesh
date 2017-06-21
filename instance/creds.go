package instance

import "github.com/evanphx/mesh"

type NoCreds struct{}

func (c NoCreds) CredsFor(name string) []byte {
	return nil
}

func (c NoCreds) Validate(name string, peer mesh.Identity, creds []byte) bool {
	return true
}
