package errors

import (
	"fmt"
	"strings"
	"testing"
)

func TestOpnixError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *OpnixError
		expected []string // Expected strings that should be in the output
	}{
		{
			name: "Complete error with all fields",
			err: &OpnixError{
				Operation:   "File operation",
				Component:   "file system",
				Issue:       "Permission denied",
				Context:     "Target path: /etc/ssl/certs/app.pem",
				Suggestions: []string{"Check permissions", "Create directory"},
				Cause:       fmt.Errorf("underlying error"),
			},
			expected: []string{
				"ERROR: File operation failed in file system",
				"Issue: Permission denied",
				"Context: Target path: /etc/ssl/certs/app.pem",
				"Cause: underlying error",
				"Suggestions:",
				"1. Check permissions",
				"2. Create directory",
			},
		},
		{
			name: "Minimal error with just operation",
			err: &OpnixError{
				Operation: "Token validation",
			},
			expected: []string{
				"ERROR: Token validation failed",
			},
		},
		{
			name: "Error without operation but with component",
			err: &OpnixError{
				Component: "configuration",
				Issue:     "Invalid JSON",
			},
			expected: []string{
				"ERROR: Operation failed",
				"Issue: Invalid JSON",
			},
		},
		{
			name: "Error with suggestions only",
			err: &OpnixError{
				Issue:       "User not found",
				Suggestions: []string{"Create user", "Use existing user"},
			},
			expected: []string{
				"ERROR: Operation failed",
				"Issue: User not found",
				"Suggestions:",
				"1. Create user",
				"2. Use existing user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected error message to contain %q, but got:\n%s", expected, result)
				}
			}
		})
	}
}

func TestOpnixError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("original error")
	err := &OpnixError{
		Operation: "test",
		Cause:     cause,
	}

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("Expected unwrapped error to be %v, got %v", cause, unwrapped)
	}

	// Test nil case
	errNoCause := &OpnixError{Operation: "test"}
	if unwrapped := errNoCause.Unwrap(); unwrapped != nil {
		t.Errorf("Expected unwrapped error to be nil, got %v", unwrapped)
	}
}

func TestConfigError(t *testing.T) {
	cause := fmt.Errorf("json parse error")
	err := ConfigError("Parsing config", "Invalid JSON format", cause)

	if err.Operation != "Parsing config" {
		t.Errorf("Expected operation 'Parsing config', got %q", err.Operation)
	}
	if err.Component != "configuration" {
		t.Errorf("Expected component 'configuration', got %q", err.Component)
	}
	if err.Issue != "Invalid JSON format" {
		t.Errorf("Expected issue 'Invalid JSON format', got %q", err.Issue)
	}
	if err.Cause != cause {
		t.Errorf("Expected cause to be %v, got %v", cause, err.Cause)
	}
}

func TestConfigValidationError(t *testing.T) {
	suggestions := []string{"Fix the field", "Check documentation"}
	err := ConfigValidationError("owner", "nonexistent", "User does not exist", suggestions)

	if err.Operation != "Configuration validation" {
		t.Errorf("Expected operation 'Configuration validation', got %q", err.Operation)
	}
	if err.Component != "configuration" {
		t.Errorf("Expected component 'configuration', got %q", err.Component)
	}
	if err.Issue != "User does not exist" {
		t.Errorf("Expected issue 'User does not exist', got %q", err.Issue)
	}
	if !strings.Contains(err.Context, "owner") {
		t.Errorf("Expected context to contain 'owner', got %q", err.Context)
	}
	if len(err.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(err.Suggestions))
	}
}

func TestFileOperationError(t *testing.T) {
	cause := fmt.Errorf("permission denied")

	tests := []struct {
		name                string
		operation           string
		path                string
		issue               string
		expectedSuggestions int
	}{
		{
			name:                "Permission denied error",
			operation:           "Writing file",
			path:                "/etc/ssl/cert.pem",
			issue:               "permission denied",
			expectedSuggestions: 3, // Should suggest mkdir, permissions check, etc.
		},
		{
			name:                "File not found error",
			operation:           "Reading file",
			path:                "/missing/file.txt",
			issue:               "no such file or directory",
			expectedSuggestions: 2, // Should suggest mkdir and path verification
		},
		{
			name:                "Disk space error",
			operation:           "Writing file",
			path:                "/tmp/large.file",
			issue:               "no space left on device",
			expectedSuggestions: 2, // Should suggest disk space check
		},
		{
			name:                "Generic error",
			operation:           "File operation",
			path:                "/some/path",
			issue:               "unknown error",
			expectedSuggestions: 0, // No specific suggestions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FileOperationError(tt.operation, tt.path, tt.issue, cause)

			if err.Operation != tt.operation {
				t.Errorf("Expected operation %q, got %q", tt.operation, err.Operation)
			}
			if err.Component != "file system" {
				t.Errorf("Expected component 'file system', got %q", err.Component)
			}
			if !strings.Contains(err.Context, tt.path) {
				t.Errorf("Expected context to contain path %q, got %q", tt.path, err.Context)
			}
			if len(err.Suggestions) != tt.expectedSuggestions {
				t.Errorf("Expected %d suggestions, got %d: %v", tt.expectedSuggestions, len(err.Suggestions), err.Suggestions)
			}
		})
	}
}

