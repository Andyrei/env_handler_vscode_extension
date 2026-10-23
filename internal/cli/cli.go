package cli

import (
	"fmt"
	"io"

	"env-handler/internal/version"
)

// Run executes the CLI and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage())
		return 0
	}

	switch args[0] {
	case "--help", "-h", "help":
		fmt.Fprint(stdout, usage())
		return 0
	case "--version", "-v", "version":
		fmt.Fprintln(stdout, version.Version)
		return 0
	default:
		fmt.Fprintf(stderr, "envh: unknown command %q\n\n", args[0])
		fmt.Fprint(stderr, usage())
		return 1
	}
}

func usage() string {
	return fmt.Sprintf(`envh %s
Standalone environment manager for .env files.

Usage:
  envh <command> [options]

Phase 0 scaffold: commands are not implemented yet.
Global options:
  --help, -h       Show this help
  --version, -v    Show the version

Documentation:
  env-manager-plan.md
`, version.Version)
}
