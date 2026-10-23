package jsonapi

// Snapshot is the project state the extension renders. It never contains values.
// Root is the folder that holds this branch's env files (outside the project).
type Snapshot struct {
	Root         string        `json:"root"`
	Sidecar      string        `json:"sidecar"`
	Project      string        `json:"project"`
	ProjectPath  string        `json:"projectPath"`
	Branch       string        `json:"branch"`
	Branches     []string      `json:"branches"`
	Active       string        `json:"active"`
	Environments []Environment `json:"environments"`
	Warnings     []string      `json:"warnings"`
}

// Environment describes one env file.
type Environment struct {
	File      string     `json:"file"`
	Name      string     `json:"name"`
	Color     string     `json:"color"`
	Order     int        `json:"order"`
	Active    bool       `json:"active"`
	Layers    []string   `json:"layers"`
	Variables []Variable `json:"variables"`
}

// Variable is a key location inside an env file.
type Variable struct {
	Key       string `json:"key"`
	Line      int    `json:"line"`
	Encrypted bool   `json:"encrypted"`
	Multiline bool   `json:"multiline"`
}

// Mutation is returned by commands that change state when --json is set.
type Mutation struct {
	Ok       bool     `json:"ok"`
	Snapshot Snapshot `json:"snapshot"`
}

// ImportResult is returned by `import postman` when --json is set.
// Snapshot is omitted for dry runs.
type ImportResult struct {
	Ok          bool      `json:"ok"`
	DryRun      bool      `json:"dryRun"`
	Imported    []string  `json:"imported"`
	Conflicts   []string  `json:"conflicts"`
	SkippedKeys []string  `json:"skippedKeys"`
	Warnings    []string  `json:"warnings"`
	Snapshot    *Snapshot `json:"snapshot,omitempty"`
}

// AdoptResult is returned by `adopt` when --json is set. Adopt only copies.
type AdoptResult struct {
	Ok      bool     `json:"ok"`
	Dir     string   `json:"dir"`
	Copied  []string `json:"copied"`
	Same    []string `json:"same"`
	Skipped []string `json:"skipped"`
}

// PurgeResult is returned by `purge` when --json is set.
// Matched files have an identical stored copy. Deleted is empty on dry runs.
type PurgeResult struct {
	Ok      bool     `json:"ok"`
	DryRun  bool     `json:"dryRun"`
	Matched []string `json:"matched"`
	Deleted []string `json:"deleted"`
	Kept    []string `json:"kept"`
}

// Value is returned by the value command when --reveal is set.
type Value struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Error is written to stderr as JSON when a command fails with --json.
type Error struct {
	Ok      bool   `json:"ok"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
