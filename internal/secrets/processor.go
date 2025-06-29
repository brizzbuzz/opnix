package secrets

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/brizzbuzz/opnix/internal/config"
	"github.com/brizzbuzz/opnix/internal/errors"
)

type SecretClient interface {
	ResolveSecret(reference string) (string, error)
}

type Processor struct {
	client       SecretClient
	outputDir    string
	pathTemplate string
	defaults     map[string]string
}

func NewProcessor(client SecretClient, outputDir string) *Processor {
	return &Processor{
		client:    client,
		outputDir: outputDir,
	}
}

func NewProcessorWithConfig(client SecretClient, outputDir, pathTemplate string, defaults map[string]string) *Processor {
	return &Processor{
		client:       client,
		outputDir:    outputDir,
		pathTemplate: pathTemplate,
		defaults:     defaults,
	}
}

func (p *Processor) Process(cfg *config.Config) error {
	// Update processor with config-level settings
	if cfg.PathTemplate != "" {
		p.pathTemplate = cfg.PathTemplate
	}
	if len(cfg.Defaults) > 0 {
		p.defaults = cfg.Defaults
	}

	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return errors.FileOperationError(
			"Creating output directory",
			p.outputDir,
			"Failed to create output directory",
			err,
		)
	}

	for i, secret := range cfg.Secrets {
		secretName := fmt.Sprintf("secret[%d]:%s", i, secret.Path)
		if err := p.processSecret(secret, secretName); err != nil {
			return errors.WrapWithSuggestions(
				err,
				fmt.Sprintf("Processing %s", secretName),
				"secret processing",
				[]string{
					"Check the secret configuration for errors",
					"Verify 1Password reference is correct",
					"Ensure target directory permissions are correct",
				},
			)
		}
	}

	return nil
}

func (p *Processor) processSecret(secret config.Secret, secretName string) error {
	// Resolve the secret value from 1Password
	value, err := p.client.ResolveSecret(secret.Reference)
	if err != nil {
		return errors.OnePasswordError(
			fmt.Sprintf("Resolving secret %s", secretName),
			fmt.Sprintf("Failed to resolve 1Password reference: %s", secret.Reference),
			err,
		)
	}

	// Determine output path with enhanced path management
	outputPath, err := p.resolveSecretPathWithTemplate(secret, secretName)
	if err != nil {
		return err
	}

	// Validate the resolved path for security
	if err := p.validateSecretPath(outputPath, secretName); err != nil {
		return err
	}

	// Create parent directory if needed (validation already ensured it's writable)
	parentDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return errors.FileOperationError(
			fmt.Sprintf("Creating parent directory for %s", secretName),
			parentDir,
			"Failed to create parent directory",
			err,
		)
	}

	// Parse file permissions
	mode := secret.Mode
	if mode == "" {
		mode = "0600" // Default secure permissions
	}
	fileMode, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return errors.ValidationError(
			fmt.Sprintf("Parsing file mode for %s", secretName),
			"mode",
			mode,
			"3-4 digit octal number (e.g., 0600, 0644)",
		)
	}

	// Write file with specified permissions
	if err := os.WriteFile(outputPath, []byte(value), os.FileMode(fileMode)); err != nil {
		return errors.FileOperationError(
			fmt.Sprintf("Writing secret file for %s", secretName),
			outputPath,
			"Failed to write secret to file",
			err,
		)
	}

	// Set ownership if specified
	if secret.Owner != "" || secret.Group != "" {
		if err := p.setOwnership(outputPath, secret.Owner, secret.Group, secretName); err != nil {
			return err
		}
	}

	// Create symlinks if specified
	if err := p.createSymlinks(outputPath, secret.Symlinks, secretName); err != nil {
		return err
	}

	return nil
}

