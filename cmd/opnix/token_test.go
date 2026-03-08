package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdio(t *testing.T, fn func()) (string, string) {
	t.Helper()

	origStdout := os.Stdout
	origStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	fn()

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("failed to close stdout writer: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("failed to close stderr writer: %v", err)
	}

	stdoutBytes, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	stderrBytes, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("failed to read stderr: %v", err)
	}

	if err := stdoutReader.Close(); err != nil {
		t.Fatalf("failed to close stdout reader: %v", err)
	}
	if err := stderrReader.Close(); err != nil {
		t.Fatalf("failed to close stderr reader: %v", err)
	}

	os.Stdout = origStdout
	os.Stderr = origStderr

	return string(stdoutBytes), string(stderrBytes)
}

func withMockInput(t *testing.T, input string, fn func()) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	origStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
	}()

	fn()

	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader: %v", err)
	}
}

func TestTokenCommandInit(t *testing.T) {
	tc := newTokenCommand()

	if err := tc.Init([]string{}); err == nil {
		t.Fatal("expected error when no subcommand provided")
	}

	args := []string{"-path", "/tmp/token", "set"}
	if err := tc.Init(args); err != nil {
		t.Fatalf("expected init to succeed, got: %v", err)
	}
	if tc.action != "set" {
		t.Fatalf("expected action 'set', got %q", tc.action)
	}
	if tc.Name() != "token" {
		t.Fatalf("expected command name token, got %q", tc.Name())
	}
}

func TestTokenCommandRunUnknownAction(t *testing.T) {
	tc := newTokenCommand()
	tc.action = "unknown"

	if err := tc.Run(); err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestTokenCommandSetTokenSuccess(t *testing.T) {
	tc := newTokenCommand()

	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token")

	if err := tc.Init([]string{"-path", tokenPath, "set"}); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	withMockInput(t, "my-secret-token\n", func() {
		_, stderr := captureStdio(t, func() {
			if err := tc.Run(); err != nil {
				t.Fatalf("expected run to succeed, got: %v", err)
			}
		})

		if !strings.Contains(stderr, "Token successfully stored") {
			t.Fatalf("expected success message, got: %s", stderr)
		}
	})

	content, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("failed to read token file: %v", err)
	}

	if string(content) != "my-secret-token" {
		t.Fatalf("expected token to be trimmed, got %q", string(content))
	}
}

func TestTokenCommandSetTokenEmpty(t *testing.T) {
	tc := newTokenCommand()

	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token")

	if err := tc.Init([]string{"-path", tokenPath, "set"}); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	withMockInput(t, "\n", func() {
		if err := tc.Run(); err == nil {
			t.Fatal("expected error for empty token")
		}
	})
}

func TestTokenCommandCheckWritePermissions(t *testing.T) {
	tc := newTokenCommand()

	t.Run("creates missing directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "nested", "token")
		tc.path = tokenPath

		if err := tc.checkWritePermissions(); err != nil {
			t.Fatalf("expected permissions check to succeed, got: %v", err)
		}

		if _, err := os.Stat(filepath.Dir(tokenPath)); err != nil {
			t.Fatalf("expected directory to be created, got: %v", err)
		}
	})

	t.Run("fails when parent path is file", func(t *testing.T) {
		tmpDir := t.TempDir()
		blocker := filepath.Join(tmpDir, "block")
		if err := os.WriteFile(blocker, []byte("content"), 0600); err != nil {
			t.Fatalf("failed to create blocker file: %v", err)
		}

		tc.path = filepath.Join(blocker, "token")
		if err := tc.checkWritePermissions(); err == nil {
			t.Fatal("expected error when parent path is a file")
		}
	})
}
