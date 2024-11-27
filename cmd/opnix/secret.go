package main

import (
	"flag"
	"fmt"

	"github.com/brizzbuzz/opnix/internal/config"
	"github.com/brizzbuzz/opnix/internal/onepass"
	"github.com/brizzbuzz/opnix/internal/secrets"
)

const defaultTokenPath = "/etc/opnix-token"

type secretCommand struct {
	fs         *flag.FlagSet
	configFile string
	outputDir  string
	tokenFile  string
}

func newSecretCommand() *secretCommand {
	sc := &secretCommand{
		fs: flag.NewFlagSet("secret", flag.ExitOnError),
	}

	sc.fs.StringVar(&sc.configFile, "config", "secrets.json", "Path to secrets configuration file")
	sc.fs.StringVar(&sc.outputDir, "output", "secrets", "Directory to store retrieved secrets")
	sc.fs.StringVar(&sc.tokenFile, "token-file", defaultTokenPath, "Path to file containing 1Password service account token")

	sc.fs.Usage = func() {
		fmt.Fprintf(sc.fs.Output(), "Usage: opnix secret [options]\n\n")
		fmt.Fprintf(sc.fs.Output(), "Retrieve and manage secrets from 1Password\n\n")
		fmt.Fprintf(sc.fs.Output(), "Options:\n")
		sc.fs.PrintDefaults()
	}

	return sc
}

func (s *secretCommand) Name() string { return s.fs.Name() }

func (s *secretCommand) Init(args []string) error {
	return s.fs.Parse(args)
}

func (s *secretCommand) Run() error {
	// Load configuration
	cfg, err := config.Load(s.configFile)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	// Initialize 1Password client
	client, err := onepass.NewClient(s.tokenFile)
	if err != nil {
		return fmt.Errorf("error creating 1Password client: %w", err)
	}

	// Process secrets
	processor := secrets.NewProcessor(client, s.outputDir)
	if err := processor.Process(cfg); err != nil {
		return fmt.Errorf("error processing secrets: %w", err)
	}

	return nil
}
