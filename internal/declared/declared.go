// Package declared builds the set of environment variable names already
// present in a process's environment, so SSM parameters can be filtered
// down to only the names a container has already declared it wants.
//
// Callers relying on Docker Compose to pre-declare names must use the
// trailing-`=` form (`- DB_HOST=`), not the bare form (`- DB_HOST`).
// Compose only creates the variable in the container for the `=` form;
// the bare form looks the name up on the host and, if absent there, omits
// it from the container's environment entirely -- Names then never sees
// it, since Go's os.Environ() only reflects what the OS actually handed
// this process. See README.md, "Only exporting declared variables".
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
