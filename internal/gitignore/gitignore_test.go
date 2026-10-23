package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoredByFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".env*\n!.env.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsIgnored(dir, ".env.local") {
		t.Fatal("expected .env.local to be ignored")
	}
	if IsIgnored(dir, ".env.example") {
		t.Fatal("expected .env.example to be unignored by negation")
	}
	if !IsIgnored(dir, ".env.staging") {
		t.Fatal("expected .env.staging to be ignored")
	}
}

func TestNestedGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "packages", "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "packages", "api", ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsIgnored(dir, "packages/api/.env") {
		t.Fatal("expected nested .env to be ignored")
	}
	if IsIgnored(dir, ".env") {
		t.Fatal("expected root .env to not be ignored (no root rule)")
	}
}

func TestCheckAndAdd(t *testing.T) {
	dir := t.TempDir()
	notIgnored := Check(dir, []string{".env.local", ".env.staging"})
	if len(notIgnored) != 2 {
		t.Fatalf("expected 2 not-ignored, got %v", notIgnored)
	}
	added, err := Add(dir, []string{".env.local", ".env.staging"})
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 2 {
		t.Fatalf("expected 2 added, got %v", added)
	}
	// Second add should add nothing.
	added, err = Add(dir, []string{".env.local"})
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 0 {
		t.Fatalf("expected 0 added, got %v", added)
	}
	// Now they should be ignored.
	if len(Check(dir, []string{".env.local", ".env.staging"})) != 0 {
		t.Fatal("expected all to be ignored after Add")
	}
}

func TestHasPlaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("A=1\nB=encrypted:abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasPlaintext(path) {
		t.Fatal("expected HasPlaintext to be true")
	}
	if err := os.WriteFile(path, []byte("A=encrypted:abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasPlaintext(path) {
		t.Fatal("expected HasPlaintext to be false (all encrypted)")
	}
}
