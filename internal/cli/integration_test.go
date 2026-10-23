package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"env-handler/internal/jsonapi"
	"env-handler/internal/metadata"
)

func TestCreateSetRevealUnset(t *testing.T) {
	dir := t.TempDir()

	if _, errOut, code := runIn(t, dir, "create", "Local", "--activate"); code != 0 {
		t.Fatalf("create failed (%d): %s", code, errOut)
	}
	if _, errOut, code := runIn(t, dir, "set", "Local", "API_URL", "--", "http://localhost:3000"); code != 0 {
		t.Fatalf("set failed (%d): %s", code, errOut)
	}

	out, _, code := runIn(t, dir, "value", "Local", "API_URL", "--reveal")
	if code != 0 || strings.TrimSpace(out) != "http://localhost:3000" {
		t.Fatalf("unexpected reveal output %q (code %d)", out, code)
	}

	meta, err := os.ReadFile(filepath.Join(envDir(t, dir), metadata.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(meta), "localhost") {
		t.Fatal("sidecar must not contain values")
	}

	if _, _, code := runIn(t, dir, "value", "Local", "API_URL"); code != 1 {
		t.Fatalf("value without --reveal should fail with code 1, got %d", code)
	}

	if _, errOut, code := runIn(t, dir, "unset", "Local", "API_URL"); code != 0 {
		t.Fatalf("unset failed (%d): %s", code, errOut)
	}
	data, err := os.ReadFile(filepath.Join(envDir(t, dir), ".env.local"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "API_URL") {
		t.Fatalf("key was not removed: %q", data)
	}
}

func TestSnapshotJSONListsKeysWithoutValues(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, ".env", "SECRET_TOKEN=supersecret\n")

	out, _, code := runIn(t, dir, "--json", "snapshot")
	if code != 0 {
		t.Fatalf("snapshot failed with code %d", code)
	}
	if strings.Contains(out, "supersecret") {
		t.Fatal("snapshot leaked a value")
	}

	var snap jsonapi.Snapshot
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatalf("snapshot is not valid JSON: %v", err)
	}
	if len(snap.Environments) != 1 || snap.Environments[0].Variables[0].Key != "SECRET_TOKEN" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if snap.Active != ".env" {
		t.Fatalf("expected active .env, got %q", snap.Active)
	}
	if snap.Branch != "default" || snap.Root != envDir(t, dir) {
		t.Fatalf("snapshot should report the store folder and branch: %+v", snap)
	}
}

func TestEncryptedIsReadOnlyJSON(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, ".env", "TOKEN=encrypted:abc\n")

	var out, errOut bytes.Buffer
	code := Run([]string{"--cwd", dir, "--json", "set", "Default", "TOKEN", "--", "x"}, &out, &errOut)
	if code != 4 {
		t.Fatalf("expected exit 4, got %d", code)
	}

	var payload jsonapi.Error
	if err := json.Unmarshal(errOut.Bytes(), &payload); err != nil {
		t.Fatalf("stderr is not JSON: %v", err)
	}
	if payload.Code != "ENCRYPTED_READ_ONLY" {
		t.Fatalf("unexpected code %q", payload.Code)
	}
}

func TestMutationJSONIncludesSnapshot(t *testing.T) {
	dir := t.TempDir()
	out, errOut, code := runIn(t, dir, "--json", "create", "Staging")
	if code != 0 {
		t.Fatalf("create failed (%d): %s", code, errOut)
	}

	var mutation jsonapi.Mutation
	if err := json.Unmarshal([]byte(out), &mutation); err != nil {
		t.Fatalf("mutation is not valid JSON: %v", err)
	}
	if !mutation.Ok || len(mutation.Snapshot.Environments) != 1 {
		t.Fatalf("unexpected mutation: %+v", mutation)
	}
}
