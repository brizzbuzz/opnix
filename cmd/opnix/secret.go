package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/brizzbuzz/opnix/internal/config"
	"github.com/brizzbuzz/opnix/internal/errors"
	"github.com/brizzbuzz/opnix/internal/onepass"
	"github.com/brizzbuzz/opnix/internal/secrets"
	"github.com/brizzbuzz/opnix/internal/validation"
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
	// Pre-flight checks
	if err := s.validatePrerequisites(); err != nil {
		return err
	}

	// Load configuration with improved error handling
	cfg, err := config.Load(s.configFile)
	if err != nil {
		// Error already has context from config.Load
		return err
	}

	log.Printf("Loaded configuration with %d secrets", len(cfg.Secrets))

	// Initialize 1Password client with validation
	client, err := onepass.NewClient(s.tokenFile)
	if err != nil {
		// Error already has context from onepass.NewClient
		return err
	}

	log.Printf("Initialized 1Password client successfully")

	// Process secrets with detailed progress
	processor := secrets.NewProcessor(client, s.outputDir)
	if err := processor.Process(cfg); err != nil {
		// Error already has context from processor.Process
		return err
	}

	log.Printf("Successfully processed all secrets to %s", s.outputDir)
	return nil
}

// validatePrerequisites performs pre-flight checks before processing
func (s *secretCommand) validatePrerequisites() error {
	// Check if config file exists
	if _, err := os.Stat(s.configFile); os.IsNotExist(err) {
		return errors.FileOperationError(
			"Checking configuration file",
			s.configFile,
			"Configuration file does not exist",
			err,
		)
	}

	// Check if output directory is writable
	if err := s.checkOutputDirectory(); err != nil {
		return err
	}

	// Validate token file (but don't fail if missing - let graceful handling work)
	validator := validation.NewValidator()
	if err := validator.ValidateTokenFile(s.tokenFile); err != nil {
		// For token errors, log a warning but don't fail
		fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
		fmt.Fprintf(os.Stderr, "INFO: Continuing with existing secrets if available\n")
	}

	return nil
}

// checkOutputDirectory ensures the output directory is accessible
func (s *secretCommand) checkOutputDirectory() error {
	// Try to create the directory if it doesn't exist
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return errors.FileOperationError(
			"Creating output directory",
			s.outputDir,
			"Cannot create or access output directory",
			err,
		)
	}

	// Test write permissions by creating a temporary file
	testFile := fmt.Sprintf("%s/.opnix-test", s.outputDir)
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		return errors.FileOperationError(
			"Testing output directory permissions",
			s.outputDir,
			"Output directory is not writable",
			err,
		)
	}

	// Clean up test file
	_ = os.Remove(testFile) // Ignore error - cleanup is best effort

	return nil
}
