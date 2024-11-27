package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
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

func (t *tokenCommand) setToken() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(t.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Please paste your 1Password service account token (press Ctrl+D when done):")

	reader := bufio.NewReader(os.Stdin)
	var token strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}
		token.WriteString(line)
	}

	// Trim whitespace and newlines
	tokenStr := strings.TrimSpace(token.String())
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
