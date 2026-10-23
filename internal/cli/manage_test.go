package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"env-handler/internal/jsonapi"
)

func TestRenameDuplicateColorMove(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, ".env.local", "A=1\n")

	if _, errOut, code := runIn(t, dir, "rename", "local", "Dev Box"); code != 0 {
		t.Fatalf("rename failed (%d): %s", code, errOut)
	}
	if _, errOut, code := runIn(t, dir, "duplicate", "Dev Box", "--name", "Dev Copy"); code != 0 {
		t.Fatalf("duplicate failed (%d): %s", code, errOut)
	}
	data, err := os.ReadFile(filepath.Join(envDir(t, dir), ".env.dev-copy"))
	if err != nil || string(data) != "A=1\n" {
		t.Fatalf("duplicate content mismatch: %q %v", data, err)
	}
	if _, errOut, code := runIn(t, dir, "color", "Dev Copy", "red"); code != 0 {
		t.Fatalf("color failed (%d): %s", code, errOut)
	}
	if _, _, code := runIn(t, dir, "color", "Dev Copy", "magenta"); code != 1 {
		t.Fatalf("invalid color should exit 1, got %d", code)
	}
	if _, errOut, code := runIn(t, dir, "move", "Dev Copy", "up"); code != 0 {
		t.Fatalf("move failed (%d): %s", code, errOut)
	}

	snap := snapshotOf(t, dir)
	if len(snap.Environments) != 2 {
		t.Fatalf("expected 2 environments, got %d", len(snap.Environments))
	}
	if snap.Environments[0].Name != "Dev Copy" || snap.Environments[0].Color != "red" {
		t.Fatalf("unexpected order or color: %+v", snap.Environments)
	}
}

func TestRenameFileKeepsActivePointer(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, ".env.local", "A=1\n")
	if _, _, code := runIn(t, dir, "use", "local"); code != 0 {
		t.Fatal("use failed")
	}
	if _, errOut, code := runIn(t, dir, "rename-file", "local", ".env.staging"); code != 0 {
		t.Fatalf("rename-file failed (%d): %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(envDir(t, dir), ".env.local")); !os.IsNotExist(err) {
		t.Fatal("old file should be gone")
	}
	if snap := snapshotOf(t, dir); snap.Active != ".env.staging" {
		t.Fatalf("active pointer not updated: %q", snap.Active)
	}

	if _, _, code := runIn(t, dir, "rename-file", ".env.staging", ".env.example"); code != 1 {
		t.Fatalf("template name should be rejected with exit 1, got %d", code)
	}
}

func TestRemoveRequiresYesAndDeletesFile(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, ".env", "A=1\n")
	writeEnv(t, dir, ".env.local", "B=2\n")
	if _, _, code := runIn(t, dir, "use", "local"); code != 0 {
		t.Fatal("use failed")
	}

	if _, _, code := runIn(t, dir, "remove", "local"); code != 1 {
		t.Fatalf("remove without --yes should exit 1, got %d", code)
	}
	if _, errOut, code := runIn(t, dir, "remove", "local", "--yes"); code != 0 {
		t.Fatalf("remove failed (%d): %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(envDir(t, dir), ".env.local")); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}
	if snap := snapshotOf(t, dir); snap.Active != ".env" || len(snap.Environments) != 1 {
		t.Fatalf("unexpected state after remove: %+v", snap)
	}
}

func TestImportPostmanDryRunThenWrite(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "export.json")
	export := `{"name":"Staging","values":[
		{"key":"API_URL","value":"https://staging.example.test","enabled":true},
		{"key":"OLD","value":"x","enabled":false},
		{"key":"TOKEN","value":"s3cr3t","type":"secret"}
	]}`
	writeFile(t, dir, "export.json", export)

	var out, errOut bytes.Buffer
	code := Run([]string{"--cwd", dir, "--json", "import", "postman", exportPath, "--dry-run"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("dry run failed (%d): %s", code, errOut.String())
	}
	var preview jsonapi.ImportResult
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || len(preview.Imported) != 1 || preview.Imported[0] != ".env.staging" || preview.Snapshot != nil {
		t.Fatalf("unexpected dry run result: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(envDir(t, dir), ".env.staging")); !os.IsNotExist(err) {
		t.Fatal("dry run must not write files")
	}

	out.Reset()
	errOut.Reset()
	code = Run([]string{"--cwd", dir, "--json", "import", "postman", exportPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("import failed (%d): %s", code, errOut.String())
	}
	data, err := os.ReadFile(filepath.Join(envDir(t, dir), ".env.staging"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "API_URL=https://staging.example.test") || !strings.Contains(text, "# OLD=x") {
		t.Fatalf("unexpected file content: %q", text)
	}

	var result jsonapi.ImportResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Snapshot == nil || len(result.Snapshot.Environments) != 1 || result.Snapshot.Environments[0].Name != "Staging" {
		t.Fatalf("import should name the environment from Postman: %+v", result)
	}

	// Second import reports a conflict and does not overwrite.
	out.Reset()
	errOut.Reset()
	writeEnv(t, dir, ".env.staging", "EDITED=yes\n")
	code = Run([]string{"--cwd", dir, "--json", "import", "postman", exportPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("conflict import failed (%d): %s", code, errOut.String())
	}
	if data, _ := os.ReadFile(filepath.Join(envDir(t, dir), ".env.staging")); string(data) != "EDITED=yes\n" {
		t.Fatalf("conflicting file was overwritten: %q", data)
	}
}

func TestImportRejectsNonPostmanFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "collection.json", `{"info":{"name":"A collection"},"item":[]}`)

	var out, errOut bytes.Buffer
	code := Run([]string{"--cwd", dir, "--json", "import", "postman", filepath.Join(dir, "collection.json")}, &out, &errOut)
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
	if !strings.Contains(errOut.String(), "POSTMAN_INVALID") {
		t.Fatalf("expected POSTMAN_INVALID, got %s", errOut.String())
	}
}
