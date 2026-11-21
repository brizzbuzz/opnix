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
	"github.com/brizzbuzz/opnix/internal/systemd"
	"github.com/brizzbuzz/opnix/internal/validation"
)

const defaultTokenPath = "/etc/opnix-token"

type secretProcessor interface {
	Process(*config.Config) (*secrets.ProcessResult, error)
}

type systemdManager interface {
	ProcessOutputChanges([]systemd.Output) error
}

type secretCommand struct {
	fs         *flag.FlagSet
	configFile string
	outputDir  string
	tokenFile  string

	loadConfig       func(string) (*config.Config, error)
	newClient        func(string) (secrets.SecretClient, error)
	processorFactory func(secrets.SecretClient, string) secretProcessor
	systemdFactory   func(config.SystemdIntegration) (systemdManager, error)
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

	sc.loadConfig = config.Load
	sc.newClient = func(path string) (secrets.SecretClient, error) {
		return onepass.NewClient(path)
	}
	sc.processorFactory = func(client secrets.SecretClient, outputDir string) secretProcessor {
		return secrets.NewProcessor(client, outputDir)
	}
	sc.systemdFactory = func(cfg config.SystemdIntegration) (systemdManager, error) {
		return systemd.NewManager(cfg)
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
	cfg, err := s.loadConfig(s.configFile)
	if err != nil {
		// Error already has context from config.Load
		return err
	}

	log.Printf("Loaded configuration with %d secrets and %d templates", len(cfg.Secrets), len(cfg.Templates))

	// Initialize 1Password client with validation
	client, err := s.newClient(s.tokenFile)
	if err != nil {
		// Error already has context from onepass.NewClient
		return err
	}

	log.Printf("Initialized 1Password client successfully")

	// Process secrets with detailed progress
	processor := s.processorFactory(client, s.outputDir)
	result, err := processor.Process(cfg)
	if err != nil {
		// Error already has context from processor.Process
		return err
	}

	log.Printf("Successfully processed %d secrets and %d templates to %s", result.ProcessedCount, len(result.Templates), s.outputDir)

	// Process systemd integration if enabled
	if cfg.SystemdIntegration.Enable {
		log.Printf("Processing systemd integration for %d services", len(cfg.SystemdIntegration.Services))

		systemdManager, err := s.systemdFactory(cfg.SystemdIntegration)
		if err != nil {
			return errors.WrapWithSuggestions(
				err,
				"Initializing systemd integration",
				"systemd integration",
				[]string{
					"Ensure systemctl is available in PATH",
					"Check if running on a systemd-enabled system",
					"Consider disabling systemd integration if not needed",
				},
			)
		}

		outputs := systemd.SecretOutputs(cfg.Secrets, result.SecretPaths)
		for _, template := range result.Templates {
			outputs = append(outputs, systemd.Output{Name: "template:" + template.Name, Path: template.Path, Services: template.Services})
		}
		if err := systemdManager.ProcessOutputChanges(outputs); err != nil {
			return errors.WrapWithSuggestions(
				err,
				"Processing systemd service changes",
				"systemd integration",
				[]string{
					"Check if specified services exist and are accessible",
					"Verify systemctl permissions",
					"Review systemd integration configuration",
					"Check systemd service logs: journalctl -u <service-name>",
				},
			)
		}

		log.Printf("Successfully processed systemd integration")
	}

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

	// Validate token file (but don't fail if missing - let graceful handling work)
	validator := validation.NewValidator()
	if err := validator.ValidateTokenFile(s.tokenFile); err != nil {
		// For token errors, log a warning but don't fail
		fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
		fmt.Fprintf(os.Stderr, "INFO: Continuing with existing secrets if available\n")
	}

	return nil
}
