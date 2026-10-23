package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"env-handler/internal/fsx"
)

const (
	// FileName is the metadata sidecar at the project root. It never contains values.
	FileName = ".envh.json"
	// Version is the sidecar schema version.
	Version = 1
)

// ErrInvalid is returned when the sidecar cannot be used.
var ErrInvalid = errors.New("invalid .envh.json")

var palette = []string{"gray", "green", "yellow", "red", "blue", "purple", "orange"}

// Environment is the metadata for one env file.
type Environment struct {
	File  string `json:"file"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Order int    `json:"order"`
}

// Sidecar is the persisted .envh.json document.
type Sidecar struct {
	Version      int           `json:"version"`
	Active       string        `json:"active,omitempty"`
	Environments []Environment `json:"environments"`
}

// Resolved is the sidecar reconciled against the files on disk.
type Resolved struct {
	Environments []Environment
	Active       string
	Warnings     []string
}

// Load reads the sidecar. A missing sidecar is not an error.
func Load(root string) (Sidecar, error) {
	data, err := os.ReadFile(filepath.Join(root, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return Sidecar{Version: Version}, nil
	}
	if err != nil {
		return Sidecar{}, err
	}

	var sidecar Sidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return Sidecar{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if sidecar.Version != Version {
		return Sidecar{}, fmt.Errorf("%w: unsupported version %d", ErrInvalid, sidecar.Version)
	}
	return sidecar, nil
}

// Save writes the sidecar atomically.
func Save(root string, sidecar Sidecar) error {
	sidecar.Version = Version
	if sidecar.Environments == nil {
		sidecar.Environments = []Environment{}
	}
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(filepath.Join(root, FileName), append(data, '\n'), 0o644)
}

// Reconcile merges sidecar metadata with the env files found on disk.
func Reconcile(sidecar Sidecar, files []string) Resolved {
	known := make(map[string]Environment, len(sidecar.Environments))
	maxOrder := -1
	for _, env := range sidecar.Environments {
		known[env.File] = env
		if env.Order > maxOrder {
			maxOrder = env.Order
		}
	}

	present := make(map[string]bool, len(files))
	envs := make([]Environment, 0, len(files))
	for _, file := range files {
		present[file] = true
		if env, ok := known[file]; ok {
			envs = append(envs, env)
			continue
		}
		maxOrder++
		envs = append(envs, Environment{
			File:  file,
			Name:  DefaultName(file),
			Color: colorFor(file),
			Order: maxOrder,
		})
	}
	sort.SliceStable(envs, func(i, j int) bool {
		if envs[i].Order != envs[j].Order {
			return envs[i].Order < envs[j].Order
		}
		return envs[i].File < envs[j].File
	})

	var warnings []string
	active := sidecar.Active
	if active != "" && !present[active] {
		warnings = append(warnings, fmt.Sprintf("active file %s not found; using a fallback", active))
		active = ""
	}
	if active == "" {
		switch {
		case present[".env"]:
			active = ".env"
		case len(envs) > 0:
			active = envs[0].File
		}
	}
	return Resolved{Environments: envs, Active: active, Warnings: warnings}
}

// DefaultName derives a display name from a file, for example .env.local -> Local.
func DefaultName(file string) string {
	base := path.Base(file)
	if base == ".env" {
		return "Default"
	}
	parts := strings.Split(strings.TrimPrefix(base, ".env."), ".")
	for i, part := range parts {
		runes := []rune(part)
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

// Slug turns a display name into a file-safe suffix.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.TrimSuffix(b.String(), "-")
	if slug == "" {
		return "env"
	}
	return slug
}

// IsColor reports whether color is one of the allowed environment colors.
func IsColor(color string) bool {
	for _, c := range palette {
		if c == color {
			return true
		}
	}
	return false
}

func colorFor(file string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(file))
	return palette[int(h.Sum32()%uint32(len(palette)))]
}
