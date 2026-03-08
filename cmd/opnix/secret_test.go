package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/brizzbuzz/opnix/internal/config"
	"github.com/brizzbuzz/opnix/internal/secrets"
)

type stubSecretClient struct{}

func (stubSecretClient) ResolveSecret(string) (string, error) {
	return "stub-value", nil
}

type stubProcessor struct {
	result *secrets.ProcessResult
	err    error

	receivedConfig *config.Config
}

func (s *stubProcessor) Process(cfg *config.Config) (*secrets.ProcessResult, error) {
	s.receivedConfig = cfg
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type stubSystemdManager struct {
	err error

	receivedSecrets     []config.Secret
	receivedSecretPaths map[string]string
}

func (s *stubSystemdManager) ProcessSecretChanges(secrets []config.Secret, secretPaths map[string]string) error {
	s.receivedSecrets = append([]config.Secret(nil), secrets...)
	s.receivedSecretPaths = make(map[string]string, len(secretPaths))
	for k, v := range secretPaths {
		s.receivedSecretPaths[k] = v
	}

	return s.err
}

func createTempConfig(t *testing.T, data string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func createTempToken(t *testing.T, value string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatalf("failed to write token: %v", err)
	}
	return path
}

func TestSecretCommandValidatePrerequisites(t *testing.T) {
	t.Run("name and init", func(t *testing.T) {
		sc := newSecretCommand()
		if sc.Name() != "secret" {
			t.Fatalf("expected command name secret, got %q", sc.Name())
		}
		if err := sc.Init([]string{"-config", "custom.json", "-output", "out", "-token-file", "token"}); err != nil {
			t.Fatalf("expected init success, got %v", err)
		}
		if sc.configFile != "custom.json" || sc.outputDir != "out" || sc.tokenFile != "token" {
			t.Fatalf("unexpected parsed values: %#v", sc)
		}
	})

	t.Run("missing config file", func(t *testing.T) {
		sc := newSecretCommand()
		sc.configFile = filepath.Join(t.TempDir(), "missing.json")
		sc.outputDir = filepath.Join(t.TempDir(), "out")
		sc.tokenFile = createTempToken(t, "token-value")

		if err := sc.validatePrerequisites(); err == nil {
			t.Fatal("expected error for missing config file")
		}
	})

	t.Run("output directory not writable", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "config.json")
		if err := os.WriteFile(cfgPath, []byte(`{"secrets":[]}`), 0600); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		outputPath := filepath.Join(tmpDir, "output")
		if err := os.WriteFile(outputPath, []byte("not a dir"), 0600); err != nil {
			t.Fatalf("failed to create blocking file: %v", err)
		}

		sc := newSecretCommand()
		sc.configFile = cfgPath
		sc.outputDir = outputPath
		sc.tokenFile = createTempToken(t, "token")

		if err := sc.validatePrerequisites(); err == nil {
			t.Fatal("expected error when output path is a file")
		}
	})

	t.Run("success", func(t *testing.T) {
		cfgPath := createTempConfig(t, `{"secrets":[{"path":"test","reference":"op://vault/item/field"}]}`)
		tokenPath := createTempToken(t, "token")

		sc := newSecretCommand()
		sc.configFile = cfgPath
		sc.outputDir = filepath.Join(t.TempDir(), "out")
		sc.tokenFile = tokenPath

		if err := sc.validatePrerequisites(); err != nil {
			t.Fatalf("expected prerequisites to pass, got error: %v", err)
		}
	})
}

func TestSecretCommandCheckOutputDirectory(t *testing.T) {
	sc := newSecretCommand()
	sc.outputDir = filepath.Join(t.TempDir(), "nested", "out")

	if err := sc.checkOutputDirectory(); err != nil {
		t.Fatalf("expected output directory to be prepared, got %v", err)
	}

	if _, err := os.Stat(sc.outputDir); err != nil {
		t.Fatalf("expected output directory to exist, got %v", err)
	}
}

func TestSecretCommandRunSuccess(t *testing.T) {
	cfgData := &config.Config{
		Secrets: []config.Secret{
			{Path: "test", Reference: "op://vault/item/field"},
		},
	}

	cfgPath := createTempConfig(t, `{"secrets":[{"path":"test","reference":"op://vault/item/field"}]}`)
	tokenPath := createTempToken(t, "token")
	outputDir := filepath.Join(t.TempDir(), "out")

	sc := newSecretCommand()
	sc.configFile = cfgPath
	sc.tokenFile = tokenPath
	sc.outputDir = outputDir

	processor := &stubProcessor{
		result: &secrets.ProcessResult{
			SecretPaths:    map[string]string{"secret[0]:test": filepath.Join(outputDir, "test")},
			ProcessedCount: 1,
		},
	}

	sc.loadConfig = func(string) (*config.Config, error) {
		return cfgData, nil
	}
	sc.newClient = func(string) (secrets.SecretClient, error) {
		return stubSecretClient{}, nil
	}
	sc.processorFactory = func(secrets.SecretClient, string) secretProcessor {
		return processor
	}
	sc.systemdFactory = func(config.SystemdIntegration) (systemdManager, error) {
		t.Fatal("systemd factory should not be called when integration disabled")
		return nil, nil
	}

	if err := sc.Run(); err != nil {
		t.Fatalf("expected run to succeed, got error: %v", err)
	}

	if processor.receivedConfig != cfgData {
		t.Fatal("expected processor to receive config")
	}
}

