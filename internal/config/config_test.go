package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create temp config file
	tmpDir, err := os.MkdirTemp("", "opnix-tests-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
        "secrets": [
            {
                "path": "test/secret",
                "reference": "op://vault/item/field"
            }
        ]
    }`

	if err := os.WriteFile(configPath, []byte(configData), 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.Secrets) != 1 {
		t.Errorf("Expected 1 secret, got %d", len(cfg.Secrets))
	}

	if cfg.Secrets[0].Path != "test/secret" {
		t.Errorf("Expected path test/secret, got %s", cfg.Secrets[0].Path)
	}
}

func TestLoadMultiple(t *testing.T) {
	// Create temp config files
	tmpDir, err := os.MkdirTemp("", "opnix-tests-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create first config file
	config1Path := filepath.Join(tmpDir, "config1.json")
	config1Data := `{
        "secrets": [
            {
                "path": "database/password",
                "reference": "op://vault/db/password"
            }
        ]
    }`

	if err := os.WriteFile(config1Path, []byte(config1Data), 0600); err != nil {
		t.Fatalf("Failed to write config1 file: %v", err)
	}

	// Create second config file
	config2Path := filepath.Join(tmpDir, "config2.json")
	config2Data := `{
        "secrets": [
            {
                "path": "ssl/cert",
                "reference": "op://vault/ssl/cert"
            },
            {
                "path": "api/key",
                "reference": "op://vault/api/key"
            }
        ]
    }`

	if err := os.WriteFile(config2Path, []byte(config2Data), 0600); err != nil {
		t.Fatalf("Failed to write config2 file: %v", err)
	}

	// Test loading multiple files
	cfg, err := LoadMultiple([]string{config1Path, config2Path})
	if err != nil {
		t.Fatalf("Failed to load multiple configs: %v", err)
	}

	if len(cfg.Secrets) != 3 {
		t.Errorf("Expected 3 secrets, got %d", len(cfg.Secrets))
	}

	// Verify all secrets are present
	paths := make(map[string]bool)
	for _, secret := range cfg.Secrets {
		paths[secret.Path] = true
	}

	expectedPaths := []string{"database/password", "ssl/cert", "api/key"}
	for _, expectedPath := range expectedPaths {
		if !paths[expectedPath] {
			t.Errorf("Expected secret path %s not found", expectedPath)
		}
	}
}

func TestLoadMultiple_InvalidFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "opnix-tests-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	validConfigPath := filepath.Join(tmpDir, "valid.json")
	validConfigData := `{
        "secrets": [
            {
                "path": "test/secret",
                "reference": "op://vault/item/field"
            }
        ]
    }`

	if err := os.WriteFile(validConfigPath, []byte(validConfigData), 0600); err != nil {
		t.Fatalf("Failed to write valid config file: %v", err)
	}

	invalidConfigPath := filepath.Join(tmpDir, "invalid.json")
	invalidConfigData := `{invalid json}`

	if err := os.WriteFile(invalidConfigPath, []byte(invalidConfigData), 0600); err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	_, err = LoadMultiple([]string{validConfigPath, invalidConfigPath})
	if err == nil {
		t.Error("Expected error when loading invalid config file")
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			Secrets: []Secret{
				{Path: "database/password", Reference: "op://vault/db/password"},
				{Path: "ssl/cert", Reference: "op://vault/ssl/cert"},
			},
		}

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validation failed for valid config: %v", err)
		}
	})

	t.Run("duplicate paths", func(t *testing.T) {
		cfg := &Config{
			Secrets: []Secret{
				{Path: "database/password", Reference: "op://vault/db/password"},
				{Path: "database/password", Reference: "op://vault/db/password2"},
			},
		}

		if err := cfg.Validate(); err == nil {
			t.Error("Expected validation error for duplicate paths")
		}
	})

	t.Run("empty reference", func(t *testing.T) {
		cfg := &Config{
			Secrets: []Secret{
				{Path: "database/password", Reference: ""},
			},
		}

		if err := cfg.Validate(); err == nil {
			t.Error("Expected validation error for empty reference")
		}
	})
}
