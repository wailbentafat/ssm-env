package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/wailbentafat/ssm-env/internal/secretname"
)

// SecretsManagerClient is the subset of the Secrets Manager API this
// package needs, so callers can substitute a fake in tests instead of
// talking to real AWS.
type SecretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// SecretsManager is a SecretProvider backed by AWS Secrets Manager. Each
// secret in SecretIDs is fetched individually: a plain string secret
// becomes one env var (see secretname.FromSecretID), a JSON object secret
// becomes one env var per key (see secretname.FromSecretKey).
type SecretsManager struct {
	Client    SecretsManagerClient
	SecretIDs []string
}

func (s SecretsManager) Fetch(ctx context.Context) (map[string]string, error) {
	secrets := make(map[string]string, len(s.SecretIDs))

	for _, id := range s.SecretIDs {
		resp, err := s.Client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: &id})
		if err != nil {
			return nil, fmt.Errorf("fetching secret %q: %w", id, err)
		}
		if resp.SecretString == nil {
			return nil, fmt.Errorf("fetching secret %q: binary secrets are not supported", id)
		}

		prefix := secretname.FromSecretID(id)

		var asObject map[string]any
		if err := json.Unmarshal([]byte(*resp.SecretString), &asObject); err == nil {
			for key, value := range asObject {
				secrets[secretname.FromSecretKey(prefix, key)] = fmt.Sprint(value)
			}
			continue
		}

		secrets[prefix] = *resp.SecretString
	}

	return secrets, nil
}
