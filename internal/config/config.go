package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Secret struct {
	Path      string `json:"path"`
	Reference string `json:"reference"`
	Owner     string `json:"owner,omitempty"`
	Group     string `json:"group,omitempty"`
	Mode      string `json:"mode,omitempty"`
}

type Config struct {
	Secrets []Secret `json:"secrets"`
}

// Load loads a single config file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadMultiple loads and merges multiple config files (GitHub #3)
func LoadMultiple(paths []string) (*Config, error) {
	var allSecrets []Secret

	for _, path := range paths {
		config, err := Load(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", path, err)
		}
		allSecrets = append(allSecrets, config.Secrets...)
	}

	return &Config{Secrets: allSecrets}, nil
}

// Validate checks for duplicate secret paths across all configs
func (c *Config) Validate() error {
	seen := make(map[string]bool)

	for _, secret := range c.Secrets {
		if seen[secret.Path] {
			return fmt.Errorf("duplicate secret path: %s", secret.Path)
		}
		seen[secret.Path] = true

		if secret.Reference == "" {
			return fmt.Errorf("secret %s has empty reference", secret.Path)
		}
	}

	return nil
}
