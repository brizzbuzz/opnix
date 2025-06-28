package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/brizzbuzz/opnix/internal/errors"
)

type command interface {
	Name() string
	Init([]string) error
	Run() error
}

func main() {
	cmds := []command{
		newSecretCommand(),
		newTokenCommand(),
	}

	if len(os.Args) < 2 {
		printUsage(cmds)
		os.Exit(1)
	}

	subcommand := os.Args[1]

	for _, cmd := range cmds {
		if cmd.Name() == subcommand {
			if err := cmd.Init(os.Args[2:]); err != nil {
				handleError(fmt.Errorf("failed to initialize %s: %w", cmd.Name(), err))
				os.Exit(1)
			}
			if err := cmd.Run(); err != nil {
				handleError(err)
				os.Exit(1)
			}
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcommand)
	printUsage(cmds)
	os.Exit(1)
}

func printUsage(cmds []command) {
	fmt.Fprintf(os.Stderr, "Usage: opnix <command> [options]\n\n")
	fmt.Fprintf(os.Stderr, "Available commands:\n")
	fmt.Fprintf(os.Stderr, "  secret    Manage and retrieve secrets from 1Password\n")
	fmt.Fprintf(os.Stderr, "  token     Manage the 1Password service account token\n\n")
	fmt.Fprintf(os.Stderr, "Use 'opnix <command> -h' for command-specific help\n")
}

// handleError provides user-friendly error output
func handleError(err error) {
	if err == nil {
		return
	}

	// Check if it's an OpnixError with structured information
	if opnixErr, ok := err.(*errors.OpnixError); ok {
		// Print structured error with full context
		fmt.Fprintf(os.Stderr, "%s\n", opnixErr.Error())
	} else {
		// Handle regular errors with some formatting
		errMsg := err.Error()

		// Add some context for common error patterns
		if strings.Contains(errMsg, "no such file or directory") {
			fmt.Fprintf(os.Stderr, "ERROR: File not found\n")
			fmt.Fprintf(os.Stderr, "  %s\n", errMsg)
			fmt.Fprintf(os.Stderr, "\n  Suggestions:\n")
			fmt.Fprintf(os.Stderr, "  1. Check the file path is correct\n")
			fmt.Fprintf(os.Stderr, "  2. Verify the file exists: ls -la <path>\n")
		} else if strings.Contains(errMsg, "permission denied") {
			fmt.Fprintf(os.Stderr, "ERROR: Permission denied\n")
			fmt.Fprintf(os.Stderr, "  %s\n", errMsg)
			fmt.Fprintf(os.Stderr, "\n  Suggestions:\n")
			fmt.Fprintf(os.Stderr, "  1. Check file/directory permissions\n")
			fmt.Fprintf(os.Stderr, "  2. Run with appropriate privileges if needed\n")
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)
		}
	}
}
