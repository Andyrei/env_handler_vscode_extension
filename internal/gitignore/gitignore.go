package gitignore

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"env-handler/internal/envfile"
)

// IsIgnored reports whether file is ignored by git, or by the fallback matcher.
// root is the project root. file is project-relative.
func IsIgnored(root, file string) bool {
	if ignoredByGit(root, file) {
		return true
	}
	return ignoredByFallback(root, file)
}

// Check returns the files (among the given list) that are NOT ignored.
func Check(root string, files []string) []string {
	var notIgnored []string
	for _, file := range files {
		if !IsIgnored(root, file) {
			notIgnored = append(notIgnored, file)
		}
	}
	return notIgnored
}

// Add appends entries to .gitignore. It creates the file if needed.
// Returns the lines that were added.
func Add(root string, files []string) ([]string, error) {
	gitignorePath := filepath.Join(root, ".gitignore")
	existing := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existing = string(data)
	}

	var toAdd []string
	lines := strings.Split(existing, "\n")
	for _, file := range files {
		if !containsLine(lines, file) {
			toAdd = append(toAdd, file)
		}
	}
	if len(toAdd) == 0 {
		return nil, nil
	}

	content := existing
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	for _, line := range toAdd {
		content += line + "\n"
	}
	return toAdd, os.WriteFile(gitignorePath, []byte(content), 0o644)
}

// ignoredByGit uses `git check-ignore -q`.
func ignoredByGit(root, file string) bool {
	cmd := exec.Command("git", "-C", root, "check-ignore", "-q", file)
	err := cmd.Run()
	return err == nil
}

// ignoredByFallback does a simple line-based check of .gitignore files.
// It handles negation (!) and nested .gitignore files.
func ignoredByFallback(root, file string) bool {
	// Build the list of .gitignore files from root to the file's directory.
	dir := filepath.Dir(file)
	var gitignores []string
	for {
		gitignores = append([]string{filepath.Join(root, dir, ".gitignore")}, gitignores...)
		if dir == "." || dir == "" {
			break
		}
		dir = filepath.Dir(dir)
	}

	ignored := false
	for _, giPath := range gitignores {
		data, err := os.ReadFile(giPath)
		if err != nil {
			continue
		}
		relDir, _ := filepath.Rel(root, filepath.Dir(giPath))
		if relDir == "." {
			relDir = ""
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			negate := strings.HasPrefix(line, "!")
			if negate {
				line = line[1:]
			}
			// Simple glob match: support * and ?.
			matched := simpleMatch(line, file, relDir)
			if matched {
				if negate {
					ignored = false
				} else {
					ignored = true
				}
			}
		}
	}
	return ignored
}

func simpleMatch(pattern, file, baseDir string) bool {
	// Normalize to slash.
	file = filepath.ToSlash(file)
	pattern = filepath.ToSlash(pattern)
	if baseDir != "" {
		baseDir = filepath.ToSlash(baseDir)
	}

	// If pattern has a slash, it's relative to baseDir.
	if strings.Contains(pattern, "/") {
		if baseDir != "" {
			file = strings.TrimPrefix(file, baseDir+"/")
		}
		return matchGlob(pattern, file)
	}
	// No slash: match against basename.
	return matchGlob(pattern, filepath.Base(file))
}

func matchGlob(pattern, name string) bool {
	// Simple glob: * matches any sequence, ? matches one char.
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return false
	}
	return matched
}

func containsLine(lines []string, line string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) == line {
			return true
		}
	}
	return false
}

// HasPlaintext checks if a file has non-encrypted, non-empty values.
func HasPlaintext(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	entries, err := envfile.Parse(string(data))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Value != "" && !e.Encrypted {
			return true
		}
	}
	return false
}
