package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateLoadAndMergedNames(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	a := write("a.json", `{"secrets":[{"name":"one","path":"a","reference":"op://Example/Service/a"}],"templates":[{"name":"app","path":"output","content":""}]}`)
	b := write("b.json", `{"secrets":[{"name":"two","path":"b","reference":"op://Example/Service/b"}],"templates":[{"name":"other","path":"output2","content":"x"}]}`)
	c, err := LoadMultiple([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Templates) != 2 || c.Templates[0].Content != "" || c.Secrets[0].Name != "one" {
		t.Fatalf("unexpected config: %#v", c)
	}
	duplicate := write("duplicate.json", `{"secrets":[{"name":"one","path":"b","reference":"op://Example/Service/b"}]}`)
	if _, err := LoadMultiple([]string{a, duplicate}); err == nil {
		t.Fatal("expected duplicate name error")
	}
	missing := write("missing.json", `{"secrets":[{"path":"a","reference":"op://Example/Service/a"}],"templates":[{"name":"x","path":"y"}]}`)
	if _, err := Load(missing); err == nil {
		t.Fatal("expected missing content error")
	}
	legacy := write("legacy.json", `{"secrets":[{"path":"a","reference":"op://Example/Service/a"}]}`)
	if c, err := Load(legacy); err != nil || c.Secrets[0].Name != "" {
		t.Fatalf("legacy config: %#v, %v", c, err)
	}
	duplicateTemplate := write("duplicate-template.json", `{"templates":[{"name":"app","path":"different","content":"x"}]}`)
	if _, err := LoadMultiple([]string{a, duplicateTemplate}); err == nil {
		t.Fatal("expected duplicate template name error")
	}
	file := write("file.json", `{"secrets":[{"name":"file","kind":"file","path":"a","reference":"op://Example/Service/a"}],"templates":[{"name":"app","path":"b","content":"`+Placeholder("file")+`"}]}`)
	if _, err := Load(file); err == nil {
		t.Fatal("expected file secret marker rejection")
	}
}
