package config

import (
	"encoding/json"
	"errors"
	"os"
)

// Config holds runtime configuration for the notifier.
type Config struct {
	Blacklist []string `json:"blacklist"`
}

// Load reads and JSON-unmarshals a Config from path.
// If the file is missing, it returns Config{}, nil (= no filtering).
// Other I/O or unmarshal errors propagate.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
