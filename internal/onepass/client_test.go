package onepass

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/1password/onepassword-sdk-go"
	opnixerrors "github.com/brizzbuzz/opnix/internal/errors"
)

type fakeSecretsAPI struct {
	resolve    func(context.Context, string) (string, error)
	resolveAll func(context.Context, []string) (onepassword.ResolveAllResponse, error)
}

func (f fakeSecretsAPI) Resolve(ctx context.Context, reference string) (string, error) {
	return f.resolve(ctx, reference)
}

func (f fakeSecretsAPI) ResolveAll(ctx context.Context, references []string) (onepassword.ResolveAllResponse, error) {
	if f.resolveAll != nil {
		return f.resolveAll(ctx, references)
	}
	responses := make(map[string]onepassword.Response[onepassword.ResolvedReference, onepassword.ResolveReferenceError], len(references))
	for _, reference := range references {
		secret, err := f.resolve(ctx, reference)
		if err != nil {
			return onepassword.ResolveAllResponse{}, err
		}
		responses[reference] = onepassword.Response[onepassword.ResolvedReference, onepassword.ResolveReferenceError]{
			Content: &onepassword.ResolvedReference{Secret: secret},
		}
	}
	return onepassword.ResolveAllResponse{IndividualResponses: responses}, nil
}

func TestGetToken(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "opnix-tests-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("environment token", func(t *testing.T) {
		expected := "ops_test_token"
		os.Setenv("OP_SERVICE_ACCOUNT_TOKEN", expected)
		defer os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")

		got, err := GetToken("")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if got != expected {
			t.Errorf("Expected token %q, got %q", expected, got)
		}
	})

	t.Run("file token", func(t *testing.T) {
		expected := "ops_test_token_from_file"
		tokenFile := filepath.Join(tmpDir, "token")
		if err := os.WriteFile(tokenFile, []byte(expected), 0600); err != nil {
			t.Fatalf("Failed to write token file: %v", err)
		}

		got, err := GetToken(tokenFile)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if got != expected {
			t.Errorf("Expected token %q, got %q", expected, got)
		}
	})

	t.Run("no token", func(t *testing.T) {
		os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")
		_, err := GetToken("")
		if err == nil {
			t.Error("Expected error when no token provided")
		}
	})

	t.Run("invalid token file", func(t *testing.T) {
		os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")
		_, err := GetToken("/nonexistent/file")
		if err == nil {
			t.Error("Expected error with invalid token file")
		}
	})
}

func TestNewClientRetriesTransientInitializationFailures(t *testing.T) {
	originalNewSDKSecrets := newSDKSecrets
	originalRetrySleep := retrySleep
	t.Cleanup(func() {
		newSDKSecrets = originalNewSDKSecrets
		retrySleep = originalRetrySleep
		os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")
	})

	os.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "ops_test_token")
	retrySleep = func(_ time.Duration) {}

	attempts := 0
	newSDKSecrets = func(_ context.Context, token string) (sdkSecretsAPI, error) {
		attempts++
		if token != "ops_test_token" {
			t.Fatalf("expected token to be passed through")
		}
		if attempts < 3 {
			return nil, stderrors.New("transient auth failure")
		}
		return fakeSecretsAPI{resolve: func(_ context.Context, reference string) (string, error) {
			return "resolved:" + reference, nil
		}}, nil
	}

	client, err := NewClient("")
	if err != nil {
		t.Fatalf("expected client initialization to succeed after retries: %v", err)
	}

	if attempts != 3 {
		t.Fatalf("expected 3 initialization attempts, got %d", attempts)
	}

	secret, err := client.ResolveSecret("op://vault/item/field")
	if err != nil {
		t.Fatalf("expected resolved secret, got error: %v", err)
	}

	if secret != "resolved:op://vault/item/field" {
		t.Fatalf("unexpected secret %q", secret)
	}
}

func TestResolveSecretRetriesTransientFailures(t *testing.T) {
	originalRetrySleep := retrySleep
	t.Cleanup(func() {
		retrySleep = originalRetrySleep
	})

	retrySleep = func(_ time.Duration) {}

	attempts := 0
	client := &Client{
		secrets: fakeSecretsAPI{resolve: func(_ context.Context, reference string) (string, error) {
			attempts++
			if reference != "op://vault/item/field" {
				t.Fatalf("unexpected reference %q", reference)
			}
			if attempts < 3 {
				return "", stderrors.New("temporary network failure")
			}
			return "secret-value", nil
		}},
	}

	secret, err := client.ResolveSecret("op://vault/item/field")
	if err != nil {
		t.Fatalf("expected secret resolution to succeed after retries: %v", err)
	}

	if attempts != 3 {
		t.Fatalf("expected 3 resolve attempts, got %d", attempts)
	}

	if secret != "secret-value" {
		t.Fatalf("unexpected secret value %q", secret)
	}
}

func TestResolveSecretDoesNotRetryMissingReference(t *testing.T) {
	originalRetrySleep := retrySleep
	t.Cleanup(func() {
		retrySleep = originalRetrySleep
	})

	retrySleep = func(_ time.Duration) {}

	attempts := 0
	client := &Client{
		secrets: fakeSecretsAPI{resolveAll: func(_ context.Context, references []string) (onepassword.ResolveAllResponse, error) {
			attempts++
			missing := onepassword.NewResolveReferenceErrorTypeVariantItemNotFound()
			return onepassword.ResolveAllResponse{IndividualResponses: map[string]onepassword.Response[onepassword.ResolvedReference, onepassword.ResolveReferenceError]{
				references[0]: {Error: &missing},
			}}, nil
		}},
	}

	_, err := client.ResolveSecret("op://vault/missing/field")
	if err == nil {
		t.Fatal("expected missing reference error")
	}

	if attempts != 1 {
		t.Fatalf("expected missing reference to be resolved once, got %d attempts", attempts)
	}

	if code := opnixerrors.ExitCode(err); code != opnixerrors.ExitCodeMissingReference {
		t.Fatalf("expected exit code %d, got %d", opnixerrors.ExitCodeMissingReference, code)
	}
}
