package validation

import (
	"fmt"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"strings"

	"github.com/brizzbuzz/opnix/internal/errors"
)

// Validator provides comprehensive validation with helpful error messages
type Validator struct{}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// Secret represents a secret for validation
type SecretData struct {
	Path         string
	Reference    string
	Owner        string
	Group        string
	Mode         string
	Symlinks     []string
	Variables    map[string]string
	Services     interface{} // Can be []string or map[string]ServiceConfig
	PathTemplate string
	Defaults     map[string]string
}

// ValidateConfigStruct validates a config with slice of SecretData
func (v *Validator) ValidateConfigStruct(secrets []SecretData) error {
	if len(secrets) == 0 {
		return errors.ConfigError(
			"Configuration validation",
			"No secrets defined in configuration",
			nil,
		)
	}

	// Track seen paths to detect duplicates
	seenPaths := make(map[string]string)

	for i, secret := range secrets {
		secretName := fmt.Sprintf("secret[%d]", i)
		if err := v.validateSecret(secret, secretName, seenPaths); err != nil {
			return err
		}
	}

	return nil
}

// validateSecret validates individual secret configuration
func (v *Validator) validateSecret(secret SecretData, secretName string, seenPaths map[string]string) error {
	// Validate reference
	if err := v.validateReference(secret.Reference, secretName); err != nil {
		return err
	}

	// Validate path and resolve final path
	finalPath, err := v.resolvePath(secret.Path, secret.PathTemplate, secret.Variables, secret.Defaults, secretName)
	if err != nil {
		return err
	}

	if err := v.validatePath(finalPath, secretName, seenPaths); err != nil {
		return err
	}

	// Validate symlinks
	if err := v.validateSymlinks(secret.Symlinks, secretName, seenPaths); err != nil {
		return err
	}

	// Validate ownership
	if err := v.validateOwnership(secret.Owner, secret.Group, secretName); err != nil {
		return err
	}

	// Validate permissions
	if err := v.validateMode(secret.Mode, secretName); err != nil {
		return err
	}

	return nil
}

// resolvePath resolves the final path using templates and variables
func (v *Validator) resolvePath(path, pathTemplate string, variables, defaults map[string]string, secretName string) (string, error) {
	// If path is explicitly set, use it directly
	if path != "" {
		return v.substituteVariables(path, variables, defaults, secretName)
	}

	// If no path template is set, return error
	if pathTemplate == "" {
		return "", errors.ConfigValidationError(
			fmt.Sprintf("%s.path", secretName),
			"<empty>",
			"Path cannot be empty and no pathTemplate is configured",
			[]string{
				"Specify a path directly in the secret configuration",
				"Or configure a pathTemplate at the config level",
				"Example template: /etc/secrets/{service}/{name}",
			},
		)
	}

	// Use template to generate path
	return v.substituteVariables(pathTemplate, variables, defaults, secretName)
}

// substituteVariables replaces template variables in a path
func (v *Validator) substituteVariables(template string, variables, defaults map[string]string, secretName string) (string, error) {
	result := template

	// Create combined variable map (variables override defaults)
	allVars := make(map[string]string)
	for k, v := range defaults {
		allVars[k] = v
	}
	for k, v := range variables {
		allVars[k] = v
	}

	// Find all template variables {varname}
	varPattern := regexp.MustCompile(`\{([^}]+)\}`)
	matches := varPattern.FindAllStringSubmatch(template, -1)

	for _, match := range matches {
		placeholder := match[0] // {varname}
		varName := match[1]     // varname

		value, exists := allVars[varName]
		if !exists {
			availableVars := make([]string, 0, len(allVars))
			for k := range allVars {
				availableVars = append(availableVars, k)
			}

			return "", errors.ConfigValidationError(
				fmt.Sprintf("%s template variable", secretName),
				varName,
				fmt.Sprintf("Template variable '{%s}' not found in variables or defaults", varName),
				append([]string{
					fmt.Sprintf("Add '%s' to the secret's variables", varName),
					fmt.Sprintf("Or add '%s' to config defaults", varName),
					"Template: " + template,
				}, fmt.Sprintf("Available variables: %v", availableVars)),
			)
		}

		// Validate variable value doesn't contain path traversal or dangerous patterns
		if err := v.validateVariableValue(value, varName, secretName); err != nil {
			return "", err
		}

		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result, nil
}

// validateVariableValue validates that a template variable value is safe
func (v *Validator) validateVariableValue(value, varName, secretName string) error {
	if strings.Contains(value, "..") {
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.variables.%s", secretName, varName),
			value,
			"Variable value contains path traversal attempt (..)",
			[]string{
				"Remove '..' from the variable value",
				"Use clean directory/file names without path traversal",
			},
		)
	}

	// Check for potentially dangerous characters
	dangerousChars := []string{";", "&", "|", "$", "`", "(", ")", "<", ">"}
	for _, char := range dangerousChars {
		if strings.Contains(value, char) {
			return errors.ConfigValidationError(
				fmt.Sprintf("%s.variables.%s", secretName, varName),
				value,
				fmt.Sprintf("Variable value contains potentially dangerous character: %s", char),
				[]string{
					"Use only alphanumeric characters, hyphens, and underscores in variable values",
					"Avoid shell metacharacters for security",
				},
			)
		}
	}

	return nil
}

