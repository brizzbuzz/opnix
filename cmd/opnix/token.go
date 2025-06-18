// cmd/opnix/token.go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const tokenFileMode = 0600

type tokenCommand struct {
	fs     *flag.FlagSet
	path   string
	action string
}

func newTokenCommand() *tokenCommand {
	tc := &tokenCommand{
		fs: flag.NewFlagSet("token", flag.ExitOnError),
	}

	tc.fs.StringVar(&tc.path, "path", defaultTokenPath, "Path to store the token file")

	tc.fs.Usage = func() {
		fmt.Fprintf(tc.fs.Output(), "Usage: opnix token <command> [options]\n\n")
		fmt.Fprintf(tc.fs.Output(), "Manage 1Password service account token\n\n")
		fmt.Fprintf(tc.fs.Output(), "Commands:\n")
		fmt.Fprintf(tc.fs.Output(), "  set     Set the service account token\n\n")
		fmt.Fprintf(tc.fs.Output(), "Options:\n")
		tc.fs.PrintDefaults()
	}

	return tc
}

func (t *tokenCommand) Name() string { return t.fs.Name() }

func (t *tokenCommand) Init(args []string) error {
	if err := t.fs.Parse(args); err != nil {
		return err
	}

	if t.fs.NArg() < 1 {
		t.fs.Usage()
		return fmt.Errorf("token subcommand required")
	}

	t.action = t.fs.Arg(0)
	return nil
}

func (t *tokenCommand) Run() error {
	switch t.action {
	case "set":
		return t.setToken()
	default:
		return fmt.Errorf("unknown token action: %s", t.action)
	}
}

// checkWritePermissions verifies we can write to the directory
func (t *tokenCommand) checkWritePermissions() error {
	dir := filepath.Dir(t.path)

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Try to create the directory
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create directory %s: %w", dir, err)
		}
	}

	// Test write permissions by attempting to create a temporary file
	tmpFile := filepath.Join(dir, ".opnix-write-test")
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("insufficient permissions to write to %s. Try running with sudo", dir)
		}
		return fmt.Errorf("cannot write to directory %s: %w", dir, err)
	}
	_ = f.Close()
	_ = os.Remove(tmpFile)

	return nil
}

func (t *tokenCommand) setToken() error {
	// Check permissions before prompting for input
	if err := t.checkWritePermissions(); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Please paste your 1Password service account token (press Enter when done):\n")

	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	// Trim whitespace and newlines
	tokenStr := strings.TrimSpace(token)
	if tokenStr == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Write token to file with secure permissions
	if err := os.WriteFile(t.path, []byte(tokenStr), tokenFileMode); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Token successfully stored at %s\n", t.path)
	return nil
}
