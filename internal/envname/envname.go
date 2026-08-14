// Package envname derives a shell environment variable name from an SSM
// parameter's full path.
package envname

import "strings"

// FromParam strips prefix from paramName, trims any leftover leading slash,
// and replaces remaining slashes with underscores, so "/staging/DB_HOST"
// under prefix "/staging/" becomes "DB_HOST", and "/staging/db/password"
// (a nested parameter) becomes "db_password" rather than the invalid shell
// identifier "db/password".
func FromParam(paramName, prefix string) string {
	name := strings.TrimPrefix(paramName, prefix)
	name = strings.TrimPrefix(name, "/")
	return strings.ReplaceAll(name, "/", "_")
}