// setOwnership sets the file ownership based on owner and group names
func (p *Processor) setOwnership(path, owner, group, secretName string) error {
	var uid, gid = -1, -1

	// Resolve owner to UID
	if owner != "" {
		if owner == "root" {
			uid = 0
		} else {
			u, err := user.Lookup(owner)
			if err != nil {
				// Get available users for suggestions
				availableUsers := p.getAvailableUsers()
				return errors.UserGroupError(
					fmt.Sprintf("Setting ownership for %s", secretName),
					owner,
					"user",
					availableUsers,
				)
			}
			parsedUID, err := strconv.Atoi(u.Uid)
			if err != nil {
				return errors.ConfigError(
					fmt.Sprintf("Parsing UID for user %s", owner),
					fmt.Sprintf("Invalid UID format: %s", u.Uid),
					err,
				)
			}
			uid = parsedUID
		}
	}

	// Resolve group to GID
	if group != "" {
		if group == "root" {
			gid = 0
		} else {
			g, err := user.LookupGroup(group)
			if err != nil {
				// Get available groups for suggestions
				availableGroups := p.getAvailableGroups()
				return errors.UserGroupError(
					fmt.Sprintf("Setting ownership for %s", secretName),
					group,
					"group",
					availableGroups,
				)
			}
			parsedGID, err := strconv.Atoi(g.Gid)
			if err != nil {
				return errors.ConfigError(
					fmt.Sprintf("Parsing GID for group %s", group),
					fmt.Sprintf("Invalid GID format: %s", g.Gid),
					err,
				)
			}
			gid = parsedGID
		}
	}

	// Set ownership
	if uid != -1 || gid != -1 {
		if err := syscall.Chown(path, uid, gid); err != nil {
			return errors.FileOperationError(
				fmt.Sprintf("Setting ownership for %s", secretName),
				path,
				fmt.Sprintf("Failed to change ownership to %s:%s", owner, group),
				err,
			)
		}
	}

	return nil
}

// getAvailableUsers returns a list of common system users for error suggestions
func (p *Processor) getAvailableUsers() []string {
	users := []string{"root"}

	// Try to get some common service users
	commonUsers := []string{"nginx", "apache", "www-data", "caddy", "postgres", "mysql", "redis", "docker"}

	for _, username := range commonUsers {
		if _, err := user.Lookup(username); err == nil {
			users = append(users, username)
		}
	}

	return users
}

// getAvailableGroups returns a list of common system groups for error suggestions
func (p *Processor) getAvailableGroups() []string {
	groups := []string{"root"}

	// Try to get some common service groups
	commonGroups := []string{"nginx", "apache", "www-data", "caddy", "postgres", "mysql", "redis", "docker", "ssl-cert"}

	for _, groupname := range commonGroups {
		if _, err := user.LookupGroup(groupname); err == nil {
			groups = append(groups, groupname)
		}
	}

	return groups
}

// resolveSecretPath resolves the final path for a secret based on custom path logic (legacy)
func (p *Processor) resolveSecretPath(secretPath, secretName string) string {
	// If path is absolute, use it directly (custom path management)
	if filepath.IsAbs(secretPath) {
		return secretPath
	}

	// For relative paths, combine with outputDir (backward compatibility)
	return filepath.Join(p.outputDir, secretPath)
}

// resolveSecretPathWithTemplate resolves the final path for a secret with template support
func (p *Processor) resolveSecretPathWithTemplate(secret config.Secret, secretName string) (string, error) {
	// If path is explicitly set, use it with variable substitution
	if secret.Path != "" {
		resolvedPath, err := p.substituteVariables(secret.Path, secret.Variables, secretName)
		if err != nil {
			return "", err
		}
		return p.resolveSecretPath(resolvedPath, secretName), nil
	}

	// If no path template is configured, return error
	if p.pathTemplate == "" {
		return "", errors.ConfigError(
			fmt.Sprintf("Resolving path for %s", secretName),
			"No path specified and no pathTemplate configured",
			nil,
		)
	}

	// Use template with variable substitution
	resolvedPath, err := p.substituteVariables(p.pathTemplate, secret.Variables, secretName)
	if err != nil {
		return "", err
	}

	return p.resolveSecretPath(resolvedPath, secretName), nil
}

