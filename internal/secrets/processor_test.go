package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brizzbuzz/opnix/internal/config"
	"github.com/brizzbuzz/opnix/internal/errors"
)

// Mock client for testing
type mockClient struct {
	secrets      map[string]string
	files        map[string][]byte
	resolveCalls map[string]int
	resolveErr   error
	fileErr      error
	fileCalls    int
}

func (m *mockClient) ResolveFiles(references []string) (map[string][]byte, error) {
	m.fileCalls++
	if m.fileErr != nil {
		return nil, m.fileErr
	}
	resolved := make(map[string][]byte, len(references))
	for _, reference := range references {
		value, ok := m.files[reference]
		if !ok {
			return nil, fmt.Errorf("file not found")
		}
		resolved[reference] = value
	}
	return resolved, nil
}

func (m *mockClient) ResolveSecret(reference string) (string, error) {
	if m.resolveCalls != nil {
		m.resolveCalls[reference]++
	}
	if value, ok := m.secrets[reference]; ok {
		return value, nil
	}
	return "", fmt.Errorf("secret not found")
}

func (m *mockClient) ResolveSecrets(references []string) (map[string]string, error) {
	if m.resolveErr != nil {
		return nil, m.resolveErr
	}

	resolved := make(map[string]string, len(references))
	for _, reference := range references {
		value, err := m.ResolveSecret(reference)
		if err != nil {
			return nil, err
		}
		resolved[reference] = value
	}
	return resolved, nil
}

