// Package config reads ssm-env's configuration from environment variables
// into a plain struct, so the rest of the program depends on a typed value
// instead of scattering os.Getenv calls through business logic.
package config

import (
	"strconv"
	"strings"
)

const (
	PathEnvVar         = "AWS_ENV_PATH"
	SecretIDsEnvVar    = "AWS_ENV_SECRET_IDS"
	BackendEnvVar      = "AWS_ENV_BACKEND"
	OnlyDeclaredEnvVar = "AWS_ENV_ONLY_DECLARED"
)

const (
	BackendSSM            = "ssm"
	BackendSecretsManager = "secretsmanager"
	BackendBoth           = "both"
)

// Config is ssm-env's resolved configuration, independent of how it was
// sourced (env vars today, flags or a file in the future).
type Config struct {
	// Path is the SSM Parameter Store path prefix to fetch from.
	Path string
	// SecretIDs is the list of Secrets Manager secret names/ARNs to fetch.
	SecretIDs []string
	// Backend selects which provider(s) to fetch from: "ssm" (default),
	// "secretsmanager", or "both".
	Backend string
	// OnlyDeclared, if true, restricts exported names to those already
	// present in the process environment.
	OnlyDeclared bool
}

// Load reads Config from environ (the "KEY=VALUE" strings os.Environ()
// returns), applying ssm-env's defaults for unset variables.
func Load(environ []string) Config {
	values := make(map[string]string, len(environ))
	for _, kv := range environ {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			values[kv[:i]] = kv[i+1:]
		}
	}

	cfg := Config{
		Path:         values[PathEnvVar],
		SecretIDs:    splitCSV(values[SecretIDsEnvVar]),
		Backend:      values[BackendEnvVar],
		OnlyDeclared: parseBool(values[OnlyDeclaredEnvVar]),
	}
	if cfg.Backend == "" {
		cfg.Backend = BackendSSM
	}
	return cfg
}

// splitCSV splits a comma-separated list, trimming whitespace around each
// item and dropping empty items (so a trailing comma or extra spaces don't
// produce a bogus secret ID).
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func parseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
