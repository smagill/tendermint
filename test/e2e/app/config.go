package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Listen string
	GRPC   bool `toml:"grpc"`
}

func LoadConfig(file string) (Config, error) {
	cfg := Config{
		Listen: "unix:///var/run/app.sock",
		GRPC:   false,
	}
	r, err := os.Open(file)
	if err != nil {
		return cfg, fmt.Errorf("failed to open app config %q: %w", file, err)
	}
	_, err = toml.DecodeReader(r, &cfg)
	return cfg, err
}
