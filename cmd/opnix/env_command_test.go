package main

import (
	"strings"
	"testing"
)

func TestParseEnvConfig_RejectsEmptyVars(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{name: "explicit empty vars", json: `{"vars": []}`},
		{name: "vars omitted", json: `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseEnvConfigString(tt.json)
			if err == nil {
				t.Fatal("expected error for empty vars")
			}
			if !strings.Contains(err.Error(), "at least one variable") {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestParseEnvConfigString_EmptyInput(t *testing.T) {
	_, err := parseEnvConfigString("   ")
	if err == nil {
		t.Fatal("expected error for empty JSON input")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnvProcessorProcess_AllowsEmptyConfig(t *testing.T) {
	t.Helper()

	cfg := &envConfig{}

	result, err := newEnvProcessor(staticResolver{}).Process(cfg)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(result.Values) != 0 {
		t.Fatalf("len(result.Values) = %d, want 0", len(result.Values))
	}

	if len(result.Skipped) != 0 {
		t.Fatalf("len(result.Skipped) = %d, want 0", len(result.Skipped))
	}
}

func TestEnvProcessorProcess_NilConfig(t *testing.T) {
	_, err := newEnvProcessor(staticResolver{}).Process(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "cannot be nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}
