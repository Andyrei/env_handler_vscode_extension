package project

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"env-handler/internal/discovery"
	"env-handler/internal/envfile"
	"env-handler/internal/fsx"
	"env-handler/internal/jsonapi"
	"env-handler/internal/layers"
	"env-handler/internal/metadata"
	"env-handler/internal/postman"
	"env-handler/internal/store"
)

var (
	// ErrEnvNotFound means no environment matches the selector.
	ErrEnvNotFound = errors.New("environment not found")
	// ErrAmbiguous means more than one environment matches a name.
	ErrAmbiguous = errors.New("environment selector is ambiguous")
	// ErrKeyNotFound means the environment has no such variable.
	ErrKeyNotFound = errors.New("variable not found")
	// ErrFileExists means a target env file already exists.
	ErrFileExists = errors.New("environment file already exists")
	// ErrInvalidName means an environment name is empty.
	ErrInvalidName = errors.New("invalid environment name")
	// ErrInvalidFile means a file name is not a managed env file name.
	ErrInvalidFile = errors.New("invalid environment file name")
	// ErrInvalidColor means the color is not in the palette.
	ErrInvalidColor = errors.New("invalid color")
)

// Project is a directory with env files and an optional sidecar.
type Project struct {
	Root     string
	Resolved metadata.Resolved
	Location store.Location
}

// ImportOptions controls a Postman import.
type ImportOptions struct {
	Force        bool
	DryRun       bool
	SkipDisabled bool
}

// ImportReport describes what an import did or would do.
type ImportReport struct {
	Imported    []string
	Conflicts   []string
	SkippedKeys []string
	Warnings    []string
}

// Open discovers env files under root and reconciles them with the sidecar.
func Open(root string) (*Project, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	files, err := discovery.Find(abs)
	if err != nil {
		return nil, err
	}
	sidecar, err := metadata.Load(abs)
	if err != nil {
		return nil, err
	}
	return &Project{Root: abs, Resolved: metadata.Reconcile(sidecar, files)}, nil
}

// Find resolves an environment by file path, display name, or file name.
func (p *Project) Find(selector string) (metadata.Environment, error) {
	cleaned := filepath.ToSlash(filepath.Clean(selector))
	for _, env := range p.Resolved.Environments {
		if env.File == cleaned {
			return env, nil
		}
	}

	needle := strings.ToLower(selector)
	var matches []metadata.Environment
	for _, env := range p.Resolved.Environments {
		if strings.ToLower(env.Name) == needle || strings.ToLower(path.Base(env.File)) == needle {
			matches = append(matches, env)
		}
	}
	switch len(matches) {
	case 0:
		return metadata.Environment{}, fmt.Errorf("%w: %s", ErrEnvNotFound, selector)
	case 1:
		return matches[0], nil
	default:
		return metadata.Environment{}, fmt.Errorf("%w: %s", ErrAmbiguous, selector)
	}
}

// Use marks an environment active and persists the choice.
func (p *Project) Use(selector string) (metadata.Environment, error) {
	env, err := p.Find(selector)
	if err != nil {
		return metadata.Environment{}, err
	}
	p.Resolved.Active = env.File
	if err := p.save(); err != nil {
		return metadata.Environment{}, err
	}
	if p.Location.ProjectDir != "" {
		if err := store.WriteActive(p.Location, store.Active{Branch: p.Location.Branch, File: env.File}); err != nil {
			return metadata.Environment{}, err
		}
	}
	return env, nil
}

// Create writes an empty .env.<slug> file and records it.
func (p *Project) Create(name string, activate bool) (metadata.Environment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return metadata.Environment{}, ErrInvalidName
	}
	env, err := p.writeNew(".env."+metadata.Slug(name), name, nil, 0o600, false)
	if err != nil {
		return metadata.Environment{}, err
	}
	if activate {
		return p.Use(env.File)
	}
	return env, nil
}

// Rename changes the display name only.
func (p *Project) Rename(selector, name string) (metadata.Environment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return metadata.Environment{}, ErrInvalidName
	}
	env, err := p.Find(selector)
	if err != nil {
		return metadata.Environment{}, err
	}
	p.update(env.File, func(e *metadata.Environment) { e.Name = name })
	env.Name = name
	return env, p.save()
}

// RenameFile renames the file on disk and keeps the sidecar and active pointer in step.
func (p *Project) RenameFile(selector, file string) (metadata.Environment, error) {
	env, err := p.Find(selector)
	if err != nil {
		return metadata.Environment{}, err
	}
	file = filepath.ToSlash(strings.TrimSpace(file))
	if !validEnvFileName(file) {
		return metadata.Environment{}, ErrInvalidFile
	}
	if file == env.File {
		return env, nil
	}
	if _, err := os.Stat(p.abs(file)); err == nil {
		return metadata.Environment{}, fmt.Errorf("%w: %s", ErrFileExists, file)
	}
	if err := os.Rename(p.abs(env.File), p.abs(file)); err != nil {
		return metadata.Environment{}, err
	}

	wasActive := p.Resolved.Active == env.File
	p.update(env.File, func(e *metadata.Environment) { e.File = file })
	if wasActive {
		p.Resolved.Active = file
	}
	env.File = file
	return env, p.save()
}

