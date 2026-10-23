package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustRead(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReconcileFallsBackWhenActiveIsMissing(t *testing.T) {
	sidecar := Sidecar{
		Active:       ".env.gone",
		Environments: []Environment{{File: ".env.local", Name: "Local", Color: "green", Order: 0}},
	}

	resolved := Reconcile(sidecar, []string{".env", ".env.local"})

	if resolved.Active != ".env" {
		t.Fatalf("expected fallback to .env, got %q", resolved.Active)
	}
	if len(resolved.Warnings) != 1 {
		t.Fatalf("expected one warning, got %v", resolved.Warnings)
	}
	if len(resolved.Environments) != 2 {
		t.Fatalf("expected 2 environments, got %d", len(resolved.Environments))
	}
	if resolved.Environments[0].Name != "Local" || resolved.Environments[1].Name != "Default" {
		t.Fatalf("unexpected order or names: %+v", resolved.Environments)
	}
}

func TestReconcileDropsMissingFiles(t *testing.T) {
	sidecar := Sidecar{
		Active: ".env.staging",
		Environments: []Environment{
			{File: ".env.staging", Name: "Staging", Order: 0},
			{File: ".env.removed", Name: "Removed", Order: 1},
		},
	}

	resolved := Reconcile(sidecar, []string{".env.staging"})

	if len(resolved.Environments) != 1 || resolved.Environments[0].File != ".env.staging" {
		t.Fatalf("expected only .env.staging, got %+v", resolved.Environments)
	}
	if resolved.Active != ".env.staging" {
		t.Fatalf("expected active .env.staging, got %q", resolved.Active)
	}
}

func TestDefaultNameAndSlug(t *testing.T) {
	names := map[string]string{
		".env":                   "Default",
		".env.local":             "Local",
		".env.development.local": "Development Local",
	}
	for file, want := range names {
		if got := DefaultName(file); got != want {
			t.Fatalf("DefaultName(%q) = %q, want %q", file, got, want)
		}
	}

	if got := Slug("  Staging EU!  "); got != "staging-eu" {
		t.Fatalf("unexpected slug %q", got)
	}
	if got := Slug("!!!"); got != "env" {
		t.Fatalf("expected fallback slug, got %q", got)
	}
}

func TestSidecarContainsNoValues(t *testing.T) {
	dir := t.TempDir()
	sidecar := Sidecar{
		Active:       ".env.local",
		Environments: []Environment{{File: ".env.local", Name: "Local", Color: "green", Order: 0}},
	}
	if err := Save(dir, sidecar); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Active != ".env.local" {
		t.Fatalf("unexpected active %q", loaded.Active)
	}

	if strings.Contains(strings.ToLower(string(mustRead(t, dir))), "secret") {
		t.Fatal("unexpected content in sidecar")
	}
}