// validateSymlinks validates symlink paths and checks for conflicts
func (v *Validator) validateSymlinks(symlinks []string, secretName string, seenPaths map[string]string) error {
	for i, symlink := range symlinks {
		symlinkName := fmt.Sprintf("%s.symlinks[%d]", secretName, i)

		if symlink == "" {
			return errors.ConfigValidationError(
				symlinkName,
				"<empty>",
				"Symlink path cannot be empty",
				[]string{
					"Specify a valid symlink path",
					"Remove empty symlink entries from the array",
				},
			)
		}

		// Check for path traversal
		if strings.Contains(symlink, "..") {
			return errors.ConfigValidationError(
				symlinkName,
				symlink,
				"Symlink path contains path traversal attempt (..)",
				[]string{
					"Remove '..' from the symlink path",
					"Use absolute paths for symlinks outside the base directory",
				},
			)
		}

		// Validate absolute symlink paths for security
		if strings.HasPrefix(symlink, "/") {
			if err := v.validateAbsolutePath(symlink, symlinkName); err != nil {
				return err
			}
		}

		// Check for duplicate symlink paths
		if existingSecret, exists := seenPaths[symlink]; exists {
			return errors.ConfigValidationError(
				symlinkName,
				symlink,
				fmt.Sprintf("Duplicate symlink path (conflicts with %s)", existingSecret),
				[]string{
					"Each symlink must have a unique path",
					"Change the symlink path to something unique",
				},
			)
		}

		seenPaths[symlink] = fmt.Sprintf("%s (symlink)", secretName)
	}

	return nil
}

// validateReference validates 1Password reference format
func (v *Validator) validateReference(reference, secretName string) error {
	if reference == "" {
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.reference", secretName),
			"<empty>",
			"Reference cannot be empty",
			[]string{
				"Add a valid 1Password reference: op://Vault/Item/field",
				"Example: op://Homelab/Database/password",
				"Check 1Password documentation for reference format",
			},
		)
	}

	// Extract and validate components first
	if !strings.HasPrefix(reference, "op://") {
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.reference", secretName),
			reference,
			"Invalid 1Password reference format",
			[]string{
				"Use format: op://Vault/Item/field or op://Vault/Item/Section/field",
				"Example: op://Homelab/Database/password",
				"Example with section: op://Homelab/Cloudflare/rgbr.ink/cert",
				"Ensure vault, item, and field names don't contain forward slashes",
				"Check the reference in 1Password web interface",
			},
		)
	}

	parts := strings.Split(strings.TrimPrefix(reference, "op://"), "/")
	if len(parts) < 3 {
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.reference", secretName),
			reference,
			"Reference must have at least 3 parts: vault/item/field",
			[]string{
				"Verify the reference format: op://Vault/Item/field",
				"Or with sections: op://Vault/Item/Section/field",
				"Check for missing forward slashes",
			},
		)
	}

	vault, item := parts[0], parts[1]
	field := parts[len(parts)-1] // Field is always the last part

	if vault == "" {
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.reference", secretName),
			reference,
			"Vault name cannot be empty",
			[]string{
				"Specify a valid vault name in the reference",
				"List available vaults: op vault list",
			},
		)
	}

	if item == "" {
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.reference", secretName),
			reference,
			"Item name cannot be empty",
			[]string{
				"Specify a valid item name in the reference",
				fmt.Sprintf("List items in vault: op item list --vault '%s'", vault),
			},
		)
	}

	if field == "" {
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.reference", secretName),
			reference,
			"Field name cannot be empty",
			[]string{
				"Specify a valid field name in the reference",
				fmt.Sprintf("View item details: op item get '%s' --vault '%s'", item, vault),
				"Common field names: password, credential, token, key",
			},
		)
	}

	return nil
}

