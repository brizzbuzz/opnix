package secrets

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/brizzbuzz/opnix/internal/config"
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
		return err
	}

	for _, secret := range cfg.Secrets {
		if err := p.processSecret(secret); err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) processSecret(secret config.Secret) error {
	value, err := p.client.ResolveSecret(secret.Reference)
	if err != nil {
		return err
	}

	outputPath := filepath.Join(p.outputDir, secret.Path)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	// Parse file permissions
	mode := secret.Mode
	if mode == "" {
		mode = "0600" // Default secure permissions
	}
	fileMode, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid file mode %s: %w", mode, err)
	}

	// Write file with specified permissions
	if err := os.WriteFile(outputPath, []byte(value), os.FileMode(fileMode)); err != nil {
		return fmt.Errorf("failed to write secret file: %w", err)
	}

	// Set ownership if specified
	if secret.Owner != "" || secret.Group != "" {
		if err := p.setOwnership(outputPath, secret.Owner, secret.Group); err != nil {
			return fmt.Errorf("failed to set ownership: %w", err)
		}
	}

	return nil
}

// setOwnership sets the file ownership based on owner and group names
func (p *Processor) setOwnership(path, owner, group string) error {
	var uid, gid int = -1, -1

	// Resolve owner to UID
	if owner != "" {
		if owner == "root" {
			uid = 0
		} else {
			u, err := user.Lookup(owner)
			if err != nil {
				return fmt.Errorf("user %s not found: %w", owner, err)
			}
			parsedUID, err := strconv.Atoi(u.Uid)
			if err != nil {
				return fmt.Errorf("invalid UID for user %s: %w", owner, err)
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
				return fmt.Errorf("group %s not found: %w", group, err)
			}
			parsedGID, err := strconv.Atoi(g.Gid)
			if err != nil {
				return fmt.Errorf("invalid GID for group %s: %w", group, err)
			}
			gid = parsedGID
		}
	}

	// Set ownership
	if uid != -1 || gid != -1 {
		if err := syscall.Chown(path, uid, gid); err != nil {
			return fmt.Errorf("chown failed: %w", err)
		}
	}

	return nil
}
