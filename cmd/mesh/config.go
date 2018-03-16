package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/evanphx/mesh/instance"
)

type ConfigSet struct {
}

func (c *ConfigSet) Execute(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("requires name and value")
	}

	cfg := instance.LoadConfig()

	switch args[0] {
	case "default-network":
		cfg.DefaultNetwork = args[1]
	default:
		return fmt.Errorf("unknown key: %s", args[0])
	}

	f, err := os.Create(filepath.Join(os.Getenv("HOME"), ".config", "mesh", "config.json"))
	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewEncoder(f).Encode(cfg)
}
