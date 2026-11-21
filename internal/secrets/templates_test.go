package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brizzbuzz/opnix/internal/config"
)

func TestTemplatePreflightRootOwnership(t *testing.T) {
	mode, uid, gid, err := metadata("root", "root", "")
	if err != nil || mode != 0600 || uid != 0 || gid != 0 {
		t.Fatalf("root metadata: mode %v, uid %d, gid %d, error %v", mode, uid, gid, err)
	}

	dir := t.TempDir()
	cfg := &config.Config{
		Secrets:   []config.Secret{{Name: "password", Path: "source", Owner: "root", Group: "root"}},
		Templates: []config.Template{{Name: "app", Path: "app", Owner: "root", Group: "root"}},
	}
	prepared, err := NewProcessor(&mockClient{}, dir).prepare(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 1 || prepared[0].uid != 0 || prepared[0].gid != 0 || prepared[0].mode != 0600 {
		t.Fatalf("unexpected prepared templates: %#v", prepared)
	}
}

func TestWriteTemplateClearsSpecialPermissionBits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app")
	content := []byte("unchanged")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	const special = os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	if err := os.Chmod(path, 0600|special); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if before.Mode()&special == 0 {
		t.Skip("filesystem did not retain any special permission bits")
	}
	if before.Mode().Perm() != 0600 {
		t.Fatalf("unexpected setup mode: %v", before.Mode())
	}
	if err := writeTemplate(preparedTemplate{path: path, content: content, mode: 0600, uid: -1, gid: -1}); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || after.Mode().Perm() != 0600 || after.Mode()&special != 0 {
		t.Fatalf("unchanged file not reconciled: before %v, after %v", before.Mode(), after.Mode())
	}
}

func TestTemplateProcessingAndUnchangedReconciliation(t *testing.T) {
	dir := t.TempDir()
	ref := "op://Example/Service/password"
	cfg := &config.Config{Secrets: []config.Secret{{Name: "password", Path: "source", Reference: ref}}, Templates: []config.Template{{Name: "app", Path: "app", Content: "value=" + config.Placeholder("password"), Mode: "0640", Services: []string{"app"}}}}
	p := NewProcessor(&mockClient{secrets: map[string]string{ref: "hello"}}, dir)
	result, err := p.Process(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "app")
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "value=hello" {
		t.Fatalf("output %q, %v", data, err)
	}
	if len(result.Templates) != 1 || result.Templates[0].Name != "app" || result.Templates[0].Path != path || result.ProcessedCount != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	before, _ := os.Stat(path)
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Process(cfg); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !os.SameFile(before, after) || after.Mode().Perm() != 0640 {
		t.Fatalf("unchanged file not reconciled: %v", after.Mode())
	}
}

func TestTemplatePreflightLeavesOutputsUnchanged(t *testing.T) {
	for _, failure := range []string{"render", "provider", "metadata", "collision", "symlink"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			dest := filepath.Join(dir, "app")
			if err := os.WriteFile(source, []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			ref := "op://Example/Service/password"
			cfg := &config.Config{Secrets: []config.Secret{{Name: "password", Path: "source", Reference: ref}}, Templates: []config.Template{{Name: "app", Path: "app", Content: config.Placeholder("password")}}}
			client := &mockClient{secrets: map[string]string{ref: "new"}}
			switch failure {
			case "render":
				cfg.Templates[0].Content = "sensitive-content <OPNIX:invalid>"
			case "provider":
				client.resolveErr = fmt.Errorf("sensitive-content")
			case "metadata":
				cfg.Templates[0].Owner = "opnix-user-does-not-exist-1438"
			case "collision":
				cfg.Templates[0].Path = source
			case "symlink":
				if err := os.Symlink("missing", dest); err != nil {
					t.Fatal(err)
				}
			}
			_, err := NewProcessor(client, dir).Process(cfg)
			if err == nil || strings.Contains(err.Error(), "sensitive-content") {
				t.Fatalf("unsafe error: %v", err)
			}
			data, err := os.ReadFile(source)
			if err != nil || string(data) != "old" {
				t.Fatalf("source changed: %q %v", data, err)
			}
		})
	}
}

func TestTemplatePreflightLateFailures(t *testing.T) {
	for _, failure := range []string{"missing provider result", "late owner", "late mode", "special mode", "link collision", "parent alias", "parent child", "secret final symlink", "protected alias"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "app")
			for _, path := range []string{source, dest} {
				if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			ref := "op://Example/Service/password"
			cfg := &config.Config{Secrets: []config.Secret{{Name: "password", Path: source, Reference: ref}}, Templates: []config.Template{{Name: "app", Path: dest, Content: config.Placeholder("password")}, {Name: "later", Path: filepath.Join(dir, "later"), Content: "later"}}}
			client := &mockClient{secrets: map[string]string{ref: "new"}}
			switch failure {
			case "missing provider result":
				cfg.Secrets = append(cfg.Secrets, config.Secret{Name: "other", Path: filepath.Join(dir, "other"), Reference: "op://Example/Service/other"})
			case "late owner":
				cfg.Templates[1].Owner = "opnix-user-does-not-exist-1438"
			case "late mode":
				cfg.Templates[1].Mode = "not-octal"
			case "special mode":
				cfg.Templates[1].Mode = "4755"
			case "link collision":
				cfg.Secrets[0].Symlinks = []string{dest}
			case "parent alias":
				alias := filepath.Join(dir, "alias")
				if err := os.Symlink(dir, alias); err != nil {
					t.Fatal(err)
				}
				cfg.Templates[1].Path = filepath.Join(alias, "app")
			case "parent child":
				cfg.Templates[1].Path = filepath.Join(dest, "child")
			case "secret final symlink":
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(dest, source); err != nil {
					t.Fatal(err)
				}
			case "protected alias":
				alias := filepath.Join(dir, "protected")
				if err := os.Symlink("/usr/bin", alias); err != nil {
					t.Fatal(err)
				}
				cfg.Templates[1].Path = filepath.Join(alias, "opnix-test")
			}
			if _, err := NewProcessor(client, dir).Process(cfg); err == nil {
				t.Fatal("expected failure")
			}
			for _, path := range []string{source, dest} {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "old" {
					t.Fatalf("%s changed: %q %v", path, data, err)
				}
			}
		})
	}
}

func TestWriteTemplateReplacementAndCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTemplate(preparedTemplate{path: path, content: []byte("new"), mode: 0640, uid: -1, gid: -1}); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new" || os.SameFile(before, after) || after.Mode().Perm() != 0640 {
		t.Fatalf("replacement: %q %v %v", data, after.Mode(), err)
	}
	if err := writeTemplate(preparedTemplate{path: path, content: []byte("new"), mode: 0640, uid: -1, gid: -1}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(after, unchanged) || after.ModTime() != unchanged.ModTime() {
		t.Fatal("unchanged output modified")
	}
	if os.Geteuid() == 0 {
		t.Skip("ownership failure requires an unprivileged user")
	}
	if err := writeTemplate(preparedTemplate{path: path, content: []byte("replacement"), mode: 0600, uid: 0, gid: -1}); err == nil {
		t.Fatal("expected chown failure")
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "new" {
		t.Fatalf("destination changed: %q %v", data, err)
	}
	entries, err := filepath.Glob(filepath.Join(dir, ".opnix-template-*"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary files: %v %v", entries, err)
	}
}