func TestSecretCommandRunWithSystemd(t *testing.T) {
	cfgData := &config.Config{
		Secrets: []config.Secret{
			{Path: "test", Reference: "op://vault/item/field", Services: []interface{}{"svc"}},
		},
		SystemdIntegration: config.SystemdIntegration{
			Enable: true,
		},
	}

	cfgPath := createTempConfig(t, `{"secrets":[{"path":"test","reference":"op://vault/item/field","services":["svc"]}],"systemdIntegration":{"enable":true}}`)
	tokenPath := createTempToken(t, "token")
	outputDir := filepath.Join(t.TempDir(), "out")

	sc := newSecretCommand()
	sc.configFile = cfgPath
	sc.tokenFile = tokenPath
	sc.outputDir = outputDir

	processor := &stubProcessor{
		result: &secrets.ProcessResult{
			SecretPaths:    map[string]string{"secret[0]:test": filepath.Join(outputDir, "test")},
			ProcessedCount: 1,
		},
	}

	systemdMgr := &stubSystemdManager{}

	sc.loadConfig = func(string) (*config.Config, error) {
		return cfgData, nil
	}
	sc.newClient = func(string) (secrets.SecretClient, error) {
		return stubSecretClient{}, nil
	}
	sc.processorFactory = func(secrets.SecretClient, string) secretProcessor {
		return processor
	}
	sc.systemdFactory = func(config.SystemdIntegration) (systemdManager, error) {
		return systemdMgr, nil
	}

	if err := sc.Run(); err != nil {
		t.Fatalf("expected run to succeed, got error: %v", err)
	}

	if len(systemdMgr.receivedSecrets) != len(cfgData.Secrets) {
		t.Fatalf("expected systemd manager to receive secrets; got %d", len(systemdMgr.receivedSecrets))
	}
	if len(systemdMgr.receivedSecretPaths) != len(processor.result.SecretPaths) {
		t.Fatalf("expected secret paths to be forwarded; got %d", len(systemdMgr.receivedSecretPaths))
	}
}

func TestSecretCommandRunErrors(t *testing.T) {
	cfgPath := createTempConfig(t, `{"secrets":[{"path":"test","reference":"op://vault/item/field"}]}`)
	tokenPath := createTempToken(t, "token")
	outputDir := filepath.Join(t.TempDir(), "out")

	newCommand := func() *secretCommand {
		sc := newSecretCommand()
		sc.configFile = cfgPath
		sc.tokenFile = tokenPath
		sc.outputDir = outputDir
		return sc
	}

	t.Run("load config error", func(t *testing.T) {
		sc := newCommand()
		expected := errors.New("load failed")
		sc.loadConfig = func(string) (*config.Config, error) {
			return nil, expected
		}
		if err := sc.Run(); !errors.Is(err, expected) {
			t.Fatalf("expected load error, got: %v", err)
		}
	})

	t.Run("client error", func(t *testing.T) {
		sc := newCommand()
		expected := errors.New("client failed")
		sc.loadConfig = func(string) (*config.Config, error) {
			return &config.Config{}, nil
		}
		sc.newClient = func(string) (secrets.SecretClient, error) {
			return nil, expected
		}

		if err := sc.Run(); !errors.Is(err, expected) {
			t.Fatalf("expected client error, got: %v", err)
		}
	})

	t.Run("processor error", func(t *testing.T) {
		sc := newCommand()
		expected := errors.New("process failed")
		sc.loadConfig = func(string) (*config.Config, error) {
			return &config.Config{}, nil
		}
		sc.newClient = func(string) (secrets.SecretClient, error) {
			return stubSecretClient{}, nil
		}
		sc.processorFactory = func(secrets.SecretClient, string) secretProcessor {
			return &stubProcessor{err: expected}
		}

		if err := sc.Run(); !errors.Is(err, expected) {
			t.Fatalf("expected processor error, got: %v", err)
		}
	})

	t.Run("systemd manager error", func(t *testing.T) {
		sc := newCommand()
		expected := errors.New("systemd failed")

		sc.loadConfig = func(string) (*config.Config, error) {
			return &config.Config{
				SystemdIntegration: config.SystemdIntegration{
					Enable: true,
				},
			}, nil
		}
		sc.newClient = func(string) (secrets.SecretClient, error) {
			return stubSecretClient{}, nil
		}
		sc.processorFactory = func(secrets.SecretClient, string) secretProcessor {
			return &stubProcessor{
				result: &secrets.ProcessResult{
					SecretPaths:    map[string]string{},
					ProcessedCount: 0,
				},
			}
		}
		sc.systemdFactory = func(config.SystemdIntegration) (systemdManager, error) {
			return &stubSystemdManager{err: expected}, nil
		}

		if err := sc.Run(); !errors.Is(err, expected) {
			t.Fatalf("expected systemd error, got: %v", err)
		}
	})

	t.Run("systemd factory error", func(t *testing.T) {
		sc := newCommand()
		expected := errors.New("factory failed")

		sc.loadConfig = func(string) (*config.Config, error) {
			return &config.Config{
				SystemdIntegration: config.SystemdIntegration{
					Enable: true,
				},
			}, nil
		}
		sc.newClient = func(string) (secrets.SecretClient, error) {
			return stubSecretClient{}, nil
		}
		sc.processorFactory = func(secrets.SecretClient, string) secretProcessor {
			return &stubProcessor{
				result: &secrets.ProcessResult{
					SecretPaths: map[string]string{},
				},
			}
		}
		sc.systemdFactory = func(config.SystemdIntegration) (systemdManager, error) {
			return nil, expected
		}

		if err := sc.Run(); !errors.Is(err, expected) {
			t.Fatalf("expected systemd factory error, got: %v", err)
		}
	})
}
