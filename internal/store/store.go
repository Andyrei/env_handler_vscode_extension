// Package store keeps env files outside the project, one folder per project and branch.
package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"env-handler/internal/discovery"
	"env-handler/internal/fsx"
	"env-handler/internal/metadata"
)

// Location says where the env files for one project branch live.
type Location struct {
	ProjectID   string `json:"project"`
	ProjectPath string `json:"projectPath"`
	ProjectDir  string `json:"projectDir"`
	Branch      string `json:"branch"`
	Dir         string `json:"dir"`
}

// Active is the one environment the project uses. It can belong to any branch.
type Active struct {
	Branch string `json:"branch"`
	File   string `json:"file"`
}

// AdoptReport lists what Adopt did with files found in the project folder.
// Adopt only copies. It never deletes anything.
type AdoptReport struct {
	Copied  []string
	Same    []string
	Skipped []string
}

// PurgeReport lists project copies that Purge matched, deleted, or kept.
type PurgeReport struct {
	Matched []string
	Deleted []string
	Kept    []string
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Home returns the folder that holds all env-handler data.
// ENVH_HOME overrides it, which tests use.
func Home() (string, error) {
	if override := os.Getenv("ENVH_HOME"); override != "" {
		return override, nil
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, "env-handler"), nil
}

// BranchDir returns the env folder for a branch of the same project.
func BranchDir(loc Location, branch string) string {
	return filepath.Join(loc.ProjectDir, "branches", url.PathEscape(branch))
}

// Locate finds the env folder for repo and its current branch, creating it if needed.
// branchOverride replaces the detected branch when it is not empty.
func Locate(repo, branchOverride string) (Location, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return Location{}, err
	}
	root := gitOutput(abs, "rev-parse", "--show-toplevel")
	if root == "" {
		root = abs
	}
	root = filepath.Clean(root)

	sum := sha256.Sum256([]byte(root))
	id := unsafeName.ReplaceAllString(filepath.Base(root), "-") + "-" + hex.EncodeToString(sum[:])[:12]

	branch := strings.TrimSpace(branchOverride)
	if branch == "" {
		branch = currentBranch(root)
	}
	if branch == "" {
		branch = "default"
	}

	home, err := Home()
	if err != nil {
		return Location{}, err
	}
	projectDir := filepath.Join(home, "projects", id)
	loc := Location{ProjectID: id, ProjectPath: root, ProjectDir: projectDir, Branch: branch}
	loc.Dir = BranchDir(loc, branch)
	if err := os.MkdirAll(loc.Dir, 0o700); err != nil {
		return Location{}, err
	}
	if err := writeProjectInfo(loc, nil); err != nil {
		return Location{}, err
	}
	return loc, nil
}

