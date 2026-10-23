package discovery

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var skippedDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"bin":          true,
}

var excludedSuffixes = []string{".example", ".sample", ".template", ".dist"}

var excludedPrefixes = []string{".env.keys", ".env.vault"}

// Find returns sorted, slash-separated paths of env files relative to root.
func Find(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && skippedDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsEnvFileName(entry.Name()) {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			info, statErr := os.Stat(path)
			if statErr != nil || info.IsDir() {
				return nil
			}
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// IsEnvFileName reports whether a base file name is a managed environment file.
func IsEnvFileName(name string) bool {
	if name == ".env" {
		return true
	}
	if !strings.HasPrefix(name, ".env.") {
		return false
	}
	for _, suffix := range excludedSuffixes {
		if strings.HasSuffix(name, suffix) {
			return false
		}
	}
	for _, prefix := range excludedPrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}
