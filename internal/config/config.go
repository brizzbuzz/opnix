package config

import (
	"encoding/json"
	"os"

	"github.com/brizzbuzz/opnix/internal/errors"
	"github.com/brizzbuzz/opnix/internal/validation"
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

// convertToValidationSecrets converts config secrets to validation format
func (c *Config) convertToValidationSecrets() []validation.SecretData {
	secrets := make([]validation.SecretData, len(c.Secrets))
	for i, s := range c.Secrets {
		secrets[i] = validation.SecretData{
			Path:      s.Path,
			Reference: s.Reference,
			Owner:     s.Owner,
			Group:     s.Group,
			Mode:      s.Mode,
		}
	}
	return secrets
}

// Load loads a single config file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.FileOperationError(
			"Loading configuration file",
			path,
			"Failed to read config file",
			err,
		)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, errors.ConfigError(
			"Parsing configuration file",
			"Invalid JSON format in config file",
			err,
		)
	}

	// Validate the loaded configuration
	validator := validation.NewValidator()
	if err := validator.ValidateConfigStruct(config.convertToValidationSecrets()); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadMultiple loads and merges multiple config files (GitHub #3)
func LoadMultiple(paths []string) (*Config, error) {
	if len(paths) == 0 {
		return nil, errors.ConfigError(
			"Loading multiple config files",
			"No config file paths provided",
			nil,
		)
	}

	var allSecrets []Secret

	for _, path := range paths {
		config, err := Load(path)
		if err != nil {
			return nil, errors.WrapWithSuggestions(
				err,
				"Loading multiple config files",
				"configuration",
				[]string{
					"Check that all config file paths are correct",
					"Ensure all config files have valid JSON format",
					"Verify file permissions allow reading",
				},
			)
		}
		allSecrets = append(allSecrets, config.Secrets...)
	}

	mergedConfig := &Config{Secrets: allSecrets}

	// Validate the merged configuration for cross-file conflicts
	validator := validation.NewValidator()
	if err := validator.ValidateConfigStruct(mergedConfig.convertToValidationSecrets()); err != nil {
		return nil, err
	}

	return mergedConfig, nil
}

// Validate checks for duplicate secret paths across all configs
// Deprecated: Use validation.Validator.ValidateConfigStruct() for comprehensive validation
func (c *Config) Validate() error {
	validator := validation.NewValidator()
	return validator.ValidateConfigStruct(c.convertToValidationSecrets())
}