// Branches lists every branch that has an env folder for this project.
func Branches(loc Location) []string {
	entries, err := os.ReadDir(filepath.Join(loc.ProjectDir, "branches"))
	if err != nil {
		return []string{loc.Branch}
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name, err := url.PathUnescape(entry.Name())
		if err != nil {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ReadActive returns the project's active environment, if one is set.
func ReadActive(loc Location) (Active, bool) {
	info, err := readProjectInfo(loc)
	if err != nil {
		return Active{}, false
	}
	raw, ok := info["active"].(map[string]any)
	if !ok {
		return Active{}, false
	}
	branch, _ := raw["branch"].(string)
	file, _ := raw["file"].(string)
	if file == "" {
		return Active{}, false
	}
	return Active{Branch: branch, File: file}, true
}

// WriteActive records the project's active environment. An empty File clears it.
func WriteActive(loc Location, active Active) error {
	info, err := readProjectInfo(loc)
	if err != nil {
		return err
	}
	if active.File == "" {
		delete(info, "active")
	} else {
		info["active"] = map[string]string{"branch": active.Branch, "file": active.File}
	}
	return writeProjectInfo(loc, info)
}

// Adopt copies env files found in the project folder into the branch folder.
// It never overwrites a file that differs and never deletes anything.
func Adopt(loc Location) (AdoptReport, error) {
	var report AdoptReport
	files, err := discovery.Find(loc.ProjectPath)
	if err != nil {
		return report, err
	}

	for _, rel := range files {
		src := filepath.Join(loc.ProjectPath, filepath.FromSlash(rel))
		dst := filepath.Join(loc.Dir, filepath.FromSlash(rel))
		if _, err := os.Stat(dst); err == nil {
			same, err := sameContent(src, dst)
			if err != nil {
				return report, err
			}
			if same {
				report.Same = append(report.Same, rel)
			} else {
				report.Skipped = append(report.Skipped, rel)
			}
			continue
		}
		if err := copyFile(src, dst); err != nil {
			return report, err
		}
		report.Copied = append(report.Copied, rel)
	}

	sidecarSrc := filepath.Join(loc.ProjectPath, metadata.FileName)
	sidecarDst := filepath.Join(loc.Dir, metadata.FileName)
	if _, err := os.Stat(sidecarSrc); err == nil {
		if _, err := os.Stat(sidecarDst); os.IsNotExist(err) {
			if err := copyFile(sidecarSrc, sidecarDst); err != nil {
				return report, err
			}
		}
	}
	return report, nil
}

// Purge deletes project copies whose stored copy has identical content.
// It never copies, and it never touches a project file that is missing from storage or differs from it.
// With dryRun, it only reports what would be deleted.
func Purge(loc Location, dryRun bool) (PurgeReport, error) {
	var report PurgeReport
	files, err := discovery.Find(loc.ProjectPath)
	if err != nil {
		return report, err
	}

	for _, rel := range files {
		src := filepath.Join(loc.ProjectPath, filepath.FromSlash(rel))
		dst := filepath.Join(loc.Dir, filepath.FromSlash(rel))
		if _, err := os.Stat(dst); err != nil {
			report.Kept = append(report.Kept, rel)
			continue
		}
		same, err := sameContent(src, dst)
		if err != nil {
			return report, err
		}
		if !same {
			report.Kept = append(report.Kept, rel)
			continue
		}
		report.Matched = append(report.Matched, rel)
		if dryRun {
			continue
		}
		if err := os.Remove(src); err != nil {
			return report, err
		}
		report.Deleted = append(report.Deleted, rel)
	}
	return report, nil
}

// readProjectInfo returns the project.json document, creating the defaults when it is missing.
func readProjectInfo(loc Location) (map[string]any, error) {
	info := map[string]any{}
	data, err := os.ReadFile(filepath.Join(loc.ProjectDir, "project.json"))
	if err == nil {
		if err := json.Unmarshal(data, &info); err != nil {
			info = map[string]any{}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if _, ok := info["name"]; !ok {
		info["name"] = filepath.Base(loc.ProjectPath)
	}
	if _, ok := info["path"]; !ok {
		info["path"] = loc.ProjectPath
	}
	return info, nil
}

// writeProjectInfo writes project.json. Passing nil keeps existing data and fills the defaults.
func writeProjectInfo(loc Location, info map[string]any) error {
	if info == nil {
		var err error
		info, err = readProjectInfo(loc)
		if err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(filepath.Join(loc.ProjectDir, "project.json"), append(data, '\n'), 0o600)
}

func currentBranch(root string) string {
	name := gitOutput(root, "rev-parse", "--abbrev-ref", "HEAD")
	if name == "" {
		return ""
	}
	if name == "HEAD" {
		short := gitOutput(root, "rev-parse", "--short", "HEAD")
		if short == "" {
			return ""
		}
		return "detached-" + short
	}
	return name
}

// gitOutput runs git with fixed arguments (no shell). Missing git or a non-repo yields "".
func gitOutput(dir string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func sameContent(a, b string) (bool, error) {
	left, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	right, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(left, right), nil
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	return fsx.WriteFileAtomic(dst, data, info.Mode().Perm())
}
