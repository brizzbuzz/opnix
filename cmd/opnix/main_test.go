package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	opnixerrors "github.com/brizzbuzz/opnix/internal/errors"
)

type fakeCommand struct {
	name    string
	initErr error
	runErr  error

	initArgs []string
	ran      bool
}

func (f *fakeCommand) Name() string {
	return f.name
}

func (f *fakeCommand) Init(args []string) error {
	f.initArgs = append(f.initArgs, args...)
	return f.initErr
}

func (f *fakeCommand) Run() error {
	f.ran = true
	return f.runErr
}

func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
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

	outBytes, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	errBytes, err := io.ReadAll(stderrReader)
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

	return string(outBytes), string(errBytes)
}

func TestRun_NoArguments(t *testing.T) {
	cmds := []command{&fakeCommand{name: "secret"}}

	_, stderr := captureOutput(t, func() {
		exitCode := run([]string{"opnix"}, cmds)
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	if !strings.Contains(stderr, "Usage: opnix <command>") {
		t.Fatalf("expected usage message in stderr, got: %s", stderr)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	cmds := []command{&fakeCommand{name: "secret"}}

	_, stderr := captureOutput(t, func() {
		exitCode := run([]string{"opnix", "unknown"}, cmds)
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	if !strings.Contains(stderr, "Unknown command: unknown") {
		t.Fatalf("expected unknown command message, got: %s", stderr)
	}
}

func TestRun_InitError(t *testing.T) {
	initErr := errors.New("initialization failed")
	fc := &fakeCommand{name: "secret", initErr: initErr}
	cmds := []command{fc}

	_, stderr := captureOutput(t, func() {
		exitCode := run([]string{"opnix", "secret"}, cmds)
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	if !strings.Contains(stderr, initErr.Error()) {
		t.Fatalf("expected init error in stderr, got: %s", stderr)
	}
}

func TestRun_Success(t *testing.T) {
	fc := &fakeCommand{name: "secret"}
	cmds := []command{fc}

	exitCode := run([]string{"opnix", "secret"}, cmds)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if !fc.ran {
		t.Fatal("expected command to run")
	}
}

func TestRun_RunError(t *testing.T) {
	runErr := errors.New("execution failed")
	fc := &fakeCommand{name: "secret", runErr: runErr}
	cmds := []command{fc}

	_, stderr := captureOutput(t, func() {
		exitCode := run([]string{"opnix", "secret"}, cmds)
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	if !strings.Contains(stderr, runErr.Error()) {
		t.Fatalf("expected run error in stderr, got: %s", stderr)
	}
}

func TestHandleError(t *testing.T) {
	t.Run("opnix error", func(t *testing.T) {
		opErr := opnixerrors.ConfigError(
			"Testing error handling",
			"config failure encountered",
			fmt.Errorf("json invalid"),
		)

		_, stderr := captureOutput(t, func() {
			handleError(opErr)
		})

		if !strings.Contains(stderr, "config failure encountered") {
			t.Fatalf("expected detailed config error output, got: %s", stderr)
		}
	})

	t.Run("standard error", func(t *testing.T) {
		_, stderr := captureOutput(t, func() {
			handleError(fmt.Errorf("file missing"))
		})

		if !strings.Contains(stderr, "ERROR:") || !strings.Contains(stderr, "file missing") {
			t.Fatalf("expected formatted generic error, got: %s", stderr)
		}
	})

	t.Run("no error", func(t *testing.T) {
		stdout, stderr := captureOutput(t, func() {
			handleError(nil)
		})

		if stdout != "" || stderr != "" {
			t.Fatalf("expected no output for nil error, got stdout: %q stderr: %q", stdout, stderr)
		}
	})

	t.Run("file not found suggestions", func(t *testing.T) {
		_, stderr := captureOutput(t, func() {
			handleError(fmt.Errorf("open missing: no such file or directory"))
		})

		if !strings.Contains(stderr, "ERROR: File not found") {
			t.Fatalf("expected file-not-found formatting, got: %s", stderr)
		}
	})

	t.Run("permission denied suggestions", func(t *testing.T) {
		_, stderr := captureOutput(t, func() {
			handleError(fmt.Errorf("open denied: permission denied"))
		})

		if !strings.Contains(stderr, "ERROR: Permission denied") {
			t.Fatalf("expected permission formatting, got: %s", stderr)
		}
	})
}

type stubSecretResolver struct {
	values map[string]string
	err    error
}

func (s stubSecretResolver) ResolveSecret(ref string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if value, ok := s.values[ref]; ok {
		return value, nil
	}
	return "", fmt.Errorf("secret %s not found", ref)
}

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func TestLoadEnvConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "env.json", `{
		"vars": [
			{"name": "API_TOKEN", "reference": "op://Vault/Item/token"},
			{"name": "STATIC_VALUE", "value": "hello"}
		]
	}`)

	cfg, err := loadEnvConfig(path)
	if err != nil {
		t.Fatalf("expected load success, got error: %v", err)
	}

	if len(cfg.Vars) != 2 {
		t.Fatalf("expected 2 vars, got %d", len(cfg.Vars))
	}

	t.Run("no vars", func(t *testing.T) {
		p := writeTempFile(t, dir, "no-vars.json", `{"vars": []}`)
		_, err := loadEnvConfig(p)
		if err == nil {
			t.Fatal("expected validation error for empty vars")
		}
	})

	errorCases := []struct {
		name    string
		content string
	}{
		{
			name:    "invalid name",
			content: `{"vars": [{"name": "lowercase", "reference": "op://Vault/Item/token"}]}`,
		},
		{
			name:    "both reference and value",
			content: `{"vars": [{"name": "API_TOKEN", "reference": "op://Vault/Item/token", "value": "static"}]}`,
		},
		{
			name:    "missing reference and value",
			content: `{"vars": [{"name": "API_TOKEN"}]}`,
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeTempFile(t, dir, fmt.Sprintf("%s.json", tc.name), tc.content)
			if _, err := loadEnvConfig(p); err == nil {
				t.Fatalf("expected error for case %s", tc.name)
			}
		})
	}
}

func TestEnvProcessor(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := &envConfig{
			Vars: []envVariable{
				{Name: "API_TOKEN", Reference: "op://Vault/Item/token"},
				{Name: "STATIC", Value: " hello "},
			},
		}

		resolver := stubSecretResolver{values: map[string]string{
			"op://Vault/Item/token": " secret\n",
		}}

		result, err := newEnvProcessor(resolver).Process(cfg)
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}

		if len(result.Values) != 2 {
			t.Fatalf("expected 2 values, got %d", len(result.Values))
		}

		if result.Values["API_TOKEN"] != "secret" {
			t.Fatalf("unexpected API_TOKEN value: %q", result.Values["API_TOKEN"])
		}

		if result.Values["STATIC"] != "hello" {
			t.Fatalf("unexpected STATIC value: %q", result.Values["STATIC"])
		}
	})

	t.Run("optional skip", func(t *testing.T) {
		cfg := &envConfig{
			Vars: []envVariable{
				{Name: "OPTIONAL_SECRET", Reference: "op://Vault/Item/missing", Optional: true},
			},
		}

		resolver := stubSecretResolver{err: fmt.Errorf("missing")}
		result, err := newEnvProcessor(resolver).Process(cfg)
		if err != nil {
			t.Fatalf("expected success due to optional var, got error: %v", err)
		}

		if len(result.Values) != 0 {
			t.Fatalf("expected no values, got %d", len(result.Values))
		}

		if len(result.Skipped) != 1 || result.Skipped[0].Name != "OPTIONAL_SECRET" {
			t.Fatalf("expected optional skip recorded, got %#v", result.Skipped)
		}
	})

	t.Run("required error", func(t *testing.T) {
		cfg := &envConfig{
			Vars: []envVariable{
				{Name: "REQUIRED", Reference: "op://Vault/Item/missing"},
			},
		}

		resolver := stubSecretResolver{err: fmt.Errorf("missing")}
		if _, err := newEnvProcessor(resolver).Process(cfg); err == nil {
			t.Fatal("expected error for required variable")
		}
	})
}

func TestEnvCommand_RunShell(t *testing.T) {
	t.Setenv("OPNIX_ENV_CONFIG", "")
	t.Setenv("OPNIX_ENV_CONFIG_JSON", "")
	t.Setenv("OPNIX_ENV_TOKEN_FILE", "")

	dir := t.TempDir()
	configPath := writeTempFile(t, dir, "env.json", `{
		"vars": [
			{"name": "API_TOKEN", "reference": "op://Vault/Item/token"}
		]
	}`)

	cmd := newEnvCommand()
	cmd.configPath = configPath
	cmd.tokenFile = filepath.Join(dir, "token")
	cmd.loadConfig = func(string) (*envConfig, error) {
		return &envConfig{
			Vars: []envVariable{
				{Name: "API_TOKEN", Reference: "op://Vault/Item/token"},
			},
		}, nil
	}
	cmd.newClient = func(string) (secretResolver, error) {
		return stubSecretResolver{values: map[string]string{
			"op://Vault/Item/token": "secret-value",
		}}, nil
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected run success, got error: %v", err)
		}
	})

	if !strings.Contains(stdout, "export API_TOKEN='secret-value'") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got: %s", stderr)
	}
}

