// Package escape provides POSIX-shell-safe quoting for values that will be
// printed as `export NAME=VALUE` and later eval'd by a shell.
package escape

import "strings"

// ShellSingleQuote wraps s in single quotes, escaping any embedded single
// quotes so the result is safe to eval in a POSIX shell regardless of
// content ($, `, ", newlines, etc. are all inert inside single quotes).
func ShellSingleQuote(s string) string {
	if !strings.Contains(s, "'") {
		return "'" + s + "'"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
