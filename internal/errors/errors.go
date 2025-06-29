package errors

import (
	"fmt"
	"strings"
)

// OpnixError represents a structured error with context and suggestions
type OpnixError struct {
	Operation   string   // What operation was being performed
	Component   string   // Which component failed (config, onepass, secrets, etc.)
	Issue       string   // The core issue description
	Context     string   // Additional context about the failure
	Suggestions []string // List of actionable suggestions to fix the issue
	Cause       error    // Underlying error that caused this
}

func (e *OpnixError) Error() string {
	var parts []string

	// Main error message
	if e.Operation != "" && e.Component != "" {
		parts = append(parts, fmt.Sprintf("ERROR: %s failed in %s", e.Operation, e.Component))
	} else if e.Operation != "" {
		parts = append(parts, fmt.Sprintf("ERROR: %s failed", e.Operation))
	} else {
		parts = append(parts, "ERROR: Operation failed")
	}

	// Issue description
	if e.Issue != "" {
		parts = append(parts, fmt.Sprintf("  Issue: %s", e.Issue))
	}

	// Additional context
	if e.Context != "" {
		parts = append(parts, fmt.Sprintf("  Context: %s", e.Context))
	}

	// Underlying cause
	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("  Cause: %v", e.Cause))
	}

	// Suggestions
	if len(e.Suggestions) > 0 {
		parts = append(parts, "")
		parts = append(parts, "  Suggestions:")
		for i, suggestion := range e.Suggestions {
			parts = append(parts, fmt.Sprintf("  %d. %s", i+1, suggestion))
		}
	}

	return strings.Join(parts, "\n")
}

func (e *OpnixError) Unwrap() error {
	return e.Cause
}

// Error constructors for common scenarios

// ConfigError creates errors related to configuration parsing and validation
func ConfigError(operation, issue string, cause error) *OpnixError {
	return &OpnixError{
		Operation: operation,
		Component: "configuration",
		Issue:     issue,
		Cause:     cause,
	}
}

// ConfigValidationError creates detailed validation errors with suggestions
func ConfigValidationError(field, value, issue string, suggestions []string) *OpnixError {
	return &OpnixError{
		Operation:   "Configuration validation",
		Component:   "configuration",
		Issue:       issue,
		Context:     fmt.Sprintf("Field '%s' has value '%s'", field, value),
		Suggestions: suggestions,
	}
}

// FileOperationError creates errors for file system operations
func FileOperationError(operation, path, issue string, cause error) *OpnixError {
	suggestions := []string{}

	// Add context-specific suggestions
	if strings.Contains(issue, "permission denied") {
		suggestions = append(suggestions,
			fmt.Sprintf("Check if OpNix has write permissions to '%s'", path),
			fmt.Sprintf("Create parent directory: sudo mkdir -p '%s'", getDirPath(path)),
			fmt.Sprintf("Check parent directory permissions: ls -la '%s'", getDirPath(path)),
		)
	} else if strings.Contains(issue, "no such file or directory") {
		suggestions = append(suggestions,
			fmt.Sprintf("Create parent directory: sudo mkdir -p '%s'", getDirPath(path)),
			fmt.Sprintf("Verify the path is correct: '%s'", path),
		)
	} else if strings.Contains(issue, "disk") || strings.Contains(issue, "space") {
		suggestions = append(suggestions,
			"Check available disk space: df -h",
			"Clean up temporary files if needed",
		)
	}

	return &OpnixError{
		Operation:   operation,
		Component:   "file system",
		Issue:       issue,
		Context:     fmt.Sprintf("Target path: %s", path),
		Suggestions: suggestions,
		Cause:       cause,
	}
}

// OnePasswordError creates errors for 1Password integration issues
func OnePasswordError(operation, issue string, cause error) *OpnixError {
	suggestions := []string{}

	// Add context-specific suggestions based on the issue
	if strings.Contains(issue, "authentication") || strings.Contains(issue, "token") {
		suggestions = append(suggestions,
			"Verify your 1Password service account token is valid",
			"Check if the token has expired: visit 1Password admin console",
			"Ensure token file exists and is readable: ls -la /etc/opnix-token",
			"Set up token using: opnix token set",
		)
	} else if strings.Contains(issue, "not found") || strings.Contains(issue, "reference") {
		suggestions = append(suggestions,
			"Verify the 1Password reference format: op://Vault/Item/field",
			"Check if the vault, item, and field exist in 1Password",
			"Ensure the service account has access to the specified vault",
			"List available items: op item list --vault VaultName",
		)
	} else if strings.Contains(issue, "network") || strings.Contains(issue, "connection") {
		suggestions = append(suggestions,
			"Check internet connectivity",
			"Verify 1Password service is accessible",
			"Check for firewall or proxy issues",
			"Retry the operation in a few minutes",
		)
	} else if strings.Contains(issue, "rate limit") {
		suggestions = append(suggestions,
			"Wait a few minutes before retrying",
			"Reduce the number of concurrent secret requests",
			"Contact 1Password support if rate limits are too restrictive",
		)
	}

	return &OpnixError{
		Operation:   operation,
		Component:   "1Password integration",
		Issue:       issue,
		Suggestions: suggestions,
		Cause:       cause,
	}
}

