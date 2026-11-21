package systemd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brizzbuzz/opnix/internal/config"
)

func TestTemplateOutputChangesAndRecovery(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "systemctl.log")
	t.Setenv("FAKE_SYSTEMCTL_DIR", dir)
	t.Setenv("FAKE_SYSTEMCTL_LOG", logFile)
	statusPath := filepath.Join(dir, "app.status")
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(statusPath, "ActiveState=active\nSubState=running\nResult=success\n")
	path := filepath.Join(dir, "rendered")
	write(path, "first")
	store, err := NewHashStore(filepath.Join(dir, "hashes.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := &Manager{config: config.SystemdIntegration{Enable: true, RestartOnChange: true, ChangeDetection: config.ChangeDetection{Enable: true}, ErrorHandling: config.ErrorHandling{MaxRetries: 1}}, hashStore: store, systemctl: writeFakeSystemctl(t, dir)}
	outputs := []Output{{Name: "template:app", Path: path, Services: []interface{}{"app"}}}
	for _, step := range []struct{ content, status, want string }{
		{"first", "", "--no-block try-restart app\n"},
		{"first", "", ""},
		{"second", "", "--no-block try-restart app\n"},
		{"second", "ActiveState=inactive\nSubState=dead\nResult=dependency\n", "--no-block start app\n"},
	} {
		write(path, step.content)
		if step.status != "" {
			write(statusPath, step.status)
		}
		write(logFile, "")
		if err := m.ProcessOutputChanges(outputs); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(logFile)
		if err != nil || string(got) != step.want {
			t.Fatalf("content %q: actions %q, want %q: %v", step.content, got, step.want, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "hashes.json"))
	if err != nil || !strings.Contains(string(data), path) {
		t.Fatalf("missing persisted template hash: %s, %v", data, err)
	}
}
