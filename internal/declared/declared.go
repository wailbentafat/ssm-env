// Package declared builds the set of environment variable names already
// present in a process's environment, so SSM parameters can be filtered
// down to only the names a container has already declared it wants
// (e.g. via Docker Compose's `environment: - VAR_NAME` no-value syntax).
package declared

import "strings"

// Names returns the set of variable names found in environ (the
// "KEY=VALUE" strings os.Environ() returns), ignoring values.
func Names(environ []string) map[string]struct{} {
	set := make(map[string]struct{}, len(environ))
	for _, kv := range environ {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			set[kv[:i]] = struct{}{}
		}
	}
	return set
}
