// Package shell formats POSIX shell exports for `envh export`.
package shell

import (
	"regexp"
	"strings"
)

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidName reports whether name can be assigned with `export name=...`.
func ValidName(name string) bool {
	return identifier.MatchString(name)
}

// Quote wraps value in single quotes, which keeps every byte literal in a
// POSIX shell. Embedded single quotes are closed, escaped, and reopened.
func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// Export returns one shell statement for key. The caller must check ValidName.
func Export(key, value string) string {
	return "export " + key + "=" + Quote(value)
}