func TestEnvCommand_RunOptionalSkip(t *testing.T) {
	t.Setenv("OPNIX_ENV_CONFIG", "")
	t.Setenv("OPNIX_ENV_CONFIG_JSON", "")
	t.Setenv("OPNIX_ENV_TOKEN_FILE", "")

	dir := t.TempDir()
	configPath := writeTempFile(t, dir, "env.json", `{
		"vars": [
			{"name": "OPTIONAL", "reference": "op://Vault/missing", "optional": true}
		]
	}`)

	cmd := newEnvCommand()
	cmd.configPath = configPath
	cmd.loadConfig = func(string) (*envConfig, error) {
		return &envConfig{
			Vars: []envVariable{
				{Name: "OPTIONAL", Reference: "op://Vault/missing", Optional: true},
			},
		}, nil
	}
	cmd.newClient = func(string) (secretResolver, error) {
		return stubSecretResolver{err: fmt.Errorf("not found")}, nil
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected run success, got error: %v", err)
		}
	})

	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout, got: %q", stdout)
	}
	if !strings.Contains(stderr, "Skipped optional env var OPTIONAL") {
		t.Fatalf("expected warning in stderr, got: %q", stderr)
	}
}

func TestEnvCommand_InvalidFormat(t *testing.T) {
	t.Setenv("OPNIX_ENV_CONFIG", "")
	t.Setenv("OPNIX_ENV_CONFIG_JSON", "")
	t.Setenv("OPNIX_ENV_TOKEN_FILE", "")

	dir := t.TempDir()
	configPath := writeTempFile(t, dir, "env.json", `{
		"vars": [
			{"name": "API_TOKEN", "reference": "op://Vault/Item/token"}
		]
	}`)

	cmd := newEnvCommand()
	cmd.configPath = configPath
	cmd.format = "unknown"
	cmd.loadConfig = func(string) (*envConfig, error) {
		return &envConfig{
			Vars: []envVariable{
				{Name: "API_TOKEN", Reference: "op://Vault/Item/token"},
			},
		}, nil
	}

	if err := cmd.Run(); err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestEnvCommand_RunWithConfigJSON(t *testing.T) {
	t.Setenv("OPNIX_ENV_CONFIG", "")
	t.Setenv("OPNIX_ENV_CONFIG_JSON", "")
	t.Setenv("OPNIX_ENV_TOKEN_FILE", "")

	cmd := newEnvCommand()
	cmd.configJSON = `{"vars":[{"name":"API_TOKEN","reference":"op://Vault/Item/token"}]}`
	cmd.newClient = func(string) (secretResolver, error) {
		return stubSecretResolver{values: map[string]string{
			"op://Vault/Item/token": "secret-value",
		}}, nil
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected run success, got error: %v", err)
		}
	})

	if !strings.Contains(stdout, "export API_TOKEN='secret-value'") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got: %s", stderr)
	}
}

