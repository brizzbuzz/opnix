package onepass

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/1password/onepassword-sdk-go"
	opnixerrors "github.com/brizzbuzz/opnix/internal/errors"
)

type fakeSecretsAPI struct {
	resolve    func(context.Context, string) (string, error)
	resolveAll func(context.Context, []string) (onepassword.ResolveAllResponse, error)
}

type fakeVaultsAPI struct {
	list func(context.Context, ...onepassword.VaultListParams) ([]onepassword.VaultOverview, error)
}

func (f fakeVaultsAPI) List(ctx context.Context, params ...onepassword.VaultListParams) ([]onepassword.VaultOverview, error) {
	return f.list(ctx, params...)
}

type fakeItemsAPI struct {
	list func(context.Context, string, ...onepassword.ItemListFilter) ([]onepassword.ItemOverview, error)
	get  func(context.Context, string, string) (onepassword.Item, error)
}

func (f fakeItemsAPI) List(ctx context.Context, vaultID string, filters ...onepassword.ItemListFilter) ([]onepassword.ItemOverview, error) {
	return f.list(ctx, vaultID, filters...)
}

func (f fakeItemsAPI) Get(ctx context.Context, vaultID, itemID string) (onepassword.Item, error) {
	return f.get(ctx, vaultID, itemID)
}

type fakeFilesAPI struct {
	read func(context.Context, string, string, onepassword.FileAttributes) ([]byte, error)
}

func (f fakeFilesAPI) Read(ctx context.Context, vaultID, itemID string, attr onepassword.FileAttributes) ([]byte, error) {
	return f.read(ctx, vaultID, itemID, attr)
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
	originalNewSDKClient := newSDKClient
	originalRetrySleep := retrySleep
	t.Cleanup(func() {
		newSDKClient = originalNewSDKClient
		retrySleep = originalRetrySleep
		os.Unsetenv("OP_SERVICE_ACCOUNT_TOKEN")
	})

	os.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "ops_test_token")
	retrySleep = func(_ time.Duration) {}

	attempts := 0
	newSDKClient = func(_ context.Context, token string) (sdkAPIs, error) {
		attempts++
		if token != "ops_test_token" {
			t.Fatalf("expected token to be passed through")
		}
		if attempts < 3 {
			return sdkAPIs{}, stderrors.New("transient auth failure")
		}
		return sdkAPIs{secrets: fakeSecretsAPI{resolve: func(_ context.Context, reference string) (string, error) {
			return "resolved:" + reference, nil
		}}}, nil
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

func TestResolveSecretDoesNotRetryRateLimit(t *testing.T) {
	attempts := 0
	client := &Client{
		secrets: fakeSecretsAPI{resolveAll: func(_ context.Context, _ []string) (onepassword.ResolveAllResponse, error) {
			attempts++
			return onepassword.ResolveAllResponse{}, &onepassword.RateLimitExceededError{}
		}},
	}

	_, err := client.ResolveSecret("op://vault/item/field")
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if attempts != 1 {
		t.Fatalf("expected rate-limited reference to be attempted once, got %d attempts", attempts)
	}
	if code := opnixerrors.ExitCode(err); code != opnixerrors.ExitCodeRateLimited {
		t.Fatalf("expected exit code %d, got %d", opnixerrors.ExitCodeRateLimited, code)
	}
}

func TestResolveFilesReadsDocumentsAndAttachments(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		vault     onepassword.VaultOverview
		overview  onepassword.ItemOverview
		item      onepassword.Item
		wantAttr  onepassword.FileAttributes
	}{
		{
			name:      "document by titles",
			reference: "op://Personal/SSH%20Key/id_ed25519",
			vault:     onepassword.VaultOverview{ID: "vault-id", Title: "Personal"},
			overview:  onepassword.ItemOverview{ID: "document-id", Title: "SSH Key"},
			item: onepassword.Item{ID: "document-id", Title: "SSH Key", Document: &onepassword.FileAttributes{
				ID: "document-file-id", Name: "id_ed25519", Size: 6,
			}},
			wantAttr: onepassword.FileAttributes{ID: "document-file-id", Name: "id_ed25519", Size: 6},
		},
		{
			name:      "attachment by vault and item IDs",
			reference: "op://vault-id/item-id/client.p12",
			vault:     onepassword.VaultOverview{ID: "vault-id", Title: "Certificates"},
			overview:  onepassword.ItemOverview{ID: "item-id", Title: "Client Certificate"},
			item: onepassword.Item{ID: "item-id", Title: "Client Certificate", Files: []onepassword.ItemFile{{
				Attributes: onepassword.FileAttributes{ID: "attachment-id", Name: "client.p12", Size: 6},
			}}},
			wantAttr: onepassword.FileAttributes{ID: "attachment-id", Name: "client.p12", Size: 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := []byte{0x00, 0xff, 0xfe, 0x80, 0x01, '\n'}
			client := &Client{
				vaults: fakeVaultsAPI{list: func(context.Context, ...onepassword.VaultListParams) ([]onepassword.VaultOverview, error) {
					return []onepassword.VaultOverview{tt.vault}, nil
				}},
				items: fakeItemsAPI{
					list: func(_ context.Context, vaultID string, _ ...onepassword.ItemListFilter) ([]onepassword.ItemOverview, error) {
						if vaultID != tt.vault.ID {
							t.Fatalf("Unexpected vault ID %q", vaultID)
						}
						return []onepassword.ItemOverview{tt.overview}, nil
					},
					get: func(_ context.Context, vaultID, itemID string) (onepassword.Item, error) {
						if vaultID != tt.vault.ID || itemID != tt.overview.ID {
							t.Fatalf("Unexpected item lookup %q/%q", vaultID, itemID)
						}
						return tt.item, nil
					},
				},
				files: fakeFilesAPI{read: func(_ context.Context, vaultID, itemID string, attr onepassword.FileAttributes) ([]byte, error) {
					if vaultID != tt.vault.ID || itemID != tt.overview.ID || attr != tt.wantAttr {
						t.Fatalf("Unexpected file read %q/%q %#v", vaultID, itemID, attr)
					}
					return want, nil
				}},
			}

			resolved, err := client.ResolveFiles([]string{tt.reference})
			if err != nil {
				t.Fatalf("Failed to resolve file: %v", err)
			}
			if got := resolved[tt.reference]; string(got) != string(want) {
				t.Fatalf("File bytes changed: got %v, want %v", got, want)
			}
		})
	}
}

