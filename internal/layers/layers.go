package layers

import (
	"os"
	"path"
	"path/filepath"
)

// Files returns the project-relative files loaded for env, base first.
// The base is the .env file in the same directory as env.
func Files(root, env string) []string {
	base := path.Join(path.Dir(env), ".env")
	if base == env {
		return []string{env}
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(base))); err != nil {
		return []string{env}
	}
	return []string{base, env}
}