func TestEnvCommand_MissingConfig(t *testing.T) {
	t.Setenv("OPNIX_ENV_CONFIG", "")
	t.Setenv("OPNIX_ENV_CONFIG_JSON", "")
	t.Setenv("OPNIX_ENV_TOKEN_FILE", "")

	cmd := newEnvCommand()
	err := cmd.Run()
	if err == nil || !strings.Contains(err.Error(), "No environment configuration provided") {
		t.Fatalf("expected missing configuration error, got: %v", err)
	}
}

func TestEnvCommandResolveConfig(t *testing.T) {
	t.Run("uses inline JSON first", func(t *testing.T) {
		cmd := newEnvCommand()
		cmd.configJSON = `{"vars":[{"name":"API_TOKEN","value":"test"}]}`
		cmd.loadConfig = func(string) (*envConfig, error) {
			t.Fatal("loadConfig should not be called when config JSON is set")
			return nil, nil
		}

		cfg, err := cmd.resolveConfig()
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if len(cfg.Vars) != 1 || cfg.Vars[0].Name != "API_TOKEN" {
			t.Fatalf("unexpected config: %#v", cfg)
		}
	})

	t.Run("uses env path", func(t *testing.T) {
		cmd := newEnvCommand()
		t.Setenv("OPNIX_ENV_CONFIG", "/tmp/env.json")
		cmd.loadConfig = func(path string) (*envConfig, error) {
			if path != "/tmp/env.json" {
				t.Fatalf("expected env path, got %q", path)
			}
			return &envConfig{Vars: []envVariable{{Name: "API_TOKEN", Value: "test"}}}, nil
		}

		cfg, err := cmd.resolveConfig()
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if len(cfg.Vars) != 1 {
			t.Fatalf("unexpected vars: %#v", cfg.Vars)
		}
	})

	t.Run("uses env JSON", func(t *testing.T) {
		cmd := newEnvCommand()
		t.Setenv("OPNIX_ENV_CONFIG_JSON", `{"vars":[{"name":"API_TOKEN","value":"test"}]}`)

		cfg, err := cmd.resolveConfig()
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if len(cfg.Vars) != 1 || cfg.Vars[0].Value != "test" {
			t.Fatalf("unexpected config: %#v", cfg)
		}
	})
}

