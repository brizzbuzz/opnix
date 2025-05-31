package onepass

import (
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/1password/onepassword-sdk-go"
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
            return "", fmt.Errorf("failed to read token file: %w", err)
        }
        token := strings.TrimSpace(string(data))
        if token != "" {
            return token, nil
        }
    }

    return "", fmt.Errorf("no token provided")
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
        return nil, fmt.Errorf("error initializing client: %w", err)
    }

    return &Client{client: client}, nil
}

func (c *Client) ResolveSecret(reference string) (string, error) {
    return c.client.Secrets().Resolve(context.Background(), reference)
}
