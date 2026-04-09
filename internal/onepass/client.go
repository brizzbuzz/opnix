package onepass

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/1password/onepassword-sdk-go"
	"github.com/brizzbuzz/opnix/internal/errors"
)

const (
	defaultInitAttempts    = 3
	defaultResolveAttempts = 3
)

type sdkSecretsAPI interface {
	Resolve(context.Context, string) (string, error)
}

type Client struct {
	secrets sdkSecretsAPI
}

var (
	newSDKSecrets = func(ctx context.Context, token string) (sdkSecretsAPI, error) {
		client, err := onepassword.NewClient(
			ctx,
			onepassword.WithServiceAccountToken(token),
			onepassword.WithIntegrationInfo("NixOS Secrets Integration", "v1.0.0"),
		)
		if err != nil {
			return nil, err
		}

		return client.Secrets(), nil
	}
	retrySleep = time.Sleep
)

// GetToken retrieves token from environment or file
func GetToken(tokenFile string) (string, error) {
	// First try environment variable
	if token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN"); token != "" {
		return token, nil
	}

	// Then try token file
	if tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			return "", errors.TokenError(
				fmt.Sprintf("Failed to read token file: %s", err.Error()),
				tokenFile,
				err,
			)
		}
		token := strings.TrimSpace(string(data))
		if len(token) == 0 {
			return "", errors.TokenError(
				"Token file is empty",
				tokenFile,
				nil,
			)
		}
		return token, nil
	}

	return "", errors.TokenError(
		"No token provided - neither OP_SERVICE_ACCOUNT_TOKEN environment variable nor token file specified",
		tokenFile,
		nil,
	)
}

func NewClient(tokenFile string) (*Client, error) {
	token, err := GetToken(tokenFile)
	if err != nil {
		return nil, err
	}

	secretsAPI, err := retryOperation(defaultInitAttempts, "Initializing 1Password client", func() (sdkSecretsAPI, error) {
		return newSDKSecrets(context.Background(), token)
	})
	if err != nil {
		return nil, errors.OnePasswordError(
			"Initializing 1Password client",
			"Failed to create 1Password SDK client - check token validity",
			err,
		)
	}

	return &Client{secrets: secretsAPI}, nil
}

func (c *Client) ResolveSecret(reference string) (string, error) {
	secret, err := retryOperation(defaultResolveAttempts, fmt.Sprintf("Resolving 1Password reference %s", reference), func() (string, error) {
		return c.secrets.Resolve(context.Background(), reference)
	})
	if err != nil {
		return "", errors.OnePasswordError(
			"Resolving 1Password secret",
			fmt.Sprintf("Failed to resolve reference: %s", reference),
			err,
		)
	}
	return secret, nil
}

func retryOperation[T any](attempts int, operation string, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		value, err := fn()
		if err == nil {
			return value, nil
		}

		lastErr = err
		if attempt == attempts {
			break
		}

		fmt.Fprintf(os.Stderr, "WARNING: %s attempt %d/%d failed: %v\n", operation, attempt, attempts, err)
		retrySleep(time.Duration(attempt) * time.Second)
	}

	return zero, fmt.Errorf("%s failed after %d attempts: %w", operation, attempts, lastErr)
}
