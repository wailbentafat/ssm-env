// Package envname derives a shell environment variable name from an SSM
// parameter's full path.
package envname

import "strings"

// FromParam strips prefix from paramName and trims any leftover leading
// slash, so "/staging/DB_HOST" under prefix "/staging/" (or "/staging")
// becomes "DB_HOST".
func FromParam(paramName, prefix string) string {
	name := strings.TrimPrefix(paramName, prefix)
	return strings.TrimPrefix(name, "/")
}
