package cli

import (
	"encoding/json"
	"testing"

	"env-handler/internal/jsonapi"
)

func TestActivateAnEnvFromAnotherBranch(t *testing.T) {
	repo := t.TempDir()

	if _, errOut, code := runIn(t, repo, "--branch", "main", "create", "Staging"); code != 0 {
		t.Fatalf("create on main failed (%d): %s", code, errOut)
	}
	if _, errOut, code := runIn(t, repo, "--branch", "feature/docker", "create", "Docker"); code != 0 {
		t.Fatalf("create on feature failed (%d): %s", code, errOut)
	}
	// Activate Docker while the project is on main.
	if _, errOut, code := runIn(t, repo, "--branch", "feature/docker", "use", "Docker"); code != 0 {
		t.Fatalf("use across branches failed (%d): %s", code, errOut)
	}

	out, errOut, code := runIn(t, repo, "--json", "overview")
	if code != 0 {
		t.Fatalf("overview failed (%d): %s", code, errOut)
	}
	var overview jsonapi.Overview
	if err := json.Unmarshal([]byte(out), &overview); err != nil {
		t.Fatalf("overview is not valid JSON: %v", err)
	}
	if overview.ActiveBranch != "feature/docker" || overview.ActiveFile != ".env.docker" {
		t.Fatalf("active pointer not recorded: %+v", overview)
	}

	found := map[string]jsonapi.Snapshot{}
	for _, branch := range overview.Branches {
		found[branch.Branch] = branch
	}
	feature, ok := found["feature/docker"]
	if !ok || len(feature.Environments) != 1 || !feature.Environments[0].Active {
		t.Fatalf("Docker should be active on feature/docker: %+v", feature)
	}
	if feature.Environments[0].Branch != "feature/docker" {
		t.Fatalf("environments should carry their branch: %+v", feature.Environments[0])
	}
	main, ok := found["main"]
	if !ok || len(main.Environments) != 1 {
		t.Fatalf("main should list its own envs: %+v", main)
	}
	if main.Environments[0].Active {
		t.Fatal("an env on another branch must not show as active on main")
	}
}

func TestRemovingTheActiveEnvClearsTheActivePointer(t *testing.T) {
	repo := t.TempDir()
	if _, errOut, code := runIn(t, repo, "create", "Local", "--activate"); code != 0 {
		t.Fatalf("create failed (%d): %s", code, errOut)
	}
	if _, errOut, code := runIn(t, repo, "remove", "Local", "--yes"); code != 0 {
		t.Fatalf("remove failed (%d): %s", code, errOut)
	}
	out, _, _ := runIn(t, repo, "--json", "overview")
	var overview jsonapi.Overview
	if err := json.Unmarshal([]byte(out), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.ActiveFile != "" {
		t.Fatalf("active pointer should be cleared, got %q", overview.ActiveFile)
	}
}
