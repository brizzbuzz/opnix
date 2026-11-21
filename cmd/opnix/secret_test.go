package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brizzbuzz/opnix/internal/config"
	"github.com/brizzbuzz/opnix/internal/secrets"
	"github.com/brizzbuzz/opnix/internal/systemd"
)

type testClient struct{ err error }

func (c testClient) ResolveSecrets([]string) (map[string]string, error) {
	return map[string]string{"op://Example/Service/password": "value"}, c.err
}
func (c testClient) ResolveFiles([]string) (map[string][]byte, error) { return nil, c.err }

type testProcessor struct{ result *secrets.ProcessResult }

func (p testProcessor) Process(*config.Config) (*secrets.ProcessResult, error) { return p.result, nil }

type testSystemd struct{ outputs []systemd.Output }

func (m *testSystemd) ProcessOutputChanges(outputs []systemd.Output) error {
	m.outputs = outputs
	return nil
}

func TestSecretRunPassesProcessedOutputs(t *testing.T) {
	dir := t.TempDir()
	s := newSecretCommand()
	s.configFile = filepath.Join(dir, "config")
	if err := os.WriteFile(s.configFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s.tokenFile = filepath.Join(dir, "token")
	services := []interface{}{"service"}
	cfg := &config.Config{Secrets: []config.Secret{{Path: "relative", Services: services}, {Services: services}}, Templates: []config.Template{{Name: "app"}}, SystemdIntegration: config.SystemdIntegration{Enable: true}}
	s.loadConfig = func(string) (*config.Config, error) { return cfg, nil }
	s.newClient = func(string) (secrets.SecretClient, error) { return testClient{}, nil }
	s.processorFactory = func(secrets.SecretClient, string) secretProcessor {
		return testProcessor{&secrets.ProcessResult{SecretPaths: map[string]string{"secret[0]:relative": "/actual/secret", "secret[1]:": "/actual/generated"}, ProcessedCount: 2, Templates: []secrets.TemplateResult{{Name: "app", Path: "/actual/template", Services: services}}}}
	}
	manager := &testSystemd{}
	s.systemdFactory = func(config.SystemdIntegration) (systemdManager, error) { return manager, nil }
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	want := []systemd.Output{{Name: "secret[0]:relative", Path: "/actual/secret", Services: services}, {Name: "secret[1]:", Path: "/actual/generated", Services: services}, {Name: "template:app", Path: "/actual/template", Services: services}}
	if !reflect.DeepEqual(manager.outputs, want) {
		t.Fatalf("outputs = %#v, want %#v", manager.outputs, want)
	}
}

func TestSecretRunFailureDoesNotMutateOutput(t *testing.T) {
	for _, failure := range []string{"render", "provider"} {
		for _, existing := range []bool{false, true} {
			t.Run(failure+map[bool]string{false: "-missing", true: "-existing"}[existing], func(t *testing.T) {
				dir := t.TempDir()
				s := newSecretCommand()
				s.configFile = filepath.Join(dir, "config")
				if err := os.WriteFile(s.configFile, nil, 0600); err != nil {
					t.Fatal(err)
				}
				s.tokenFile = filepath.Join(dir, "token")
				s.outputDir = filepath.Join(dir, "output")
				probe := filepath.Join(s.outputDir, ".opnix-test")
				if existing {
					if err := os.Mkdir(s.outputDir, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(probe, []byte("keep"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				cfg := &config.Config{Secrets: []config.Secret{{Name: "password", Path: "source", Reference: "op://Example/Service/password"}}, Templates: []config.Template{{Name: "app", Path: "app", Content: "<OPNIX:invalid>"}}}
				client := testClient{}
				if failure == "provider" {
					cfg.Templates[0].Content = config.Placeholder("password")
					client.err = errors.New("provider failed")
				}
				s.loadConfig = func(string) (*config.Config, error) { return cfg, nil }
				s.newClient = func(string) (secrets.SecretClient, error) { return client, nil }
				if err := s.Run(); err == nil {
					t.Fatal("expected processing failure")
				}
				if existing {
					data, err := os.ReadFile(probe)
					if err != nil || string(data) != "keep" {
						t.Fatalf("probe changed: %q, %v", data, err)
					}
				} else if _, err := os.Stat(s.outputDir); !os.IsNotExist(err) {
					t.Fatalf("output directory exists or stat failed: %v", err)
				}
			})
		}
	}
}
