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

func TestSecretOwnership(t *testing.T) {
	t.Run("secret with ownership", func(t *testing.T) {
		secret := Secret{
			Path:      "ssl/cert",
			Reference: "op://vault/ssl/cert",
			Owner:     "root",
			Group:     "root",
			Mode:      "0644",
		}

		if secret.Owner != "root" {
			t.Errorf("Expected owner root, got %s", secret.Owner)
		}
		if secret.Group != "root" {
			t.Errorf("Expected group root, got %s", secret.Group)
		}
		if secret.Mode != "0644" {
			t.Errorf("Expected mode 0644, got %s", secret.Mode)
		}
	})

	t.Run("secret with defaults", func(t *testing.T) {
		secret := Secret{
			Path:      "database/password",
			Reference: "op://vault/db/password",
			// Owner, Group, Mode not specified - should work fine
		}

		if secret.Owner != "" {
			t.Errorf("Expected empty owner, got %s", secret.Owner)
		}
		if secret.Group != "" {
			t.Errorf("Expected empty group, got %s", secret.Group)
		}
		if secret.Mode != "" {
			t.Errorf("Expected empty mode, got %s", secret.Mode)
		}
	})
}

func TestLoadWithOwnership(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "opnix-tests-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"secrets": [
			{
				"path": "ssl/cert",
				"reference": "op://vault/ssl/cert",
				"owner": "root",
				"group": "root",
				"mode": "0644"
			},
			{
				"path": "database/password",
				"reference": "op://vault/db/password"
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

	if len(cfg.Secrets) != 2 {
		t.Errorf("Expected 2 secrets, got %d", len(cfg.Secrets))
	}

	// Check first secret with ownership
	sslSecret := cfg.Secrets[0]
	if sslSecret.Owner != "root" {
		t.Errorf("Expected owner root, got %s", sslSecret.Owner)
	}
	if sslSecret.Group != "root" {
		t.Errorf("Expected group root, got %s", sslSecret.Group)
	}
	if sslSecret.Mode != "0644" {
		t.Errorf("Expected mode 0644, got %s", sslSecret.Mode)
	}

	// Check second secret without ownership (should be empty)
	dbSecret := cfg.Secrets[1]
	if dbSecret.Owner != "" {
		t.Errorf("Expected empty owner, got %s", dbSecret.Owner)
	}
	if dbSecret.Group != "" {
		t.Errorf("Expected empty group, got %s", dbSecret.Group)
	}
	if dbSecret.Mode != "" {
		t.Errorf("Expected empty mode, got %s", dbSecret.Mode)
	}
}

func TestLoadWithPathTemplateAndSymlinks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "opnix-tests-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"pathTemplate": "/etc/secrets/{service}/{name}",
		"defaults": {
			"environment": "production"
		},
		"secrets": [
			{
				"path": "ssl/cert",
				"reference": "op://vault/ssl/cert",
				"symlinks": ["/etc/ssl/certs/legacy.pem", "/opt/service/ssl/cert.pem"]
			},
			{
				"path": "/etc/database/password",
				"reference": "op://vault/db/password",
				"variables": {
					"service": "postgresql"
				}
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

	// Check path template
	if cfg.PathTemplate != "/etc/secrets/{service}/{name}" {
		t.Errorf("Expected pathTemplate '/etc/secrets/{service}/{name}', got %s", cfg.PathTemplate)
	}

	// Check defaults
	if len(cfg.Defaults) != 1 {
		t.Errorf("Expected 1 default, got %d", len(cfg.Defaults))
	}
	if cfg.Defaults["environment"] != "production" {
		t.Errorf("Expected default environment 'production', got %s", cfg.Defaults["environment"])
	}

	if len(cfg.Secrets) != 2 {
		t.Errorf("Expected 2 secrets, got %d", len(cfg.Secrets))
	}

	// Check first secret with symlinks
	sslSecret := cfg.Secrets[0]
	if len(sslSecret.Symlinks) != 2 {
		t.Errorf("Expected 2 symlinks, got %d", len(sslSecret.Symlinks))
	}
	expectedSymlinks := []string{"/etc/ssl/certs/legacy.pem", "/opt/service/ssl/cert.pem"}
	for i, expected := range expectedSymlinks {
		if sslSecret.Symlinks[i] != expected {
			t.Errorf("Expected symlink[%d] %s, got %s", i, expected, sslSecret.Symlinks[i])
		}
	}

	// Check second secret with variables
	dbSecret := cfg.Secrets[1]
	if len(dbSecret.Variables) != 1 {
		t.Errorf("Expected 1 variable, got %d", len(dbSecret.Variables))
	}
	if dbSecret.Variables["service"] != "postgresql" {
		t.Errorf("Expected variable service 'postgresql', got %s", dbSecret.Variables["service"])
	}
}

func TestLoadMultipleWithTemplates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "opnix-tests-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create first config file with template
	config1Path := filepath.Join(tmpDir, "config1.json")
	config1Data := `{
		"pathTemplate": "/etc/secrets/{service}/{name}",
		"defaults": {
			"environment": "dev"
		},
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

	// Create second config file with different template (should override)
	config2Path := filepath.Join(tmpDir, "config2.json")
	config2Data := `{
		"pathTemplate": "/run/secrets/{service}/{name}",
		"defaults": {
			"environment": "production",
			"service": "default"
		},
		"secrets": [
			{
				"path": "ssl/cert",
				"reference": "op://vault/ssl/cert"
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

	// The last config file's template and defaults should win
	if cfg.PathTemplate != "/run/secrets/{service}/{name}" {
		t.Errorf("Expected pathTemplate from last config '/run/secrets/{service}/{name}', got %s", cfg.PathTemplate)
	}

	if len(cfg.Defaults) != 2 {
		t.Errorf("Expected 2 defaults, got %d", len(cfg.Defaults))
	}
	if cfg.Defaults["environment"] != "production" {
		t.Errorf("Expected default environment 'production', got %s", cfg.Defaults["environment"])
	}
	if cfg.Defaults["service"] != "default" {
		t.Errorf("Expected default service 'default', got %s", cfg.Defaults["service"])
	}

	if len(cfg.Secrets) != 2 {
		t.Errorf("Expected 2 secrets, got %d", len(cfg.Secrets))
	}
}
