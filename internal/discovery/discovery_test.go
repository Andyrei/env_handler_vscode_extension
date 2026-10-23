package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFindSkipsTemplatesSecretsAndDependencies(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		".env",
		".env.local",
		".env.example",
		".env.local.sample",
		".env.keys",
		".envh.json",
		"README.md",
		"node_modules/pkg/.env",
		"packages/api/.env.staging",
	}
	for _, file := range files {
		full := filepath.Join(dir, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("A=1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Find(dir)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	want := []string{".env", ".env.local", "packages/api/.env.staging"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
