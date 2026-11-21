package config

import (
	"strings"
	"testing"
)

func TestRenderTemplate(t *testing.T) {
	marker := Placeholder("password")
	if marker != "<OPNIX:5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8>" {
		t.Fatalf("unexpected placeholder: %s", marker)
	}
	value := "line one\n" + Placeholder("unknown")
	got, err := RenderTemplate(marker+":"+marker+":"+Placeholder("empty"), map[string]string{
		"password": value,
		"empty":    "",
	})
	if err != nil || got != value+":"+value+":" {
		t.Fatalf("unexpected render result: error=%v", err)
	}
}

func TestRenderTemplateRejectsMarkersWithoutLeakingContent(t *testing.T) {
	for _, content := range []string{"sensitive <OPNIX:bad>", "sensitive <OPNIX:unfinished", "sensitive " + Placeholder("unknown"), "sensitive <OPNIX:>", "sensitive <OPNIX:" + strings.Repeat("a", 64) + ">>"} {
		_, err := RenderTemplate(content, nil)
		if err == nil || strings.Contains(err.Error(), "sensitive") {
			t.Fatalf("expected sanitized error, got %v", err)
		}
	}
}