// validatePath validates secret path and checks for duplicates
func (v *Validator) validatePath(path, secretName string, seenPaths map[string]string) error {
	if path == "" {
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.path", secretName),
			"<empty>",
			"Path cannot be empty",
			[]string{
				"Specify a valid file path for the secret",
				"Use relative path: database/password",
				"Or absolute path: /etc/ssl/certs/app.pem",
			},
		)
	}

	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.path", secretName),
			path,
			"Path traversal detected (contains '..')",
			[]string{
				"Remove '..' from the path",
				"Use absolute paths if you need to place files outside the base directory",
				"Example: /etc/ssl/certs/cert.pem instead of ../../../etc/ssl/certs/cert.pem",
			},
		)
	}

	// Check for duplicate paths
	if existingSecret, exists := seenPaths[path]; exists {
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.path", secretName),
			path,
			fmt.Sprintf("Duplicate path (already used by %s)", existingSecret),
			[]string{
				"Each secret must have a unique path",
				"Change the path to something unique",
				fmt.Sprintf("Current conflicting path: %s", path),
			},
		)
	}

	seenPaths[path] = secretName

	// Validate absolute path security
	if strings.HasPrefix(path, "/") {
		if err := v.validateAbsolutePath(path, secretName); err != nil {
			return err
		}
	}

	return nil
}

// validateAbsolutePath validates absolute paths for security
func (v *Validator) validateAbsolutePath(path, secretName string) error {
	// Check for potentially dangerous locations
	dangerousPaths := []string{
		"/bin", "/sbin", "/usr/bin", "/usr/sbin",
		"/boot", "/dev", "/proc", "/sys",
		"/etc/passwd", "/etc/shadow", "/etc/group",
	}

	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(path, dangerous) {
			return errors.ConfigValidationError(
				fmt.Sprintf("%s.path", secretName),
				path,
				fmt.Sprintf("Path starts with potentially dangerous location: %s", dangerous),
				[]string{
					"Avoid placing secrets in system directories",
					"Use /etc/secrets/, /var/lib/opnix/secrets/, or /run/secrets/ instead",
					"Consider using relative paths under the configured output directory",
				},
			)
		}
	}

	return nil
}

// validateOwnership validates user and group settings
func (v *Validator) validateOwnership(owner, group, secretName string) error {
	if owner != "" {
		if err := v.validateUser(owner, secretName); err != nil {
			return err
		}
	}

	if group != "" {
		if err := v.validateGroup(group, secretName); err != nil {
			return err
		}
	}

	return nil
}

// validateUser validates that a user exists
func (v *Validator) validateUser(username, secretName string) error {
	if username == "root" {
		return nil // root always exists
	}

	_, err := user.Lookup(username)
	if err != nil {
		// Get list of available users for suggestions
		availableUsers := v.getAvailableUsers()

		return errors.UserGroupError(
			fmt.Sprintf("Validating %s.owner", secretName),
			username,
			"user",
			availableUsers,
		)
	}

	return nil
}

// validateGroup validates that a group exists
func (v *Validator) validateGroup(groupname, secretName string) error {
	if groupname == "root" {
		return nil // root group always exists
	}

	_, err := user.LookupGroup(groupname)
	if err != nil {
		// Get list of available groups for suggestions
		availableGroups := v.getAvailableGroups()

		return errors.UserGroupError(
			fmt.Sprintf("Validating %s.group", secretName),
			groupname,
			"group",
			availableGroups,
		)
	}

	return nil
}

