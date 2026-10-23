package cli

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points the env store at a throwaway folder so tests never touch the user's real data.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "envh-test-home")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Setenv("ENVH_HOME", home); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
