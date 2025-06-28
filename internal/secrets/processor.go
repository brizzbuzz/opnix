package secrets

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/brizzbuzz/opnix/internal/config"
	"github.com/brizzbuzz/opnix/internal/errors"
)

type SecretClient interface {
	ResolveSecret(reference string) (string, error)
}

type Processor struct {
	client    SecretClient
	outputDir string
}

func NewProcessor(client SecretClient, outputDir string) *Processor {
	return &Processor{
		client:    client,
		outputDir: outputDir,
	}
}

func (p *Processor) Process(cfg *config.Config) error {
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

	// Determine output path
	outputPath := filepath.Join(p.outputDir, secret.Path)

	// Create parent directory if needed
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

	return nil
}

// setOwnership sets the file ownership based on owner and group names
func (p *Processor) setOwnership(path, owner, group, secretName string) error {
	var uid, gid int = -1, -1

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
