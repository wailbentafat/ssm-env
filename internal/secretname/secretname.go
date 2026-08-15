// Package secretname derives environment variable names from AWS Secrets
// Manager secret names and, for JSON secrets, their keys.
package secretname

import "strings"

// FromSecretID derives the env var prefix for a Secrets Manager secret from
// its name or ARN: the last "/"-separated path segment, with "-" replaced
// by "_", uppercased. "prod/db-password" becomes "DB_PASSWORD";
// "prod/database" becomes "DATABASE".
func FromSecretID(id string) string {
	if i := strings.LastIndexByte(id, '/'); i >= 0 {
		id = id[i+1:]
	}
	id = strings.ReplaceAll(id, "-", "_")
	return strings.ToUpper(id)
}

// FromSecretKey derives an env var name for a single key of a JSON secret,
// by joining the secret's prefix (see FromSecretID) with the key name:
// prefix "DATABASE" and key "host" becomes "DATABASE_HOST".
func FromSecretKey(prefix, key string) string {
	key = strings.ReplaceAll(key, "-", "_")
	return prefix + "_" + strings.ToUpper(key)
}
