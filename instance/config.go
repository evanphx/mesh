package instance

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DefaultNetwork string   `json:"default_network"`
	Networks       []string `json:"-"`
}

func LoadConfig() *Config {
	var cfg Config

	f, err := os.Open(filepath.Join(os.Getenv("HOME"), ".config", "mesh", "config.json"))
	if err != nil {
		return &cfg
	}

	err = json.NewDecoder(f).Decode(&cfg)
	if err == nil {
		if cfg.DefaultNetwork != "" {
			cfg.Networks = []string{cfg.DefaultNetwork}
		}
	}

	return &cfg
}