// Duplicate copies an environment file to a new .env.<slug> file.
func (p *Project) Duplicate(selector, name string) (metadata.Environment, error) {
	src, err := p.Find(selector)
	if err != nil {
		return metadata.Environment{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = src.Name + " Copy"
	}
	data, err := os.ReadFile(p.abs(src.File))
	if err != nil {
		return metadata.Environment{}, err
	}
	info, err := os.Stat(p.abs(src.File))
	if err != nil {
		return metadata.Environment{}, err
	}
	return p.writeNew(".env."+metadata.Slug(name), name, data, info.Mode().Perm(), false)
}

// Remove moves the environment file to trash and removes its sidecar entry.
func (p *Project) Remove(selector string) (metadata.Environment, error) {
	env, err := p.Find(selector)
	if err != nil {
		return metadata.Environment{}, err
	}
	if _, err := store.Trash(p.Location, env.File); err != nil {
		return metadata.Environment{}, err
	}

	var remaining []metadata.Environment
	var files []string
	for _, e := range p.Resolved.Environments {
		if e.File == env.File {
			continue
		}
		remaining = append(remaining, e)
		files = append(files, e.File)
	}
	active := p.Resolved.Active
	if active == env.File {
		active = ""
	}
	p.Resolved = metadata.Reconcile(metadata.Sidecar{Active: active, Environments: remaining}, files)
	return env, p.save()
}

// SetColor sets an environment color from the palette.
func (p *Project) SetColor(selector, color string) (metadata.Environment, error) {
	if !metadata.IsColor(color) {
		return metadata.Environment{}, ErrInvalidColor
	}
	env, err := p.Find(selector)
	if err != nil {
		return metadata.Environment{}, err
	}
	p.update(env.File, func(e *metadata.Environment) { e.Color = color })
	env.Color = color
	return env, p.save()
}

// Move swaps an environment with its neighbour. delta is -1 for up, 1 for down.
func (p *Project) Move(selector string, delta int) (metadata.Environment, error) {
	env, err := p.Find(selector)
	if err != nil {
		return metadata.Environment{}, err
	}
	envs := p.Resolved.Environments
	index := -1
	for i := range envs {
		if envs[i].File == env.File {
			index = i
			break
		}
	}
	target := index + delta
	if target < 0 || target >= len(envs) {
		return env, nil
	}
	envs[index], envs[target] = envs[target], envs[index]
	for i := range envs {
		envs[i].Order = i
	}
	return env, p.save()
}

// ImportPostman writes one .env.<slug> file per Postman environment.
func (p *Project) ImportPostman(envs []postman.Environment, opts ImportOptions) (ImportReport, error) {
	var report ImportReport
	for _, pm := range envs {
		name := strings.TrimSpace(pm.Name)
		if name == "" {
			name = "Imported"
		}
		file := ".env." + metadata.Slug(name)

		rendered := postman.Render(pm, opts.SkipDisabled)
		report.SkippedKeys = append(report.SkippedKeys, rendered.SkippedKeys...)
		report.Warnings = append(report.Warnings, rendered.Warnings...)

		if _, err := os.Stat(p.abs(file)); err == nil && !opts.Force {
			report.Conflicts = append(report.Conflicts, file)
			continue
		}
		if opts.DryRun {
			report.Imported = append(report.Imported, file)
			continue
		}
		if _, err := p.writeNew(file, name, []byte(rendered.Text), 0o600, opts.Force); err != nil {
			return report, err
		}
		report.Imported = append(report.Imported, file)
	}

	if len(report.Imported) > 0 {
		report.Warnings = append(report.Warnings,
			"Imported values are plaintext on disk. Add the env files to .gitignore before committing.")
	}
	return report, nil
}

// Snapshot builds the keys-only view used by the CLI and extension.
func (p *Project) Snapshot() (jsonapi.Snapshot, error) {
	envs := make([]jsonapi.Environment, 0, len(p.Resolved.Environments))
	for _, env := range p.Resolved.Environments {
		entries, _, err := p.entries(env.File)
		if err != nil {
			return jsonapi.Snapshot{}, err
		}
		variables := make([]jsonapi.Variable, 0, len(entries))
		for _, entry := range entries {
			variables = append(variables, jsonapi.Variable{
				Key:       entry.Key,
				Line:      entry.Line,
				Encrypted: entry.Encrypted,
				Multiline: entry.Multiline,
			})
		}
		envs = append(envs, jsonapi.Environment{
			File:      env.File,
			Name:      env.Name,
			Color:     env.Color,
			Order:     env.Order,
			Active:    env.File == p.Resolved.Active,
			Layers:    layers.Files(p.Root, env.File),
			Variables: variables,
		})
	}

	warnings := p.Resolved.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return jsonapi.Snapshot{
		Root:         p.Root,
		Sidecar:      metadata.FileName,
		Active:       p.Resolved.Active,
		Environments: envs,
		Warnings:     warnings,
	}, nil
}

// Value returns the first entry for key. The caller decides whether to print it.
func (p *Project) Value(selector, key string) (envfile.Entry, error) {
	env, err := p.Find(selector)
	if err != nil {
		return envfile.Entry{}, err
	}
	entries, _, err := p.entries(env.File)
	if err != nil {
		return envfile.Entry{}, err
	}
	for _, entry := range entries {
		if entry.Key == key {
			return entry, nil
		}
	}
	return envfile.Entry{}, fmt.Errorf("%w: %s", ErrKeyNotFound, key)
}

// Set updates the first occurrence of key or appends it when missing.
func (p *Project) Set(selector, key, value string) error {
	env, err := p.Find(selector)
	if err != nil {
		return err
	}
	entries, text, err := p.entries(env.File)
	if err != nil {
		return err
	}

	var updated string
	found := false
	for _, entry := range entries {
		if entry.Key != key {
			continue
		}
		updated, err = envfile.SetValue(text, entry, value)
		found = true
		break
	}
	if !found {
		updated, err = envfile.AddKey(text, key, value)
	}
	if err != nil {
		return err
	}
	return p.write(env.File, updated)
}

// Unset removes every occurrence of key.
func (p *Project) Unset(selector, key string) error {
	env, err := p.Find(selector)
	if err != nil {
		return err
	}
	entries, text, err := p.entries(env.File)
	if err != nil {
		return err
	}

	removed := false
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Key != key {
			continue
		}
		text, err = envfile.RemoveEntry(text, entries[i])
		if err != nil {
			return err
		}
		removed = true
	}
	if !removed {
		return fmt.Errorf("%w: %s", ErrKeyNotFound, key)
	}
	return p.write(env.File, text)
}