// UserGroupError creates errors for user/group validation issues
func UserGroupError(operation, userOrGroup, entityType string, availableEntities []string) *OpnixError {
	suggestions := []string{
		fmt.Sprintf("Create the %s: sudo %s %s", entityType, getUserGroupCommand(entityType), userOrGroup),
	}

	if len(availableEntities) > 0 {
		suggestions = append(suggestions,
			fmt.Sprintf("Use an existing %s instead:", entityType),
		)
		for _, entity := range availableEntities {
			suggestions = append(suggestions, fmt.Sprintf("  - %s", entity))
		}
	}

	if entityType == "user" {
		suggestions = append(suggestions,
			"List all users: cut -d: -f1 /etc/passwd | sort",
		)
	} else {
		suggestions = append(suggestions,
			"List all groups: cut -d: -f1 /etc/group | sort",
		)
	}

	return &OpnixError{
		Operation:   operation,
		Component:   "user management",
		Issue:       fmt.Sprintf("%s '%s' does not exist", entityType, userOrGroup),
		Suggestions: suggestions,
	}
}

// ValidationError creates general validation errors
func ValidationError(operation, field, value, expectedFormat string) *OpnixError {
	return &OpnixError{
		Operation: operation,
		Component: "validation",
		Issue:     fmt.Sprintf("Invalid value '%s' for field '%s'", value, field),
		Context:   fmt.Sprintf("Expected format: %s", expectedFormat),
		Suggestions: []string{
			fmt.Sprintf("Update field '%s' to match the expected format", field),
			"Check the documentation for valid values",
		},
	}
}

// TokenError creates token-related errors with setup instructions
func TokenError(issue, tokenPath string, cause error) *OpnixError {
	suggestions := []string{
		"Set up your 1Password service account token:",
		"  1. Visit https://my.1password.com/developer-tools/infrastructure-secrets",
		"  2. Create a new service account",
		"  3. Copy the token and run: opnix token set",
		fmt.Sprintf("  4. Or manually create file: echo 'your-token' | sudo tee %s", tokenPath),
		fmt.Sprintf("  5. Set correct permissions: sudo chmod 640 %s", tokenPath),
	}

	return &OpnixError{
		Operation:   "Token access",
		Component:   "authentication",
		Issue:       issue,
		Context:     fmt.Sprintf("Token file: %s", tokenPath),
		Suggestions: suggestions,
		Cause:       cause,
	}
}

// Helper functions

func getDirPath(filePath string) string {
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash == -1 {
		return "."
	}
	if lastSlash == 0 {
		return "/"
	}
	return filePath[:lastSlash]
}

func getUserGroupCommand(entityType string) string {
	if entityType == "user" {
		return "useradd"
	}
	return "groupadd"
}

// Wrap provides a simple way to wrap existing errors with OpNix context
func Wrap(err error, operation, component string) error {
	if err == nil {
		return nil
	}

	return &OpnixError{
		Operation: operation,
		Component: component,
		Issue:     err.Error(),
		Cause:     err,
	}
}

// ServiceError creates errors for systemd service operations
func ServiceError(operation, serviceName, action string, cause error) *OpnixError {
	suggestions := []string{}

	// Add context-specific suggestions based on the action
	switch action {
	case "restart", "reload":
		suggestions = append(suggestions,
			fmt.Sprintf("Check service status: systemctl status %s", serviceName),
			fmt.Sprintf("Check service logs: journalctl -u %s -n 20", serviceName),
			"Verify service configuration is valid",
			fmt.Sprintf("Try manual restart: sudo systemctl restart %s", serviceName),
		)
	case "is-active":
		suggestions = append(suggestions,
			fmt.Sprintf("Check if service exists: systemctl cat %s", serviceName),
			fmt.Sprintf("Check service status: systemctl status %s", serviceName),
			"List all services: systemctl list-units --type=service",
		)
	case "cat":
		suggestions = append(suggestions,
			fmt.Sprintf("Check if service unit file exists: ls -la /etc/systemd/system/%s.service", serviceName),
			fmt.Sprintf("Check if service is installed: systemctl list-unit-files | grep %s", serviceName),
			"Reload systemd configuration: sudo systemctl daemon-reload",
		)
	}

	return &OpnixError{
		Operation:   operation,
		Component:   "systemd service",
		Issue:       fmt.Sprintf("Service operation '%s' failed for service '%s'", action, serviceName),
		Suggestions: suggestions,
		Cause:       cause,
	}
}

// WrapWithSuggestions wraps an error and adds suggestions
func WrapWithSuggestions(err error, operation, component string, suggestions []string) error {
	if err == nil {
		return nil
	}

	return &OpnixError{
		Operation:   operation,
		Component:   component,
		Issue:       err.Error(),
		Suggestions: suggestions,
		Cause:       err,
	}
}