func TestEnvCommandBuildResolver(t *testing.T) {
	t.Run("uses static resolver for literal values", func(t *testing.T) {
		cmd := newEnvCommand()
		resolver, err := cmd.buildResolver(&envConfig{Vars: []envVariable{{Name: "STATIC", Value: "value"}}})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if _, ok := resolver.(staticResolver); !ok {
			t.Fatalf("expected staticResolver, got %T", resolver)
		}
	})

	t.Run("creates onepassword client when references exist", func(t *testing.T) {
		cmd := newEnvCommand()
		cmd.tokenFile = "/tmp/token"
		cmd.newClient = func(path string) (secretResolver, error) {
			if path != "/tmp/token" {
				t.Fatalf("expected token file path, got %q", path)
			}
			return stubSecretResolver{}, nil
		}

		resolver, err := cmd.buildResolver(&envConfig{Vars: []envVariable{{Name: "API_TOKEN", Reference: "op://Vault/Item/token"}}})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if _, ok := resolver.(stubSecretResolver); !ok {
			t.Fatalf("expected stub resolver, got %T", resolver)
		}
	})
}

func TestRenderHelpers(t *testing.T) {
	if got := shellQuote("foo'bar"); got != "'foo'\"'\"'bar'" {
		t.Fatalf("unexpected shell quote: %q", got)
	}
	if got := shellQuote(""); got != "''" {
		t.Fatalf("unexpected empty shell quote: %q", got)
	}

	if got := dotenvValue("simple"); got != "simple" {
		t.Fatalf("unexpected dotenv value: %q", got)
	}
	if got := dotenvValue("line 1\nline 2"); got != `"line 1\nline 2"` {
		t.Fatalf("unexpected escaped dotenv value: %q", got)
	}

	values := map[string]string{"B": "two", "A": "one"}
	if got := renderShell(values); got != "export A='one'\nexport B='two'\n" {
		t.Fatalf("unexpected shell render: %q", got)
	}
	if got := renderDotenv(values); got != "A=one\nB=two\n" {
		t.Fatalf("unexpected dotenv render: %q", got)
	}

	jsonOut, err := renderOutput(values, "json")
	if err != nil {
		t.Fatalf("expected JSON render success, got %v", err)
	}
	if !strings.Contains(jsonOut, "\"A\": \"one\"") {
		t.Fatalf("unexpected JSON output: %s", jsonOut)
	}

	if _, err := renderOutput(values, "unknown"); err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestHelpers(t *testing.T) {
	if !(envVariable{}).shouldTrim() {
		t.Fatal("expected default env variable to trim")
	}
	if (envVariable{PreserveWhitespace: true}).shouldTrim() {
		t.Fatal("expected preserve whitespace variable to skip trimming")
	}

	if !isSupportedFormat("shell") || !isSupportedFormat("dotenv") || !isSupportedFormat("json") {
		t.Fatal("expected known formats to be supported")
	}
	if isSupportedFormat("yaml") {
		t.Fatal("expected yaml to be unsupported")
	}

	if _, err := (staticResolver{}).ResolveSecret("op://Vault/Item/token"); err == nil {
		t.Fatal("expected static resolver error")
	}
}
