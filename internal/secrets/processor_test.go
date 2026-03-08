package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	result, err := processor.Process(cfg)
	if err != nil {
		t.Fatalf("Failed to process secrets: %v", err)
	}

	if result.ProcessedCount != 1 {
		t.Errorf("Expected 1 processed secret, got %d", result.ProcessedCount)
	}

	if len(result.SecretPaths) != 1 {
		t.Errorf("Expected 1 secret path, got %d", len(result.SecretPaths))
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
	result, err := processor.Process(cfg)
	if err != nil {
		t.Fatalf("Failed to process secrets: %v", err)
	}

	if result.ProcessedCount != 2 {
		t.Errorf("Expected 2 processed secrets, got %d", result.ProcessedCount)
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

		_, err := processor.Process(cfg)
		if err != nil {
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

		_, err := processor.Process(cfg)
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

		_, err := processor.Process(cfg)
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

		_, err := processor.Process(cfg)
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

		_, err := processor.Process(cfg)
		if err != nil {
			t.Errorf("No ownership should not fail: %v", err)
		}

		// Verify file was created
		filePath := filepath.Join(tmpDir, "test/no-ownership")
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("File should exist: %v", err)
		}
	})
}

func TestResolveSecretPathHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	processor := NewProcessor(&mockClient{}, tmpDir)
	processor.defaults = map[string]string{"service": "api"}
	processor.pathTemplate = "/run/secrets/{service}/{name}"

	if got := processor.resolveSecretPath("relative/path", "secret"); got != filepath.Join(tmpDir, "relative/path") {
		t.Fatalf("unexpected relative path: %q", got)
	}
	if got := processor.resolveSecretPath("/absolute/path", "secret"); got != "/absolute/path" {
		t.Fatalf("unexpected absolute path: %q", got)
	}

	resolved, err := processor.resolveSecretPathWithTemplate(config.Secret{
		Variables: map[string]string{"name": "token"},
	}, "secret")
	if err != nil {
		t.Fatalf("expected template resolution to succeed, got %v", err)
	}
	if resolved != "/run/secrets/api/token" {
		t.Fatalf("unexpected template path: %q", resolved)
	}

	resolved, err = processor.resolveSecretPathWithTemplate(config.Secret{
		Path:      "configs/{service}.txt",
		Variables: map[string]string{"service": "web"},
	}, "secret")
	if err != nil {
		t.Fatalf("expected explicit path resolution to succeed, got %v", err)
	}
	if resolved != filepath.Join(tmpDir, "configs/web.txt") {
		t.Fatalf("unexpected explicit path: %q", resolved)
	}

	processor.pathTemplate = ""
	if _, err := processor.resolveSecretPathWithTemplate(config.Secret{}, "secret"); err == nil {
		t.Fatal("expected error when no path or template is configured")
	}
}

func TestValidateSecretPathAndVariables(t *testing.T) {
	processor := NewProcessor(&mockClient{}, t.TempDir())

	if err := processor.validateSecretPath(filepath.Join(t.TempDir(), "safe", "secret"), "secret"); err != nil {
		t.Fatalf("expected safe path to validate, got %v", err)
	}

	if err := processor.validateSecretPath("../secret", "secret"); err == nil {
		t.Fatal("expected traversal error")
	}
	if err := processor.validateSecretPath("/etc/passwd", "secret"); err == nil {
		t.Fatal("expected dangerous path error")
	}

	processor.defaults = map[string]string{"service": "api"}
	resolved, err := processor.substituteVariables("/run/{service}/{name}", map[string]string{"name": "token"}, "secret")
	if err != nil {
		t.Fatalf("expected substitution success, got %v", err)
	}
	if resolved != "/run/api/token" {
		t.Fatalf("unexpected substitution result: %q", resolved)
	}

	if _, err := processor.substituteVariables("/run/{missing}", nil, "secret"); err == nil {
		t.Fatal("expected missing variable error")
	}
	if err := processor.validateVariableValue("../bad", "name", "secret"); err == nil {
		t.Fatal("expected variable validation error")
	}
}

func TestProcessSecretCreatesSymlinksAndUsesTemplateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	client := &mockClient{secrets: map[string]string{"op://vault/item/field": "templated-secret"}}
	processor := NewProcessorWithConfig(client, tmpDir, "services/{service}/{name}", map[string]string{"service": "api"})

	result, err := processor.Process(&config.Config{
		PathTemplate: "services/{service}/{name}",
		Defaults:     map[string]string{"service": "api"},
		Secrets: []config.Secret{
			{
				Reference: "op://vault/item/field",
				Variables: map[string]string{"name": "token"},
				Symlinks:  []string{filepath.Join(tmpDir, "links", "token")},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected process success, got %v", err)
	}

	if result.ProcessedCount != 1 {
		t.Fatalf("expected one processed secret, got %d", result.ProcessedCount)
	}

	targetPath := filepath.Join(tmpDir, "services", "api", "token")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("expected target file, got %v", err)
	}
	if string(content) != "templated-secret" {
		t.Fatalf("unexpected content: %q", string(content))
	}

	symlinkPath := filepath.Join(tmpDir, "links", "token")
	linkTarget, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("expected symlink, got %v", err)
	}
	if linkTarget != targetPath {
		t.Fatalf("unexpected symlink target: %q", linkTarget)
	}
}

func TestProcessSecretErrors(t *testing.T) {
	tmpDir := t.TempDir()
	processor := NewProcessor(&mockClient{}, tmpDir)

	_, err := processor.processSecret(config.Secret{Reference: "op://missing/item/field", Path: "missing"}, "secret")
	if err == nil || !strings.Contains(err.Error(), "Failed to resolve") {
		t.Fatalf("expected resolve error, got %v", err)
	}

	client := &mockClient{secrets: map[string]string{"op://vault/item/field": "value"}}
	processor = NewProcessor(client, tmpDir)
	_, err = processor.processSecret(config.Secret{Reference: "op://vault/item/field", Path: "/etc/passwd"}, "secret")
	if err == nil {
		t.Fatal("expected invalid path error")
	}
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
