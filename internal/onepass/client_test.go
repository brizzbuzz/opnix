package onepass

import (
    "os"
    "path/filepath"
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
        os.Setenv("OP_SERVICE_ACCOUNT_TOKEN", expected)
        defer os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")

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
        expected := "ops_test_token_from_file"
        tokenFile := filepath.Join(tmpDir, "token")
        if err := os.WriteFile(tokenFile, []byte(expected), 0600); err != nil {
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
}

// Note: We'll skip actual client initialization tests since they require valid tokens
