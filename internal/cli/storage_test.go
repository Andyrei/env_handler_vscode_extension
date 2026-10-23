package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectFilesAreIgnoredUntilAdopted(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, ".env.local", "A=1\n")

	if snap := snapshotOf(t, repo); len(snap.Environments) != 0 {
		t.Fatalf("project file must not be read from the repo: %+v", snap.Environments)
	}

	if _, errOut, code := runIn(t, repo, "adopt"); code != 0 {
		t.Fatalf("adopt failed (%d): %s", code, errOut)
	}
	snap := snapshotOf(t, repo)
	if len(snap.Environments) != 1 || snap.Environments[0].File != ".env.local" {
		t.Fatalf("adopted file should be listed: %+v", snap.Environments)
	}
	if _, err := os.Stat(filepath.Join(repo, ".env.local")); err != nil {
		t.Fatal("adopt must keep the project file")
	}
}

func TestPurgeNeedsConfirmationAndOnlyRemovesIdenticalCopies(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, ".env.local", "A=1\n")
	if _, errOut, code := runIn(t, repo, "adopt"); code != 0 {
		t.Fatalf("adopt failed (%d): %s", code, errOut)
	}

	if _, _, code := runIn(t, repo, "purge"); code != 1 {
		t.Fatalf("purge without --yes or --dry-run should exit 1, got %d", code)
	}
	if out, _, code := runIn(t, repo, "purge", "--dry-run"); code != 0 || out == "" {
		t.Fatalf("dry run failed (%d)", code)
	}
	if _, err := os.Stat(filepath.Join(repo, ".env.local")); err != nil {
		t.Fatal("dry run must not delete")
	}

	if _, errOut, code := runIn(t, repo, "purge", "--yes"); code != 0 {
		t.Fatalf("purge failed (%d): %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(repo, ".env.local")); !os.IsNotExist(err) {
		t.Fatal("identical project copy should be removed after confirmed purge")
	}
	if snap := snapshotOf(t, repo); len(snap.Environments) != 1 {
		t.Fatal("stored copy must survive purging the project copy")
	}
}
