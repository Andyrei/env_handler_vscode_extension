package cli

import (
	"os"
	"strings"
	"testing"

	"env-handler/internal/store"
)

func TestExportLayersAndQuotes(t *testing.T) {
	repo := t.TempDir()
	if _, errOut, code := runIn(t, repo, "create", "Local", "--activate"); code != 0 {
		t.Fatalf("create failed (%d): %s", code, errOut)
	}
	writeEnv(t, repo, ".env", "BASE=base\nSHARED=base\n")
	writeEnv(t, repo, ".env.local", "SHARED=local\nQUOTED=it's\nLOCK=encrypted:abc\nDOTTED.KEY=no\n")

	out, errOut, code := runIn(t, repo, "export")
	if code != 0 {
		t.Fatalf("export failed (%d): %s", code, errOut)
	}
	want := strings.Join([]string{
		"export BASE='base'",
		"export LOCK='encrypted:abc'",
		"export QUOTED='it'\\''s'",
		"export SHARED='local'",
		"",
	}, "\n")
	if out != want {
		t.Fatalf("export output:\n%s\nwant:\n%s", out, want)
	}
	if !strings.Contains(errOut, "warning: LOCK is encrypted") {
		t.Fatalf("missing encrypted warning: %q", errOut)
	}
	if !strings.Contains(errOut, "warning: skipping DOTTED.KEY") {
		t.Fatalf("missing unsafe-name warning: %q", errOut)
	}
	if strings.Contains(out, "encrypted:abc") && strings.Contains(errOut, "encrypted:abc") {
		t.Fatal("encrypted value leaked into the warning")
	}
}

func TestExportSkipEncrypted(t *testing.T) {
	repo := t.TempDir()
	if _, errOut, code := runIn(t, repo, "create", "Local", "--activate"); code != 0 {
		t.Fatalf("create failed (%d): %s", code, errOut)
	}
	writeEnv(t, repo, ".env.local", "LOCK=encrypted:abc\nOK=yes\n")

	out, errOut, code := runIn(t, repo, "export", "--skip-encrypted")
	if code != 0 {
		t.Fatalf("export failed (%d): %s", code, errOut)
	}
	if strings.Contains(out, "LOCK") {
		t.Fatalf("encrypted key was exported:\n%s", out)
	}
	if out != "export OK='yes'\n" {
		t.Fatalf("export output = %q", out)
	}
	if !strings.Contains(errOut, "warning: skipping encrypted LOCK") {
		t.Fatalf("missing skip warning: %q", errOut)
	}
}

func TestExportFromAnotherBranch(t *testing.T) {
	repo := t.TempDir()
	if _, errOut, code := runIn(t, repo, "--branch", "feature/x", "create", "Feature", "--activate"); code != 0 {
		t.Fatalf("create failed (%d): %s", code, errOut)
	}
	loc, err := store.Locate(repo, "feature/x")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, loc.Dir, ".env.feature", "ONLY=feature\n")

	out, errOut, code := runIn(t, repo, "--branch", "feature/x", "export", "--env", ".env.feature")
	if code != 0 {
		t.Fatalf("export failed (%d): %s", code, errOut)
	}
	if out != "export ONLY='feature'\n" {
		t.Fatalf("export output = %q", out)
	}
}

func TestRunLoadsLayeredValues(t *testing.T) {
	repo := t.TempDir()
	if _, errOut, code := runIn(t, repo, "create", "Local", "--activate"); code != 0 {
		t.Fatalf("create failed (%d): %s", code, errOut)
	}
	writeEnv(t, repo, ".env", "BASE=base\nSHARED=base\n")
	writeEnv(t, repo, ".env.local", "SHARED=local\nLOCK=encrypted:abc\n")

	t.Setenv("GO_WANT_CLI_HELPER", "1")
	t.Setenv("CLI_HELPER_MODE", "print")
	t.Setenv("SHARED", "parent")

	out, errOut, code := runIn(t, repo, "run", "--", os.Args[0], "-test.run=TestCLIHelperProcess")
	if code != 0 {
		t.Fatalf("run failed (%d): %s", code, errOut)
	}
	if strings.TrimSpace(out) != "local" {
		t.Fatalf("run stdout = %q, want local", out)
	}
	if !strings.Contains(errOut, "warning: LOCK is encrypted") {
		t.Fatalf("missing encrypted warning: %q", errOut)
	}
	if strings.Contains(errOut, "encrypted:abc") {
		t.Fatalf("encrypted value leaked: %q", errOut)
	}
}

func TestRunNoOverrideLetsParentWin(t *testing.T) {
	repo := t.TempDir()
	if _, errOut, code := runIn(t, repo, "create", "Local", "--activate"); code != 0 {
		t.Fatalf("create failed (%d): %s", code, errOut)
	}
	writeEnv(t, repo, ".env.local", "SHARED=file\n")

	t.Setenv("GO_WANT_CLI_HELPER", "1")
	t.Setenv("CLI_HELPER_MODE", "print")
	t.Setenv("SHARED", "parent")

	out, errOut, code := runIn(t, repo, "run", "--no-override", "--", os.Args[0], "-test.run=TestCLIHelperProcess")
	if code != 0 {
		t.Fatalf("run failed (%d): %s", code, errOut)
	}
	if strings.TrimSpace(out) != "parent" {
		t.Fatalf("run stdout = %q, want parent", out)
	}
}

func TestRunReturnsChildExitCode(t *testing.T) {
	repo := t.TempDir()
	if _, errOut, code := runIn(t, repo, "create", "Local", "--activate"); code != 0 {
		t.Fatalf("create failed (%d): %s", code, errOut)
	}
	writeEnv(t, repo, ".env.local", "OK=1\n")

	t.Setenv("GO_WANT_CLI_HELPER", "1")
	t.Setenv("CLI_HELPER_MODE", "exit")

	_, errOut, code := runIn(t, repo, "run", "--", os.Args[0], "-test.run=TestCLIHelperProcess")
	if code != 9 {
		t.Fatalf("exit code = %d, want 9 (stderr: %s)", code, errOut)
	}
}

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_CLI_HELPER") != "1" {
		return
	}
	switch os.Getenv("CLI_HELPER_MODE") {
	case "print":
		_, _ = os.Stdout.WriteString(os.Getenv("SHARED") + "\n")
		os.Exit(0)
	case "exit":
		os.Exit(9)
	default:
		os.Exit(0)
	}
}
