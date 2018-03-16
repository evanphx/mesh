package main

import (
	"os"
	"path/filepath"

	"github.com/evanphx/mesh/auth"
)

type AuthGen struct {
	Network string `short:"n" long:"network" required:"true" description:"name of network to generate authorization for"`
}

func (a *AuthGen) Execute(args []string) error {
	z, err := auth.NewAuthorizer(a.Network)
	if err != nil {
		return err
	}

	dir := filepath.Join(os.Getenv("HOME"), ".config", "mesh", "authorizer")
	os.MkdirAll(dir, 0755)

	return z.SaveToPath(filepath.Join(dir, a.Network+".json"))
}
