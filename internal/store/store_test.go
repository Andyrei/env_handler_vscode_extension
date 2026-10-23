package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.test",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.test")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestLocateKeepsBranchFoldersOutsideRepo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ENVH_HOME", home)
	repo := t.TempDir()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	runGit(t, repo, "init")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "init")
	runGit(t, repo, "checkout", "-b", "feature/login")

	loc, err := Locate(repo, "")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if loc.Branch != "feature/login" {
		t.Fatalf("expected branch feature/login, got %q", loc.Branch)
	}
	if !strings.HasPrefix(loc.Dir, home) || strings.HasPrefix(loc.Dir, repo) {
		t.Fatalf("env dir must live under home, not the repo: %s", loc.Dir)
	}
	if filepath.Base(loc.Dir) != "feature%2Flogin" {
		t.Fatalf("branch folder should be escaped, got %s", filepath.Base(loc.Dir))
	}
}

func TestLocateWithoutGitUsesDefaultBranch(t *testing.T) {
	t.Setenv("ENVH_HOME", t.TempDir())
	loc, err := Locate(t.TempDir(), "")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if loc.Branch != "default" {
		t.Fatalf("expected default branch, got %q", loc.Branch)
	}
}

func TestAdoptOnlyCopies(t *testing.T) {
	t.Setenv("ENVH_HOME", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "packages", "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "packages", "api", ".env.local"), []byte("B=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	loc, err := Locate(repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Adopt(loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Copied) != 2 {
		t.Fatalf("expected 2 copies, got %v", report.Copied)
	}
	data, err := os.ReadFile(filepath.Join(loc.Dir, "packages", "api", ".env.local"))
	if err != nil || string(data) != "B=2\n" {
		t.Fatalf("nested env not copied: %q %v", data, err)
	}

	// Adopt never deletes project files, even after they are stored.
	if _, err := os.Stat(filepath.Join(repo, ".env")); err != nil {
		t.Fatal("adopt must not delete project files")
	}
	again, err := Adopt(loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Same) != 2 || len(again.Copied) != 0 {
		t.Fatalf("second adopt should see identical copies: %+v", again)
	}
}

func TestPurgeDeletesOnlyIdenticalCopies(t *testing.T) {
	t.Setenv("ENVH_HOME", t.TempDir())
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env.local"), []byte("B=project\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env.new"), []byte("C=3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	loc, err := Locate(repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Adopt(loc); err != nil {
		t.Fatal(err)
	}
	// .env.local differs in storage; .env.new is not stored at all.
	if err := os.WriteFile(filepath.Join(loc.Dir, ".env.local"), []byte("B=stored\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(loc.Dir, ".env.new")); err != nil {
		t.Fatal(err)
	}

	preview, err := Purge(loc, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Matched) != 1 || preview.Matched[0] != ".env" || len(preview.Deleted) != 0 {
		t.Fatalf("dry run should only match the identical copy: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(repo, ".env")); err != nil {
		t.Fatal("dry run must not delete")
	}

	report, err := Purge(loc, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Deleted) != 1 || report.Deleted[0] != ".env" {
		t.Fatalf("expected only .env deleted, got %+v", report)
	}
	if _, err := os.Stat(filepath.Join(repo, ".env")); !os.IsNotExist(err) {
		t.Fatal("identical copy should be deleted from the project")
	}
	for _, name := range []string{".env.local", ".env.new"} {
		if _, err := os.Stat(filepath.Join(repo, name)); err != nil {
			t.Fatalf("%s must be kept in the project: %v", name, err)
		}
	}
}