func TestResolveFilesClassifiesSelectorFailures(t *testing.T) {
	t.Run("ambiguous vault", func(t *testing.T) {
		client := &Client{vaults: fakeVaultsAPI{list: func(context.Context, ...onepassword.VaultListParams) ([]onepassword.VaultOverview, error) {
			return []onepassword.VaultOverview{{ID: "one", Title: "Shared"}, {ID: "two", Title: "Shared"}}, nil
		}}}
		_, err := client.ResolveFiles([]string{"op://Shared/Item/file.bin"})
		if err == nil || opnixerrors.ExitCode(err) != opnixerrors.ExitCodeMissingReference {
			t.Fatalf("Expected ambiguous reference exit %d, got %v", opnixerrors.ExitCodeMissingReference, err)
		}
	})

	t.Run("rate limit", func(t *testing.T) {
		attempts := 0
		client := &Client{vaults: fakeVaultsAPI{list: func(context.Context, ...onepassword.VaultListParams) ([]onepassword.VaultOverview, error) {
			attempts++
			return nil, &onepassword.RateLimitExceededError{}
		}}}
		_, err := client.ResolveFiles([]string{"op://Vault/Item/file.bin"})
		if err == nil || opnixerrors.ExitCode(err) != opnixerrors.ExitCodeRateLimited {
			t.Fatalf("Expected rate-limit exit %d, got %v", opnixerrors.ExitCodeRateLimited, err)
		}
		if attempts != 1 {
			t.Fatalf("Expected one rate-limited attempt, got %d", attempts)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		client := &Client{
			vaults: fakeVaultsAPI{list: func(context.Context, ...onepassword.VaultListParams) ([]onepassword.VaultOverview, error) {
				return []onepassword.VaultOverview{{ID: "vault-id", Title: "Vault"}}, nil
			}},
			items: fakeItemsAPI{
				list: func(context.Context, string, ...onepassword.ItemListFilter) ([]onepassword.ItemOverview, error) {
					return []onepassword.ItemOverview{{ID: "item-id", Title: "Item"}}, nil
				},
				get: func(context.Context, string, string) (onepassword.Item, error) {
					return onepassword.Item{ID: "item-id", Title: "Item"}, nil
				},
			},
		}
		_, err := client.ResolveFiles([]string{"op://Vault/Item/missing.bin"})
		if err == nil || opnixerrors.ExitCode(err) != opnixerrors.ExitCodeMissingReference {
			t.Fatalf("Expected missing reference exit %d, got %v", opnixerrors.ExitCodeMissingReference, err)
		}
	})

	t.Run("invalid reference", func(t *testing.T) {
		client := &Client{vaults: fakeVaultsAPI{list: func(context.Context, ...onepassword.VaultListParams) ([]onepassword.VaultOverview, error) {
			return nil, nil
		}}}
		_, err := client.ResolveFiles([]string{"op://Vault/Item/Section/file.bin"})
		if err == nil || opnixerrors.ExitCode(err) != opnixerrors.ExitCodeMissingReference {
			t.Fatalf("Expected invalid reference exit %d, got %v", opnixerrors.ExitCodeMissingReference, err)
		}
	})
}

func TestUnsupportedFileFormatSuggestsFileKind(t *testing.T) {
	sdkErr := onepassword.NewResolveReferenceErrorTypeVariantUnsupportedFileFormat()
	err := classifyResolveReferenceError("op://Vault/Item/file.bin", sdkErr)
	if !strings.Contains(err.Error(), `kind = "file"`) {
		t.Fatalf("Expected file-kind guidance, got %v", err)
	}
}
