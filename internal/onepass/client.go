package onepass

import (
	"context"
	stderrors "errors"
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
	ResolveAll(context.Context, []string) (onepassword.ResolveAllResponse, error)
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
	secrets, err := c.ResolveSecrets([]string{reference})
	if err != nil {
		return "", err
	}
	return secrets[reference], nil
}

func (c *Client) ResolveSecrets(references []string) (map[string]string, error) {
	response, err := retryOperation(defaultResolveAttempts, "Resolving 1Password references", func() (onepassword.ResolveAllResponse, error) {
		return c.secrets.ResolveAll(context.Background(), references)
	}, shouldRetryProviderError)
	if err != nil {
		return nil, errors.OnePasswordError(
			"Resolving 1Password secrets",
			"Failed to resolve 1Password references",
			classifyTopLevelProviderError(err),
		)
	}

	resolved := make(map[string]string, len(references))
	var failures []*errors.ProviderError
	for _, reference := range references {
		individual, ok := response.IndividualResponses[reference]
		if !ok {
			failures = append(failures, &errors.ProviderError{
				Kind:      errors.ProviderErrorOther,
				Reference: reference,
				Issue:     "1Password did not return a result for this reference",
			})
			continue
		}

		if individual.Error != nil {
			failures = append(failures, classifyResolveReferenceError(reference, *individual.Error))
			continue
		}
		if individual.Content == nil {
			failures = append(failures, &errors.ProviderError{
				Kind:      errors.ProviderErrorOther,
				Reference: reference,
				Issue:     "1Password returned an empty result for this reference",
			})
			continue
		}

		resolved[reference] = individual.Content.Secret
	}

	if len(failures) > 0 {
		return nil, &errors.ProviderResolutionError{Failures: failures}
	}

	return resolved, nil
}

func retryOperation[T any](attempts int, operation string, fn func() (T, error), shouldRetry ...func(error) bool) (T, error) {
	var zero T
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		value, err := fn()
		if err == nil {
			return value, nil
		}

		lastErr = err
		if len(shouldRetry) > 0 && !shouldRetry[0](err) {
			break
		}
		if attempt == attempts {
			break
		}

		fmt.Fprintf(os.Stderr, "WARNING: %s attempt %d/%d failed: %v\n", operation, attempt, attempts, err)
		retrySleep(time.Duration(attempt) * time.Second)
	}

	return zero, fmt.Errorf("%s failed after %d attempts: %w", operation, attempts, lastErr)
}

func shouldRetryProviderError(err error) bool {
	var rateLimited *onepassword.RateLimitExceededError
	return !stderrors.As(err, &rateLimited)
}

func classifyTopLevelProviderError(err error) error {
	var rateLimited *onepassword.RateLimitExceededError
	if stderrors.As(err, &rateLimited) {
		return &errors.ProviderResolutionError{Failures: []*errors.ProviderError{{
			Kind:  errors.ProviderErrorRateLimited,
			Issue: "1Password rate limit exceeded; wait for the provider reset window before retrying",
			Cause: err,
		}}}
	}
	return &errors.ProviderResolutionError{Failures: []*errors.ProviderError{{
		Kind:  errors.ProviderErrorTransient,
		Issue: err.Error(),
		Cause: err,
	}}}
}

func classifyResolveReferenceError(reference string, err onepassword.ResolveReferenceError) *errors.ProviderError {
	switch err.Type {
	case onepassword.ResolveReferenceErrorTypeVariantVaultNotFound,
		onepassword.ResolveReferenceErrorTypeVariantItemNotFound,
		onepassword.ResolveReferenceErrorTypeVariantFieldNotFound,
		onepassword.ResolveReferenceErrorTypeVariantNoMatchingSections:
		return &errors.ProviderError{Kind: errors.ProviderErrorMissingReference, Reference: reference, Issue: string(err.Type)}
	case onepassword.ResolveReferenceErrorTypeVariantParsing:
		return &errors.ProviderError{Kind: errors.ProviderErrorInvalidReference, Reference: reference, Issue: string(err.Parsing())}
	case onepassword.ResolveReferenceErrorTypeVariantTooManyVaults,
		onepassword.ResolveReferenceErrorTypeVariantTooManyItems,
		onepassword.ResolveReferenceErrorTypeVariantTooManyMatchingFields:
		return &errors.ProviderError{Kind: errors.ProviderErrorAmbiguousReference, Reference: reference, Issue: string(err.Type)}
	default:
		return &errors.ProviderError{Kind: errors.ProviderErrorOther, Reference: reference, Issue: string(err.Type)}
	}
}
