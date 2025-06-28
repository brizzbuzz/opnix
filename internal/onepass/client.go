package onepass

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/1password/onepassword-sdk-go"
	"github.com/brizzbuzz/opnix/internal/errors"
)

type Client struct {
	client *onepassword.Client
}

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

	client, err := onepassword.NewClient(
		context.Background(),
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("NixOS Secrets Integration", "v1.0.0"),
	)
	if err != nil {
		return nil, errors.OnePasswordError(
			"Initializing 1Password client",
			"Failed to create 1Password SDK client - check token validity",
			err,
		)
	}

	return &Client{client: client}, nil
}

func (c *Client) ResolveSecret(reference string) (string, error) {
	secret, err := c.client.Secrets().Resolve(context.Background(), reference)
	if err != nil {
		return "", errors.OnePasswordError(
			"Resolving 1Password secret",
			fmt.Sprintf("Failed to resolve reference: %s", reference),
			err,
		)
	}
	return secret, nil
}