func TestOnePasswordError(t *testing.T) {
	cause := fmt.Errorf("auth failed")

	tests := []struct {
		name           string
		operation      string
		issue          string
		minSuggestions int
	}{
		{
			name:           "Authentication error",
			operation:      "Authenticating",
			issue:          "authentication failed",
			minSuggestions: 3, // Should suggest token verification, etc.
		},
		{
			name:           "Reference not found",
			operation:      "Resolving secret",
			issue:          "reference not found",
			minSuggestions: 3, // Should suggest reference format check, etc.
		},
		{
			name:           "Network error",
			operation:      "Connecting",
			issue:          "network timeout",
			minSuggestions: 3, // Should suggest connectivity check, etc.
		},
		{
			name:           "Rate limit error",
			operation:      "API call",
			issue:          "rate limit exceeded",
			minSuggestions: 2, // Should suggest waiting, etc.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := OnePasswordError(tt.operation, tt.issue, cause)

			if err.Component != "1Password integration" {
				t.Errorf("Expected component '1Password integration', got %q", err.Component)
			}
			if len(err.Suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(err.Suggestions))
			}
		})
	}
}

func TestUserGroupError(t *testing.T) {
	availableUsers := []string{"root", "nginx", "postgres"}
	err := UserGroupError("Setting ownership", "nonexistent", "user", availableUsers)

	if err.Component != "user management" {
		t.Errorf("Expected component 'user management', got %q", err.Component)
	}
	if !strings.Contains(err.Issue, "nonexistent") {
		t.Errorf("Expected issue to contain 'nonexistent', got %q", err.Issue)
	}
	if !strings.Contains(err.Issue, "user") {
		t.Errorf("Expected issue to contain 'user', got %q", err.Issue)
	}

	// Should have suggestions for creating user and using existing ones
	if len(err.Suggestions) < 2 {
		t.Errorf("Expected at least 2 suggestions, got %d", len(err.Suggestions))
	}

	// Check that available users are mentioned in suggestions
	fullError := err.Error()
	for _, user := range availableUsers {
		if !strings.Contains(fullError, user) {
			t.Errorf("Expected error to mention available user %q", user)
		}
	}
}

func TestValidationError(t *testing.T) {
	err := ValidationError("Field validation", "mode", "777", "3-4 digit octal")

	if err.Component != "validation" {
		t.Errorf("Expected component 'validation', got %q", err.Component)
	}
	if !strings.Contains(err.Issue, "mode") {
		t.Errorf("Expected issue to contain field name 'mode', got %q", err.Issue)
	}
	if !strings.Contains(err.Context, "3-4 digit octal") {
		t.Errorf("Expected context to contain expected format, got %q", err.Context)
	}
}

func TestTokenError(t *testing.T) {
	cause := fmt.Errorf("file not found")
	err := TokenError("Token file missing", "/etc/opnix-token", cause)

	if err.Component != "authentication" {
		t.Errorf("Expected component 'authentication', got %q", err.Component)
	}
	if !strings.Contains(err.Context, "/etc/opnix-token") {
		t.Errorf("Expected context to contain token path, got %q", err.Context)
	}

	// Should have comprehensive setup instructions
	if len(err.Suggestions) < 5 {
		t.Errorf("Expected at least 5 suggestions for token setup, got %d", len(err.Suggestions))
	}

	// Check for key setup instructions
	fullError := err.Error()
	expectedInstructions := []string{
		"1password.com",
		"service account",
		"opnix token set",
		"chmod",
	}

	for _, instruction := range expectedInstructions {
		if !strings.Contains(fullError, instruction) {
			t.Errorf("Expected token error to contain instruction about %q", instruction)
		}
	}
}

func TestWrap(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	wrappedErr := Wrap(originalErr, "Test operation", "test component")

	opnixErr, ok := wrappedErr.(*OpnixError)
	if !ok {
		t.Fatalf("Expected wrapped error to be *OpnixError, got %T", wrappedErr)
	}

	if opnixErr.Operation != "Test operation" {
		t.Errorf("Expected operation 'Test operation', got %q", opnixErr.Operation)
	}
	if opnixErr.Component != "test component" {
		t.Errorf("Expected component 'test component', got %q", opnixErr.Component)
	}
	if opnixErr.Cause != originalErr {
		t.Errorf("Expected cause to be original error, got %v", opnixErr.Cause)
	}

	// Test nil case
	if wrappedNil := Wrap(nil, "operation", "component"); wrappedNil != nil {
		t.Errorf("Expected nil error to remain nil, got %v", wrappedNil)
	}
}

func TestWrapWithSuggestions(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	suggestions := []string{"Try this", "Try that"}
	wrappedErr := WrapWithSuggestions(originalErr, "Test operation", "test component", suggestions)

	opnixErr, ok := wrappedErr.(*OpnixError)
	if !ok {
		t.Fatalf("Expected wrapped error to be *OpnixError, got %T", wrappedErr)
	}

	if len(opnixErr.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(opnixErr.Suggestions))
	}

	// Test nil case
	if wrappedNil := WrapWithSuggestions(nil, "op", "comp", suggestions); wrappedNil != nil {
		t.Errorf("Expected nil error to remain nil, got %v", wrappedNil)
	}
}

func TestGetDirPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/etc/ssl/certs/app.pem", "/etc/ssl/certs"},
		{"/file.txt", "/"},
		{"relative/path/file.txt", "relative/path"},
		{"file.txt", "."},
		{"", "."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := getDirPath(tt.input)
			if result != tt.expected {
				t.Errorf("getDirPath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetUserGroupCommand(t *testing.T) {
	tests := []struct {
		entityType string
		expected   string
	}{
		{"user", "useradd"},
		{"group", "groupadd"},
		{"unknown", "groupadd"}, // Default case
	}

	for _, tt := range tests {
		t.Run(tt.entityType, func(t *testing.T) {
			result := getUserGroupCommand(tt.entityType)
			if result != tt.expected {
				t.Errorf("getUserGroupCommand(%q) = %q, expected %q", tt.entityType, result, tt.expected)
			}
		})
	}
}