// validateSecretPath validates that the resolved path is secure and accessible
func (p *Processor) validateSecretPath(resolvedPath, secretName string) error {
	// Check for path traversal attempts
	if strings.Contains(resolvedPath, "..") {
		return errors.FileOperationError(
			fmt.Sprintf("Validating path for %s", secretName),
			resolvedPath,
			"Path contains path traversal attempt (..)",
			nil,
		)
	}

	// Check for potentially dangerous system locations
	dangerousPaths := []string{
		"/bin", "/sbin", "/usr/bin", "/usr/sbin",
		"/boot", "/dev", "/proc", "/sys",
		"/etc/passwd", "/etc/shadow", "/etc/group",
	}

	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(resolvedPath, dangerous) {
			return errors.FileOperationError(
				fmt.Sprintf("Validating path for %s", secretName),
				resolvedPath,
				fmt.Sprintf("Path targets potentially dangerous system location: %s", dangerous),
				nil,
			)
		}
	}

	// Check if parent directory is writable (or can be created)
	parentDir := filepath.Dir(resolvedPath)
	if err := p.ensureDirectoryWritable(parentDir); err != nil {
		return errors.FileOperationError(
			fmt.Sprintf("Validating parent directory for %s", secretName),
			parentDir,
			"Parent directory is not writable or cannot be created",
			err,
		)
	}

	return nil
}

// ensureDirectoryWritable ensures a directory exists and is writable
func (p *Processor) ensureDirectoryWritable(dir string) error {
	// Try to create the directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Test write permissions by creating a temporary file
	testFile := filepath.Join(dir, ".opnix-write-test")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		return err
	}

	// Clean up test file
	_ = os.Remove(testFile) // Ignore error - cleanup is best effort
	return nil
}

// createSymlinks creates symlinks for a secret file
func (p *Processor) createSymlinks(targetPath string, symlinks []string, secretName string) error {
	for i, symlinkPath := range symlinks {
		symlinkName := fmt.Sprintf("%s.symlinks[%d]", secretName, i)

		// Validate symlink path
		if err := p.validateSecretPath(symlinkPath, symlinkName); err != nil {
			return err
		}

		// Create parent directory for symlink if needed
		parentDir := filepath.Dir(symlinkPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return errors.FileOperationError(
				fmt.Sprintf("Creating parent directory for symlink %s", symlinkName),
				parentDir,
				"Failed to create parent directory for symlink",
				err,
			)
		}

		// Remove existing symlink or file if it exists
		if err := os.Remove(symlinkPath); err != nil && !os.IsNotExist(err) {
			return errors.FileOperationError(
				fmt.Sprintf("Removing existing symlink %s", symlinkName),
				symlinkPath,
				"Failed to remove existing symlink or file",
				err,
			)
		}

		// Create the symlink
		if err := os.Symlink(targetPath, symlinkPath); err != nil {
			return errors.FileOperationError(
				fmt.Sprintf("Creating symlink %s", symlinkName),
				symlinkPath,
				fmt.Sprintf("Failed to create symlink to %s", targetPath),
				err,
			)
		}
	}

	return nil
}

// substituteVariables replaces template variables in a path
func (p *Processor) substituteVariables(template string, variables map[string]string, secretName string) (string, error) {
	result := template

	// Create combined variable map (secret variables override defaults)
	allVars := make(map[string]string)
	for k, v := range p.defaults {
		allVars[k] = v
	}
	for k, v := range variables {
		allVars[k] = v
	}

	// Find all template variables {varname}
	for strings.Contains(result, "{") && strings.Contains(result, "}") {
		start := strings.Index(result, "{")
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		placeholder := result[start : end+1] // {varname}
		varName := result[start+1 : end]     // varname

		value, exists := allVars[varName]
		if !exists {
			return "", errors.ConfigError(
				fmt.Sprintf("Processing template variable for %s", secretName),
				fmt.Sprintf("Template variable '{%s}' not found in variables or defaults", varName),
				nil,
			)
		}

		// Validate variable value doesn't contain dangerous patterns
		if err := p.validateVariableValue(value, varName, secretName); err != nil {
			return "", err
		}

		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result, nil
}

// validateVariableValue validates that a template variable value is safe
func (p *Processor) validateVariableValue(value, varName, secretName string) error {
	if strings.Contains(value, "..") {
		return errors.ConfigError(
			fmt.Sprintf("Validating variable %s for %s", varName, secretName),
			fmt.Sprintf("Variable value '%s' contains path traversal attempt (..)", value),
			nil,
		)
	}

	return nil
}
