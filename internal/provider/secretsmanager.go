package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/wailbentafat/ssm-env/internal/secretname"
)

// SecretsManagerClient is the subset of the Secrets Manager API this
// package needs, so callers can substitute a fake in tests instead of
// talking to real AWS.
type SecretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// RDSClient is the subset of the RDS API needed to resolve a "rds:"-prefixed
// entry (see SecretsManager.SecretIDs) into the secret ARN it currently
// points to, so callers can substitute a fake in tests.
type RDSClient interface {
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

// rdsIDPrefix marks a SecretIDs entry as a DB instance identifier rather
// than a secret ID/ARN directly - e.g. "rds:db-bricoram-staging|POSTGRES"
// resolves to whatever secret ARN that database's managed master-user
// credentials currently live at, instead of requiring every developer to
// know and keep in sync the AWS-auto-generated secret name/ARN.
const rdsIDPrefix = "rds:"

// SecretsManager is a SecretProvider backed by AWS Secrets Manager. Each
// secret in SecretIDs is fetched individually: a plain string secret
// becomes one env var (see secretname.FromSecretID), a JSON object secret
// becomes one env var per key (see secretname.FromSecretKey).
//
// An entry may include an explicit prefix override as "secret-id|PREFIX",
// which replaces the auto-derived prefix from secretname.FromSecretID. This
// matters for AWS-auto-named secrets - e.g. an RDS-managed master user
// secret is named "rds!db-<uuid>", which auto-derives to a useless prefix;
// "rds!db-<uuid>|POSTGRES" produces POSTGRES_PASSWORD, POSTGRES_HOST, etc.
// instead.
//
// An entry may also be given as "rds:<db-instance-identifier>" (optionally
// with the same "|PREFIX" override) instead of a literal secret ID/ARN. In
// that case RDSClient is used to look up the database's current managed
// secret ARN before fetching it - see rdsIDPrefix.
type SecretsManager struct {
	Client    SecretsManagerClient
	RDSClient RDSClient
	SecretIDs []string
}

func (s SecretsManager) Fetch(ctx context.Context) (map[string]string, error) {
	secrets := make(map[string]string, len(s.SecretIDs))

	for _, raw := range s.SecretIDs {
		rawID, prefixOverride := splitPrefixOverride(raw)

		id, err := s.resolveSecretID(ctx, rawID)
		if err != nil {
			return nil, err
		}

		resp, err := s.Client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: &id})
		if err != nil {
			return nil, fmt.Errorf("fetching secret %q: %w", id, err)
		}
		if resp.SecretString == nil {
			return nil, fmt.Errorf("fetching secret %q: binary secrets are not supported", id)
		}

		prefix := prefixOverride
		if prefix == "" {
			prefix = secretname.FromSecretID(id)
		}

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

// splitPrefixOverride splits "secret-id|PREFIX" into its two parts. Secret
// ARNs/names never contain "|", so the last "|" (if any) unambiguously
// separates an explicit prefix override from the id itself.
func splitPrefixOverride(raw string) (id, prefixOverride string) {
	if i := strings.LastIndexByte(raw, '|'); i >= 0 {
		return raw[:i], raw[i+1:]
	}
	return raw, ""
}

// resolveSecretID returns rawID unchanged unless it carries the rdsIDPrefix,
// in which case it's treated as a DB instance identifier and resolved to
// that database's current managed master-user secret ARN via RDS.
func (s SecretsManager) resolveSecretID(ctx context.Context, rawID string) (string, error) {
	instanceID, ok := strings.CutPrefix(rawID, rdsIDPrefix)
	if !ok {
		return rawID, nil
	}

	if s.RDSClient == nil {
		return "", fmt.Errorf("resolving %q: no RDS client configured", rawID)
	}

	out, err := s.RDSClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: &instanceID,
	})
	if err != nil {
		return "", fmt.Errorf("describing RDS instance %q: %w", instanceID, err)
	}
	if len(out.DBInstances) == 0 {
		return "", fmt.Errorf("RDS instance %q not found", instanceID)
	}

	secret := out.DBInstances[0].MasterUserSecret
	if secret == nil || secret.SecretArn == nil {
		return "", fmt.Errorf("RDS instance %q has no managed master user secret (is manage_master_user_password enabled?)", instanceID)
	}

	return *secret.SecretArn, nil
}
