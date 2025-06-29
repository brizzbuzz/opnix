package validation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brizzbuzz/opnix/internal/errors"
)

func TestValidator_ValidateConfig(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		secrets   []SecretData
		wantError bool
		errorType string
	}{
		{
			name:      "empty config",
			secrets:   []SecretData{},
			wantError: true,
			errorType: "No secrets defined",
		},
		{
			name: "valid config",
			secrets: []SecretData{
				{
					Path:      "database/password",
					Reference: "op://Vault/Database/password",
					Owner:     "root",
					Group:     "root",
					Mode:      "0600",
				},
			},
			wantError: false,
		},
		{
			name: "duplicate paths",
			secrets: []SecretData{
				{
					Path:      "same/path",
					Reference: "op://Vault/Item1/field",
				},
				{
					Path:      "same/path",
					Reference: "op://Vault/Item2/field",
				},
			},
			wantError: true,
			errorType: "Duplicate path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateConfigStruct(tt.secrets)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorType != "" && !containsString(err.Error(), tt.errorType) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateReference(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		reference string
		wantError bool
		errorType string
	}{
		{
			name:      "empty reference",
			reference: "",
			wantError: true,
			errorType: "Reference cannot be empty",
		},
		{
			name:      "valid reference",
			reference: "op://Vault/Item/field",
			wantError: false,
		},
		{
			name:      "invalid format - no op prefix",
			reference: "vault/item/field",
			wantError: true,
			errorType: "Invalid 1Password reference format",
		},
		{
			name:      "invalid format - too few parts",
			reference: "op://Vault/Item",
			wantError: true,
			errorType: "at least 3 parts",
		},
		{
			name:      "valid format - with section",
			reference: "op://Vault/Item/Section/field",
			wantError: false,
		},
		{
			name:      "valid format - with nested sections",
			reference: "op://Homelab/Cloudflare Origin Certs/rgbr.ink/cert",
			wantError: false,
		},
		{
			name:      "valid format - deeply nested sections",
			reference: "op://Work/API Keys/Production/GitHub/token",
			wantError: false,
		},
		{
			name:      "valid format - section with spaces",
			reference: "op://Homelab/SSL Certificates/example.com/private key",
			wantError: false,
		},
		{
			name:      "empty field in sectioned reference",
			reference: "op://Vault/Item/Section/",
			wantError: true,
			errorType: "Field name cannot be empty",
		},
		{
			name:      "empty vault",
			reference: "op:///Item/field",
			wantError: true,
			errorType: "Vault name cannot be empty",
		},
		{
			name:      "empty item",
			reference: "op://Vault//field",
			wantError: true,
			errorType: "Item name cannot be empty",
		},
		{
			name:      "empty field",
			reference: "op://Vault/Item/",
			wantError: true,
			errorType: "Field name cannot be empty",
		},
		{
			name:      "valid complex reference",
			reference: "op://My-Vault/Complex_Item-Name/custom.field",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateReference(tt.reference, "test-secret")

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorType != "" && !containsString(err.Error(), tt.errorType) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidatePath(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		path      string
		wantError bool
		errorType string
	}{
		{
			name:      "empty path",
			path:      "",
			wantError: true,
			errorType: "Path cannot be empty",
		},
		{
			name:      "valid relative path",
			path:      "database/password",
			wantError: false,
		},
		{
			name:      "valid absolute path",
			path:      "/etc/ssl/certs/app.pem",
			wantError: false,
		},
		{
			name:      "path traversal attempt",
			path:      "../../../etc/passwd",
			wantError: true,
			errorType: "Path traversal detected",
		},
		{
			name:      "dangerous absolute path - /etc/passwd",
			path:      "/etc/passwd",
			wantError: true,
			errorType: "potentially dangerous location",
		},
		{
			name:      "dangerous absolute path - /bin",
			path:      "/bin/something",
			wantError: true,
			errorType: "potentially dangerous location",
		},
		{
			name:      "safe absolute path",
			path:      "/var/lib/app/secret",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seenPaths := make(map[string]string)
			err := validator.validatePath(tt.path, "test-secret", seenPaths)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorType != "" && !containsString(err.Error(), tt.errorType) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateMode(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		mode      string
		wantError bool
		errorType string
	}{
		{
			name:      "empty mode (valid - uses default)",
			mode:      "",
			wantError: false,
		},
		{
			name:      "valid mode 0600",
			mode:      "0600",
			wantError: false,
		},
		{
			name:      "valid mode 0644",
			mode:      "0644",
			wantError: false,
		},
		{
			name:      "valid mode 0755",
			mode:      "0755",
			wantError: false,
		},
		{
			name:      "invalid mode - not octal",
			mode:      "999",
			wantError: true,
			errorType: "3-4 digit octal number",
		},
		{
			name:      "invalid mode - contains letters",
			mode:      "0abc",
			wantError: true,
			errorType: "3-4 digit octal number",
		},
		{
			name:      "invalid mode - too short",
			mode:      "60",
			wantError: true,
			errorType: "3-4 digit octal number",
		},
		{
			name:      "insecure mode - world readable",
			mode:      "0604",
			wantError: false, // We now allow world-readable modes
		},
		{
			name:      "insecure mode - world writable",
			mode:      "0602",
			wantError: true,
			errorType: "world write access",
		},
		{
			name:      "valid 4-digit mode",
			mode:      "0600",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateMode(tt.mode, "test-secret")

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorType != "" && !containsString(err.Error(), tt.errorType) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateUser(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		username  string
		wantError bool
	}{
		{
			name:      "root user (always exists)",
			username:  "root",
			wantError: false,
		},
		{
			name:      "nonexistent user",
			username:  "nonexistent-user-12345",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateUser(tt.username, "test-secret")

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else {
					// Check that it's a UserGroupError
					if opnixErr, ok := err.(*errors.OpnixError); ok {
						if opnixErr.Component != "user management" {
							t.Errorf("Expected user management error, got component: %s", opnixErr.Component)
						}
					} else {
						t.Errorf("Expected OpnixError, got: %T", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateGroup(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		groupname string
		wantError bool
	}{
		{
			name:      "root group (always exists)",
			groupname: "root",
			wantError: false,
		},
		{
			name:      "nonexistent group",
			groupname: "nonexistent-group-12345",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateGroup(tt.groupname, "test-secret")

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else {
					// Check that it's a UserGroupError
					if opnixErr, ok := err.(*errors.OpnixError); ok {
						if opnixErr.Component != "user management" {
							t.Errorf("Expected user management error, got component: %s", opnixErr.Component)
						}
					} else {
						t.Errorf("Expected OpnixError, got: %T", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateTokenFile(t *testing.T) {
	validator := NewValidator()

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "opnix-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test cases
	tests := []struct {
		name      string
		setup     func() string // Returns token file path
		wantError bool
		errorType string
	}{
		{
			name: "nonexistent file",
			setup: func() string {
				return filepath.Join(tempDir, "nonexistent")
			},
			wantError: true,
			errorType: "does not exist",
		},
		{
			name: "empty file",
			setup: func() string {
				tokenFile := filepath.Join(tempDir, "empty-token")
				os.WriteFile(tokenFile, []byte(""), 0600)
				return tokenFile
			},
			wantError: true,
			errorType: "empty",
		},
		{
			name: "whitespace only file",
			setup: func() string {
				tokenFile := filepath.Join(tempDir, "whitespace-token")
				os.WriteFile(tokenFile, []byte("   \n\t  "), 0600)
				return tokenFile
			},
			wantError: true,
			errorType: "empty",
		},
		{
			name: "valid token file",
			setup: func() string {
				tokenFile := filepath.Join(tempDir, "valid-token")
				os.WriteFile(tokenFile, []byte("valid-token-content"), 0600)
				return tokenFile
			},
			wantError: false,
		},
		{
			name: "unreadable file",
			setup: func() string {
				tokenFile := filepath.Join(tempDir, "unreadable-token")
				os.WriteFile(tokenFile, []byte("token"), 0000) // No read permissions
				return tokenFile
			},
			wantError: true,
			errorType: "Cannot read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenPath := tt.setup()
			err := validator.ValidateTokenFile(tokenPath)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorType != "" && !containsString(err.Error(), tt.errorType) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorType, err)
				}
				// Should be a TokenError
				if opnixErr, ok := err.(*errors.OpnixError); ok {
					if opnixErr.Component != "authentication" {
						t.Errorf("Expected authentication error, got component: %s", opnixErr.Component)
					}
				} else {
					t.Errorf("Expected OpnixError, got: %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_GetAvailableUsers(t *testing.T) {
	validator := NewValidator()
	users := validator.getAvailableUsers()

	// Should always include root
	if !containsStringSlice(users, "root") {
		t.Errorf("Expected available users to include 'root', got: %v", users)
	}

	// Should be limited to reasonable number
	if len(users) > 10 {
		t.Errorf("Expected at most 10 users, got %d", len(users))
	}
}

func TestValidator_GetAvailableGroups(t *testing.T) {
	validator := NewValidator()
	groups := validator.getAvailableGroups()

	// Should always include root
	if !containsStringSlice(groups, "root") {
		t.Errorf("Expected available groups to include 'root', got: %v", groups)
	}

	// Should be limited to reasonable number
	if len(groups) > 10 {
		t.Errorf("Expected at most 10 groups, got %d", len(groups))
	}
}

func TestIsServiceUser(t *testing.T) {
	tests := []struct {
		username string
		expected bool
	}{
		{"nginx", true},
		{"postgres", true},
		{"caddy", true},
		{"root", false},
		{"randomuser", false},
		{"nobody", true},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			result := isServiceUser(tt.username)
			if result != tt.expected {
				t.Errorf("isServiceUser(%q) = %v, expected %v", tt.username, result, tt.expected)
			}
		})
	}
}

func TestIsServiceGroup(t *testing.T) {
	tests := []struct {
		groupname string
		expected  bool
	}{
		{"nginx", true},
		{"postgres", true},
		{"ssl-cert", true},
		{"root", false},
		{"randomgroup", false},
		{"docker", true},
	}

	for _, tt := range tests {
		t.Run(tt.groupname, func(t *testing.T) {
			result := isServiceGroup(tt.groupname)
			if result != tt.expected {
				t.Errorf("isServiceGroup(%q) = %v, expected %v", tt.groupname, result, tt.expected)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{10, 10, 10},
		{0, 1, 0},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := min(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("min(%d, %d) = %d, expected %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// Helper functions

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) >= len(substr) &&
			findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(s) < len(substr) {
		return -1
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func containsStringSlice(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
