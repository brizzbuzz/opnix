package onepass

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetToken(t *testing.T) {
	// Create temp dir for test files
	tmpDir, err := os.MkdirTemp("", "opnix-tests-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test getting token from environment
	t.Run("environment token", func(t *testing.T) {
		expected := "ops_test_token"
		t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", expected)

		got, err := GetToken("")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if got != expected {
			t.Errorf("Expected token %q, got %q", expected, got)
		}
	})

	// Test getting token from file
	t.Run("file token", func(t *testing.T) {
		os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")
		expected := "ops_test_token_from_file"
		tokenFile := filepath.Join(tmpDir, "token")
		if err := os.WriteFile(tokenFile, []byte("  "+expected+"\n"), 0600); err != nil {
			t.Fatalf("Failed to write token file: %v", err)
		}

		got, err := GetToken(tokenFile)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if got != expected {
			t.Errorf("Expected token %q, got %q", expected, got)
		}
	})

	t.Run("environment overrides file", func(t *testing.T) {
		t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "env-token")
		tokenFile := filepath.Join(tmpDir, "override-token")
		if err := os.WriteFile(tokenFile, []byte("file-token"), 0600); err != nil {
			t.Fatalf("Failed to write token file: %v", err)
		}

		got, err := GetToken(tokenFile)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if got != "env-token" {
			t.Fatalf("Expected environment token, got %q", got)
		}
	})

	// Test no token provided
	t.Run("no token", func(t *testing.T) {
		os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")
		_, err := GetToken("")
		if err == nil {
			t.Error("Expected error when no token provided")
		}
	})

	// Test invalid token file
	t.Run("invalid token file", func(t *testing.T) {
		os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")
		_, err := GetToken("/nonexistent/file")
		if err == nil {
			t.Error("Expected error with invalid token file")
		}
	})

	t.Run("empty token file", func(t *testing.T) {
		os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")
		tokenFile := filepath.Join(tmpDir, "empty-token")
		if err := os.WriteFile(tokenFile, []byte(" \n\t"), 0600); err != nil {
			t.Fatalf("Failed to write token file: %v", err)
		}

		_, err := GetToken(tokenFile)
		if err == nil {
			t.Fatal("Expected error for empty token file")
		}
		if !strings.Contains(err.Error(), "empty") {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

func TestNewClient(t *testing.T) {
	t.Run("missing token returns error", func(t *testing.T) {
		os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")
		client, err := NewClient(filepath.Join(t.TempDir(), "missing-token"))
		if err == nil {
			t.Fatal("expected error for missing token")
		}
		if client != nil {
			t.Fatalf("expected nil client on error, got %#v", client)
		}
	})

	t.Run("constructor accepts token source", func(t *testing.T) {
		t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "ops_test_invalid_token")
		client, err := NewClient("")
		if err != nil {
			// Constructor behavior can vary with SDK validation, but this still covers the path.
			if !strings.Contains(err.Error(), "1Password") && !strings.Contains(err.Error(), "token") {
				t.Fatalf("unexpected constructor error: %v", err)
			}
			return
		}
		if client == nil {
			t.Fatal("expected non-nil client when constructor succeeds")
		}
	})
}