func TestProcessorMissingReferencePreservesExistingSecrets(t *testing.T) {
	const reference = "op://Example/Service/old-field"
	providerErr := &errors.ProviderResolutionError{Failures: []*errors.ProviderError{{
		Kind:      errors.ProviderErrorMissingReference,
		Reference: reference,
		Issue:     "field_not_found",
	}}}
	tmpDir := t.TempDir()
	existingPath := filepath.Join(tmpDir, "database/password")
	if err := os.MkdirAll(filepath.Dir(existingPath), 0755); err != nil {
		t.Fatalf("Failed to create existing secret directory: %v", err)
	}
	if err := os.WriteFile(existingPath, []byte("existing-secret"), 0600); err != nil {
		t.Fatalf("Failed to create existing secret: %v", err)
	}

	processor := NewProcessor(&mockClient{resolveErr: providerErr}, tmpDir)
	result, err := processor.Process(&config.Config{Secrets: []config.Secret{{
		Path:      "database/password",
		Reference: reference,
	}}})
	if err == nil {
		t.Fatal("Expected missing reference error")
	}
	if result != nil {
		t.Fatalf("Expected no process result, got %#v", result)
	}
	if got := errors.ExitCode(err); got != errors.ExitCodeMissingReference {
		t.Fatalf("Expected exit code %d, got %d", errors.ExitCodeMissingReference, got)
	}
	if !strings.Contains(err.Error(), "older NixOS generation") {
		t.Fatalf("Expected rollback recovery suggestion, got:\n%s", err)
	}

	content, readErr := os.ReadFile(existingPath)
	if readErr != nil {
		t.Fatalf("Failed to read existing secret: %v", readErr)
	}
	if string(content) != "existing-secret" {
		t.Fatalf("Expected existing secret to remain unchanged, got %q", content)
	}
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

func TestProcessorWritesBinaryFileExactly(t *testing.T) {
	const reference = "op://vault/document/private-key"
	want := []byte{0x00, 0xff, 0xfe, '\n', 0x80, 0x01}
	tmpDir := t.TempDir()
	processor := NewProcessor(&mockClient{files: map[string][]byte{reference: want}}, tmpDir)

	_, err := processor.Process(&config.Config{Secrets: []config.Secret{{
		Path:      "keys/private.key",
		Reference: reference,
		Kind:      config.SecretKindFile,
	}}})
	if err != nil {
		t.Fatalf("Failed to process binary file: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(tmpDir, "keys/private.key"))
	if err != nil {
		t.Fatalf("Failed to read binary output: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("Binary output changed: got %v, want %v", got, want)
	}
}

func TestProcessorResolvesAllKindsBeforeWrites(t *testing.T) {
	const (
		fieldReference = "op://vault/item/password"
		fileReference  = "op://vault/item/private-key"
	)
	tmpDir := t.TempDir()
	existingPath := filepath.Join(tmpDir, "password")
	if err := os.WriteFile(existingPath, []byte("existing-password"), 0600); err != nil {
		t.Fatalf("Failed to create existing secret: %v", err)
	}
	fileErr := &errors.ProviderResolutionError{Failures: []*errors.ProviderError{{
		Kind:      errors.ProviderErrorMissingReference,
		Reference: fileReference,
		Issue:     "file not found",
	}}}
	processor := NewProcessor(&mockClient{
		secrets:      map[string]string{fieldReference: "new-password"},
		fileErr:      fileErr,
		resolveCalls: map[string]int{},
	}, tmpDir)

	result, err := processor.Process(&config.Config{Secrets: []config.Secret{
		{Path: "password", Reference: fieldReference},
		{Path: "private-key", Reference: fileReference, Kind: config.SecretKindFile},
	}})
	if err == nil {
		t.Fatal("Expected file resolution error")
	}
	if result != nil {
		t.Fatalf("Expected no result, got %#v", result)
	}
	if got := errors.ExitCode(err); got != errors.ExitCodeMissingReference {
		t.Fatalf("Expected exit code %d, got %d", errors.ExitCodeMissingReference, got)
	}
	content, readErr := os.ReadFile(existingPath)
	if readErr != nil {
		t.Fatalf("Failed to read existing secret: %v", readErr)
	}
	if string(content) != "existing-password" {
		t.Fatalf("Field was updated before file resolution completed: %q", content)
	}
}

func TestProcessorResolutionFailureDoesNotCreateOutputDirectory(t *testing.T) {
	const reference = "op://vault/item/missing.bin"
	outputDir := filepath.Join(t.TempDir(), "secrets")
	processor := NewProcessor(&mockClient{fileErr: &errors.ProviderResolutionError{Failures: []*errors.ProviderError{{
		Kind: errors.ProviderErrorMissingReference, Reference: reference, Issue: "file not found",
	}}}}, outputDir)

	_, err := processor.Process(&config.Config{Secrets: []config.Secret{{
		Path: "missing.bin", Reference: reference, Kind: config.SecretKindFile,
	}}})
	if err == nil {
		t.Fatal("Expected resolution failure")
	}
	if _, statErr := os.Stat(outputDir); !os.IsNotExist(statErr) {
		t.Fatalf("Expected output directory to remain absent, got %v", statErr)
	}
}

func TestProcessorResolvesDuplicateReferencesOnce(t *testing.T) {
	mock := &mockClient{
		secrets: map[string]string{
			"op://vault/shared/field": "shared-secret-value",
		},
		resolveCalls: map[string]int{},
	}

	tmpDir, err := os.MkdirTemp("", "opnix-processor-duplicate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	processor := NewProcessor(mock, tmpDir)
	cfg := &config.Config{Secrets: []config.Secret{
		{Path: "first", Reference: "op://vault/shared/field"},
		{Path: "second", Reference: "op://vault/shared/field"},
	}}

	result, err := processor.Process(cfg)
	if err != nil {
		t.Fatalf("Failed to process duplicate references: %v", err)
	}

	if result.ProcessedCount != 2 {
		t.Fatalf("Expected 2 processed secrets, got %d", result.ProcessedCount)
	}

	if got := mock.resolveCalls["op://vault/shared/field"]; got != 1 {
		t.Fatalf("Expected duplicate reference to be resolved once, got %d", got)
	}
}

func TestProcessorResolvesDuplicateFileReferenceOnce(t *testing.T) {
	const reference = "op://vault/item/archive.bin"
	want := []byte{0x00, 0xff, 0x01}
	mock := &mockClient{files: map[string][]byte{reference: want}}
	tmpDir := t.TempDir()
	processor := NewProcessor(mock, tmpDir)

	result, err := processor.Process(&config.Config{Secrets: []config.Secret{
		{Path: "first.bin", Reference: reference, Kind: config.SecretKindFile},
		{Path: "second.bin", Reference: reference, Kind: config.SecretKindFile},
	}})
	if err != nil {
		t.Fatalf("Failed to process duplicate file reference: %v", err)
	}
	if result.ProcessedCount != 2 {
		t.Fatalf("Expected 2 processed files, got %d", result.ProcessedCount)
	}
	if mock.fileCalls != 1 {
		t.Fatalf("Expected one batched file resolution, got %d", mock.fileCalls)
	}
	for _, name := range []string{"first.bin", "second.bin"} {
		got, readErr := os.ReadFile(filepath.Join(tmpDir, name))
		if readErr != nil {
			t.Fatalf("Failed to read %s: %v", name, readErr)
		}
		if string(got) != string(want) {
			t.Fatalf("File %s changed bytes: got %v, want %v", name, got, want)
		}
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

func TestProcessorReconcilesExistingFileMode(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string
		secretContent   string
		existingMode    os.FileMode
		declaredMode    string
		expectedMode    os.FileMode
	}{
		{
			name:            "content and mode changed",
			existingContent: "old-secret-value",
			secretContent:   "new-secret-value",
			existingMode:    0600,
			declaredMode:    "0400",
			expectedMode:    0400,
		},
		{
			name:            "mode changed with identical content",
			existingContent: "same-secret-value",
			secretContent:   "same-secret-value",
			existingMode:    0600,
			declaredMode:    "0400",
			expectedMode:    0400,
		},
		{
			name:            "default mode restored",
			existingContent: "same-secret-value",
			secretContent:   "same-secret-value",
			existingMode:    0644,
			expectedMode:    0600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, "secret")
			if err := os.WriteFile(outputPath, []byte(tt.existingContent), tt.existingMode); err != nil {
				t.Fatalf("Failed to create existing secret: %v", err)
			}
			if err := os.Chmod(outputPath, tt.existingMode); err != nil {
				t.Fatalf("Failed to set existing secret mode: %v", err)
			}

			processor := NewProcessor(&mockClient{secrets: map[string]string{
				"op://vault/item/field": tt.secretContent,
			}}, tmpDir)
			cfg := &config.Config{Secrets: []config.Secret{{
				Path:      "secret",
				Reference: "op://vault/item/field",
				Mode:      tt.declaredMode,
			}}}

			if _, err := processor.Process(cfg); err != nil {
				t.Fatalf("Failed to process existing secret: %v", err)
			}

			content, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("Failed to read processed secret: %v", err)
			}
			if string(content) != tt.secretContent {
				t.Errorf("Expected secret content %q, got %q", tt.secretContent, string(content))
			}

			info, err := os.Stat(outputPath)
			if err != nil {
				t.Fatalf("Failed to stat processed secret: %v", err)
			}
			if info.Mode().Perm() != tt.expectedMode {
				t.Errorf("Expected permissions %04o, got %04o", tt.expectedMode, info.Mode().Perm())
			}
		})
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
