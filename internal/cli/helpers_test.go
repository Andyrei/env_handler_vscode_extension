package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"env-handler/internal/jsonapi"
	"env-handler/internal/store"
)

func runIn(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(append([]string{"--cwd", dir}, args...), &out, &errOut)
	return out.String(), errOut.String(), code
}

// writeFile writes directly into dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// envDir returns the store folder that holds the env files for a project's default branch.
func envDir(t *testing.T, project string) string {
	t.Helper()
	loc, err := store.Locate(project, "")
	if err != nil {
		t.Fatal(err)
	}
	return loc.Dir
}

// writeEnv writes an env file into the project's store folder.
func writeEnv(t *testing.T, project, name, content string) {
	t.Helper()
	writeFile(t, envDir(t, project), name, content)
}

func snapshotOf(t *testing.T, dir string) jsonapi.Snapshot {
	t.Helper()
	out, errOut, code := runIn(t, dir, "--json", "snapshot")
	if code != 0 {
		t.Fatalf("snapshot failed (%d): %s", code, errOut)
	}
	var snap jsonapi.Snapshot
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatal(err)
	}
	return snap
}