// writeNew creates an env file, reconciles the project, and records the name.
func (p *Project) writeNew(file, name string, data []byte, mode os.FileMode, overwrite bool) (metadata.Environment, error) {
	if !overwrite {
		if _, err := os.Stat(p.abs(file)); err == nil {
			return metadata.Environment{}, fmt.Errorf("%w: %s", ErrFileExists, file)
		}
	}
	if err := fsx.WriteFileAtomic(p.abs(file), data, mode); err != nil {
		return metadata.Environment{}, err
	}

	reopened, err := Open(p.Root)
	if err != nil {
		return metadata.Environment{}, err
	}
	location := p.Location
	*p = *reopened
	p.Location = location
	p.update(file, func(e *metadata.Environment) { e.Name = name })
	if err := p.save(); err != nil {
		return metadata.Environment{}, err
	}
	for _, env := range p.Resolved.Environments {
		if env.File == file {
			return env, nil
		}
	}
	return metadata.Environment{}, fmt.Errorf("%w: %s", ErrEnvNotFound, file)
}

func (p *Project) update(file string, change func(*metadata.Environment)) {
	for i := range p.Resolved.Environments {
		if p.Resolved.Environments[i].File == file {
			change(&p.Resolved.Environments[i])
			return
		}
	}
}

func (p *Project) save() error {
	return metadata.Save(p.Root, metadata.Sidecar{
		Active:       p.Resolved.Active,
		Environments: p.Resolved.Environments,
	})
}

// Entries parses a file and returns its entries plus raw text.
func (p *Project) Entries(file string) ([]envfile.Entry, string, error) {
	return p.entries(file)
}

func (p *Project) entries(file string) ([]envfile.Entry, string, error) {
	data, err := os.ReadFile(p.abs(file))
	if err != nil {
		return nil, "", err
	}
	text := string(data)
	entries, err := envfile.Parse(text)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", file, err)
	}
	return entries, text, nil
}

func (p *Project) write(file, text string) error {
	full := p.abs(file)
	mode := os.FileMode(0o600)
	if info, err := os.Stat(full); err == nil {
		mode = info.Mode().Perm()
	}
	return fsx.WriteFileAtomic(full, []byte(text), mode)
}

// Restore moves an env file from trash back into the project.
func (p *Project) Restore(file string) error {
	if err := store.Restore(p.Location, file); err != nil {
		return err
	}
	// Reconcile so the restored file appears.
	reopened, err := Open(p.Location.Dir)
	if err != nil {
		return err
	}
	*p = *reopened
	p.Location = reopened.Location
	return p.save()
}

// ListTrash returns the relative paths of trashed env files.
func (p *Project) ListTrash() ([]string, error) {
	return store.ListTrash(p.Location)
}

// EmptyTrash permanently removes all trashed files.
func (p *Project) EmptyTrash() (int, error) {
	return store.EmptyTrash(p.Location)
}

func (p *Project) abs(file string) string {
	return filepath.Join(p.Root, filepath.FromSlash(file))
}

func validEnvFileName(file string) bool {
	return file != "" &&
		!strings.ContainsAny(file, `/\`) &&
		discovery.IsEnvFileName(file)
}
