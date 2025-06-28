package secrets

import (
	"os"
	"path/filepath"

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

	return os.WriteFile(outputPath, []byte(value), 0600)
}