// validateMode validates file permission mode
func (v *Validator) validateMode(mode, secretName string) error {
	if mode == "" {
		return nil // Empty mode is ok, will use default
	}

	// Check if it's a valid octal string
	modePattern := regexp.MustCompile(`^[0-7]{3,4}$`)
	if !modePattern.MatchString(mode) {
		return errors.ValidationError(
			fmt.Sprintf("Validating %s.mode", secretName),
			"mode",
			mode,
			"3-4 digit octal number (e.g., 0600, 0644, 0755)",
		)
	}

	// Parse the mode to ensure it's valid
	_, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return errors.ValidationError(
			fmt.Sprintf("Validating %s.mode", secretName),
			"mode",
			mode,
			"valid octal number",
		)
	}

	// Security check: warn about overly permissive modes
	if err := v.validateModeSecurity(mode, secretName); err != nil {
		return err
	}

	return nil
}

// validateModeSecurity checks for potentially insecure file modes
func (v *Validator) validateModeSecurity(mode, secretName string) error {
	modeInt, _ := strconv.ParseUint(mode, 8, 32)

	// Check for world-writable secrets (always an error)
	if modeInt&0002 != 0 { // Others can write
		return errors.ConfigValidationError(
			fmt.Sprintf("%s.mode", secretName),
			mode,
			"Mode allows world write access (others can modify the secret)",
			[]string{
				"Remove write permission for others",
				"This is a serious security risk",
				"Use modes like 0600, 0640, or 0644 instead",
			},
		)
	}

	// Note: We allow world-readable modes like 0644 for certain use cases (certificates, etc.)
	// but could add a warning in the future

	return nil
}

// getAvailableUsers returns a list of available system users
func (v *Validator) getAvailableUsers() []string {
	users := []string{"root"} // Always include root

	// Try to read /etc/passwd for more users
	if content, err := os.ReadFile("/etc/passwd"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.Split(line, ":")
			if len(parts) > 0 && parts[0] != "root" {
				// Add common service users
				username := parts[0]
				if isServiceUser(username) {
					users = append(users, username)
				}
			}
		}
	}

	return users[:min(len(users), 10)] // Limit to 10 suggestions
}

// getAvailableGroups returns a list of available system groups
func (v *Validator) getAvailableGroups() []string {
	groups := []string{"root"} // Always include root

	// Try to read /etc/group for more groups
	if content, err := os.ReadFile("/etc/group"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.Split(line, ":")
			if len(parts) > 0 && parts[0] != "root" {
				groupname := parts[0]
				if isServiceGroup(groupname) {
					groups = append(groups, groupname)
				}
			}
		}
	}

	return groups[:min(len(groups), 10)] // Limit to 10 suggestions
}

// isServiceUser checks if a username looks like a service user
func isServiceUser(username string) bool {
	serviceUsers := []string{
		"nginx", "apache", "www-data", "caddy",
		"postgres", "mysql", "redis",
		"docker", "systemd", "nobody",
	}

	for _, service := range serviceUsers {
		if username == service {
			return true
		}
	}

	return false
}

// isServiceGroup checks if a groupname looks like a service group
func isServiceGroup(groupname string) bool {
	serviceGroups := []string{
		"nginx", "apache", "www-data", "caddy",
		"postgres", "mysql", "redis",
		"docker", "systemd", "nobody", "ssl-cert",
	}

	for _, service := range serviceGroups {
		if groupname == service {
			return true
		}
	}

	return false
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ValidateTokenFile validates the token file exists and has correct permissions
func (v *Validator) ValidateTokenFile(tokenPath string) error {
	// Check if file exists
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		return errors.TokenError(
			fmt.Sprintf("Token file does not exist: %s", tokenPath),
			tokenPath,
			err,
		)
	}

	// Check if file is readable
	if _, err := os.ReadFile(tokenPath); err != nil {
		return errors.TokenError(
			fmt.Sprintf("Cannot read token file: %s", err.Error()),
			tokenPath,
			err,
		)
	}

	// Check if file is empty
	content, err := os.ReadFile(tokenPath)
	if err != nil {
		return errors.TokenError(
			fmt.Sprintf("Failed to read token file: %s", err.Error()),
			tokenPath,
			err,
		)
	}

	if len(strings.TrimSpace(string(content))) == 0 {
		return errors.TokenError(
			"Token file is empty",
			tokenPath,
			nil,
		)
	}

	return nil
}
