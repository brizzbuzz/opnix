package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/brizzbuzz/opnix/internal/config"
)

// Mock client for testing
type mockClient struct {
	secrets map[string]string
}

func (m *mockClient) ResolveSecret(reference string) (string, error) {
	if value, ok := m.secrets[reference]; ok {
		return value, nil
	}
	return "", fmt.Errorf("secret not found")
}

func TestProcessor(t *testing.T) {
	// Create mock client
	mock := &mockClient{
		secrets: map[string]string{
			"op://vault/item/field": "test-secret-value",
		},
	}

	// Create temp output directory
	tmpDir, err := os.MkdirTemp("", "opnix-processor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create processor
	processor := NewProcessor(mock, tmpDir)

	// Create test config
	cfg := &config.Config{
		Secrets: []config.Secret{
			{
				Path:      "test/secret",
				Reference: "op://vault/item/field",
			},
		},
	}

	// Process secrets
	if err := processor.Process(cfg); err != nil {
		t.Fatalf("Failed to process secrets: %v", err)
	}

	// Verify output
	outputPath := filepath.Join(tmpDir, "test/secret")
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "test-secret-value" {
		t.Errorf("Expected secret value test-secret-value, got %s", string(content))
	}
}

func TestProcessorWithOwnership(t *testing.T) {
	// Skip ownership tests on Windows
	if runtime.GOOS == "windows" {
		t.Skip("Ownership tests not supported on Windows")
	}

	// Create mock client
	mock := &mockClient{
		secrets: map[string]string{
			"op://vault/ssl/cert":    "test-certificate",
			"op://vault/db/password": "secret-password",
		},
	}

	// Create temp output directory
	tmpDir, err := os.MkdirTemp("", "opnix-processor-ownership-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create processor
	processor := NewProcessor(mock, tmpDir)

	// Test without ownership first (should always work)
	cfg := &config.Config{
		Secrets: []config.Secret{
			{
				Path:      "ssl/cert",
				Reference: "op://vault/ssl/cert",
				Mode:      "0644",
				// No ownership specified
			},
			{
				Path:      "database/password",
				Reference: "op://vault/db/password",
				// No ownership specified - should use defaults
			},
		},
	}

	// Process secrets
	if err := processor.Process(cfg); err != nil {
		t.Fatalf("Failed to process secrets: %v", err)
	}

	// Verify SSL cert file
	sslPath := filepath.Join(tmpDir, "ssl/cert")
	content, err := os.ReadFile(sslPath)
	if err != nil {
		t.Fatalf("Failed to read SSL cert file: %v", err)
	}
	if string(content) != "test-certificate" {
		t.Errorf("Expected certificate content, got %s", string(content))
	}

	// Check file permissions for SSL cert
	info, err := os.Stat(sslPath)
	if err != nil {
		t.Fatalf("Failed to stat SSL cert file: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("Expected permissions 0644, got %o", info.Mode().Perm())
	}

	// Verify database password file
	dbPath := filepath.Join(tmpDir, "database/password")
	content, err = os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("Failed to read database password file: %v", err)
	}
	if string(content) != "secret-password" {
		t.Errorf("Expected password content, got %s", string(content))
	}

	// Check default permissions for database password
	info, err = os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Failed to stat database password file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected default permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestProcessorModeValidation(t *testing.T) {
	// Create mock client
	mock := &mockClient{
		secrets: map[string]string{
			"op://vault/item/field": "test-value",
		},
	}

	// Create temp output directory
	tmpDir, err := os.MkdirTemp("", "opnix-processor-mode-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create processor
	processor := NewProcessor(mock, tmpDir)

	t.Run("valid mode", func(t *testing.T) {
		cfg := &config.Config{
			Secrets: []config.Secret{
				{
					Path:      "test/valid-mode",
					Reference: "op://vault/item/field",
					Mode:      "0755",
				},
			},
		}

		if err := processor.Process(cfg); err != nil {
			t.Errorf("Valid mode should not fail: %v", err)
		}

		// Check the actual file permissions
		filePath := filepath.Join(tmpDir, "test/valid-mode")
		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("Expected permissions 0755, got %o", info.Mode().Perm())
		}
	})

	t.Run("invalid mode", func(t *testing.T) {
		cfg := &config.Config{
			Secrets: []config.Secret{
				{
					Path:      "test/invalid-mode",
					Reference: "op://vault/item/field",
					Mode:      "invalid-mode",
				},
			},
		}

		err := processor.Process(cfg)
		if err == nil {
			t.Error("Expected error with invalid mode, got nil")
		}
		if err != nil && !contains(err.Error(), "Invalid value") && !contains(err.Error(), "mode") {
			t.Errorf("Expected mode validation error, got: %v", err)
		}
	})
}

func TestProcessorOwnershipValidation(t *testing.T) {
	// Skip on Windows
	if runtime.GOOS == "windows" {
		t.Skip("User tests not supported on Windows")
	}

	// Create mock client
	mock := &mockClient{
		secrets: map[string]string{
			"op://vault/item/field": "test-value",
		},
	}

	// Create temp output directory
	tmpDir, err := os.MkdirTemp("", "opnix-processor-ownership-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create processor
	processor := NewProcessor(mock, tmpDir)

	t.Run("invalid user", func(t *testing.T) {
		cfg := &config.Config{
			Secrets: []config.Secret{
				{
					Path:      "test/secret",
					Reference: "op://vault/item/field",
					Owner:     "nonexistent-user-12345",
				},
			},
		}

		err := processor.Process(cfg)
		if err == nil {
			t.Error("Expected error with invalid user, got nil")
		}
		if err != nil && !contains(err.Error(), "does not exist") {
			t.Errorf("Expected 'does not exist' error, got: %v", err)
		}
	})

	t.Run("invalid group", func(t *testing.T) {
		cfg := &config.Config{
			Secrets: []config.Secret{
				{
					Path:      "test/secret",
					Reference: "op://vault/item/field",
					Group:     "nonexistent-group-12345",
				},
			},
		}

		err := processor.Process(cfg)
		if err == nil {
			t.Error("Expected error with invalid group, got nil")
		}
		if err != nil && !contains(err.Error(), "does not exist") {
			t.Errorf("Expected 'does not exist' error, got: %v", err)
		}
	})

	t.Run("no ownership specified", func(t *testing.T) {
		cfg := &config.Config{
			Secrets: []config.Secret{
				{
					Path:      "test/no-ownership",
					Reference: "op://vault/item/field",
					// No owner or group specified - should work fine
				},
			},
		}

		if err := processor.Process(cfg); err != nil {
			t.Errorf("No ownership should not fail: %v", err)
		}

		// Verify file was created
		filePath := filepath.Join(tmpDir, "test/no-ownership")
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("File should exist: %v", err)
		}
	})
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsAtIndex(s, substr))))
}

func containsAtIndex(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
