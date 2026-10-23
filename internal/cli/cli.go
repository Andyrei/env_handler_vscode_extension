package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"env-handler/internal/envfile"
	"env-handler/internal/gitignore"
	"env-handler/internal/jsonapi"
	"env-handler/internal/layers"
	"env-handler/internal/metadata"
	"env-handler/internal/postman"
	"env-handler/internal/project"
	"env-handler/internal/store"
	"env-handler/internal/swap"
	"env-handler/internal/version"
)

var errUsage = errors.New("usage error")

type options struct {
	cwd           string
	branch        string
	name          string
	json          bool
	reveal        bool
	activate      bool
	quiet         bool
	yes           bool
	force         bool
	dryRun        bool
	skipDisabled  bool
	skipEncrypted bool
	restore       bool
	env           string
	help          bool
	version       bool
	positional    []string
}

// Run executes the CLI and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	opts, err := parse(args)
	if err != nil {
		return fail(stderr, opts.json, usageErr(err.Error()))
	}
	if opts.version {
		fmt.Fprintln(stdout, version.Version)
		return 0
	}
	if opts.help || len(opts.positional) == 0 {
		fmt.Fprint(stdout, usage())
		return 0
	}

	command, rest := opts.positional[0], opts.positional[1:]
	var runErr error
	switch command {
	case "help":
		fmt.Fprint(stdout, usage())
		return 0
	case "version":
		fmt.Fprintln(stdout, version.Version)
		return 0
	case "where":
		runErr = runWhere(stdout, opts)
	case "branches":
		runErr = runBranches(stdout, opts)
	case "overview":
		runErr = runOverview(stdout, opts)
	case "swap":
		runErr = runSwap(stdout, opts, rest)
	case "gitignore":
		runErr = runGitignore(stdout, opts, rest)
	case "adopt":
		runErr = runAdopt(stdout, opts)
	case "purge":
		runErr = runPurge(stdout, opts)
	case "list", "snapshot":
		runErr = runList(stdout, opts)
	case "use":
		runErr = runUse(stdout, opts, rest)
	case "create":
		runErr = runCreate(stdout, opts, rest)
	case "rename":
		runErr = runRename(stdout, opts, rest)
	case "rename-file":
		runErr = runRenameFile(stdout, opts, rest)
	case "duplicate":
		runErr = runDuplicate(stdout, opts, rest)
	case "remove":
		runErr = runRemove(stdout, opts, rest)
	case "restore":
		runErr = runRestore(stdout, opts, rest)
	case "trash":
		runErr = runTrash(stdout, opts, rest)
	case "color":
		runErr = runColor(stdout, opts, rest)
	case "move":
		runErr = runMove(stdout, opts, rest)
	case "import":
		runErr = runImport(stdout, opts, rest)
	case "value":
		runErr = runValue(stdout, opts, rest)
	case "resolve":
		runErr = runResolve(stdout, opts, rest)
	case "set":
		runErr = runSet(stdout, opts, rest)
	case "unset":
		runErr = runUnset(stdout, opts, rest)
	default:
		return fail(stderr, opts.json, usageErr(fmt.Sprintf("unknown command %q", command)))
	}
	if runErr != nil {
		return fail(stderr, opts.json, runErr)
	}
	return 0
}

func parse(args []string) (options, error) {
	opts := options{cwd: "."}
	onlyPositional := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if onlyPositional {
			opts.positional = append(opts.positional, arg)
			continue
		}
		switch arg {
		case "--":
			onlyPositional = true
		case "--json":
			opts.json = true
		case "--reveal":
			opts.reveal = true
		case "--activate":
			opts.activate = true
		case "--quiet", "-q":
			opts.quiet = true
		case "--yes":
			opts.yes = true
		case "--force":
			opts.force = true
		case "--dry-run":
			opts.dryRun = true
		case "--skip-disabled":
			opts.skipDisabled = true
		case "--restore":
			opts.restore = true
		case "--skip-encrypted":
			opts.skipEncrypted = true
		case "--env":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("%s requires a value", arg)
			}
			i++
			opts.env = args[i]
		case "--help", "-h":
			opts.help = true
		case "--version", "-v":
			opts.version = true
		case "--cwd", "--name", "--branch":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("%s requires a value", arg)
			}
			i++
			switch arg {
			case "--cwd":
				opts.cwd = args[i]
			case "--branch":
				opts.branch = args[i]
			default:
				opts.name = args[i]
			}
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown option %s", arg)
			}
			opts.positional = append(opts.positional, arg)
		}
	}
	return opts, nil
}

// openProject opens the env folder for the project's current (or --branch) branch.
func openProject(opts options) (*project.Project, error) {
	loc, err := store.Locate(opts.cwd, opts.branch)
	if err != nil {
		return nil, err
	}
	return openLocation(loc)
}

// openLocation opens one branch and applies the project's active environment.
// The active environment only shows as active on the branch it belongs to.
func openLocation(loc store.Location) (*project.Project, error) {
	p, err := project.Open(loc.Dir)
	if err != nil {
		return nil, err
	}
	p.Location = loc
	if active, ok := store.ReadActive(loc); ok {
		p.Resolved.Active = ""
		if active.Branch == loc.Branch && hasEnv(p, active.File) {
			p.Resolved.Active = active.File
		}
	}
	return p, nil
}

func hasEnv(p *project.Project, file string) bool {
	for _, env := range p.Resolved.Environments {
		if env.File == file {
			return true
		}
	}
	return false
}

// snapshotFor adds the project and branch context to a snapshot.
func snapshotFor(p *project.Project) (jsonapi.Snapshot, error) {
	snap, err := p.Snapshot()
	if err != nil {
		return snap, err
	}
	snap.Project = p.Location.ProjectID
	snap.ProjectPath = p.Location.ProjectPath
	snap.Branch = p.Location.Branch
	snap.Branches = store.Branches(p.Location)
	for i := range snap.Environments {
		snap.Environments[i].Branch = p.Location.Branch
	}
	return snap, nil
}

// forgetActive clears the project's active pointer when it points at file on this branch.
func forgetActive(loc store.Location, file string) error {
	active, ok := store.ReadActive(loc)
	if !ok || active.Branch != loc.Branch || active.File != file {
		return nil
	}
	return store.WriteActive(loc, store.Active{})
}

// moveActive follows a file rename when the active pointer referred to the old name.
func moveActive(loc store.Location, from, to string) error {
	active, ok := store.ReadActive(loc)
	if !ok || active.Branch != loc.Branch || active.File != from {
		return nil
	}
	return store.WriteActive(loc, store.Active{Branch: loc.Branch, File: to})
}

func runWhere(stdout io.Writer, opts options) error {
	loc, err := store.Locate(opts.cwd, opts.branch)
	if err != nil {
		return err
	}
	if opts.json {
		return writeJSON(stdout, loc)
	}
	fmt.Fprintf(stdout, "Project:  %s\nBranch:   %s\nEnv dir:  %s\n", loc.ProjectPath, loc.Branch, loc.Dir)
	return nil
}

type branchesResult struct {
	Current  string   `json:"current"`
	Branches []string `json:"branches"`
}

func runBranches(stdout io.Writer, opts options) error {
	loc, err := store.Locate(opts.cwd, opts.branch)
	if err != nil {
		return err
	}
	names := store.Branches(loc)
	if opts.json {
		return writeJSON(stdout, branchesResult{Current: loc.Branch, Branches: names})
	}
	for _, name := range names {
		marker := " "
		if name == loc.Branch {
			marker = "*"
		}
		fmt.Fprintf(stdout, "%s %s\n", marker, name)
	}
	return nil
}

// runOverview lists every branch with its environments and the one active environment.
func runOverview(stdout io.Writer, opts options) error {
	loc, err := store.Locate(opts.cwd, "")
	if err != nil {
		return err
	}
	active, _ := store.ReadActive(loc)
	overview := jsonapi.Overview{
		Ok:            true,
		Project:       loc.ProjectID,
		ProjectPath:   loc.ProjectPath,
		CurrentBranch: loc.Branch,
		ActiveBranch:  active.Branch,
		ActiveFile:    active.File,
		Branches:      []jsonapi.Snapshot{},
	}
	for _, name := range store.Branches(loc) {
		branchLoc := loc
		branchLoc.Branch = name
		branchLoc.Dir = store.BranchDir(loc, name)
		p, err := openLocation(branchLoc)
		if err != nil {
			return err
		}
		snap, err := snapshotFor(p)
		if err != nil {
			return err
		}
		overview.Branches = append(overview.Branches, snap)
	}

	if opts.json {
		return writeJSON(stdout, overview)
	}
	for _, branch := range overview.Branches {
		header := branch.Branch
		if branch.Branch == overview.CurrentBranch {
			header += " (current)"
		}
		fmt.Fprintf(stdout, "%s\n", header)
		for _, env := range branch.Environments {
			marker := " "
			if env.Active {
				marker = "*"
			}
			fmt.Fprintf(stdout, "  %s %-16s %-24s %d vars\n", marker, env.Name, env.File, len(env.Variables))
		}
	}
	return nil
}

// runAdopt copies project env files into storage. It never deletes anything.
func runAdopt(stdout io.Writer, opts options) error {
	loc, err := store.Locate(opts.cwd, opts.branch)
	if err != nil {
		return err
	}
	rep, err := store.Adopt(loc)
	if err != nil {
		return err
	}
	result := jsonapi.AdoptResult{
		Ok:      true,
		Dir:     loc.Dir,
		Copied:  orEmpty(rep.Copied),
		Same:    orEmpty(rep.Same),
		Skipped: orEmpty(rep.Skipped),
	}
	if opts.json {
		return writeJSON(stdout, result)
	}
	fmt.Fprintf(stdout, "Stored in %s\n", loc.Dir)
	fmt.Fprintf(stdout, "Copied: %s\n", joinOrNone(result.Copied))
	if len(result.Same) > 0 {
		fmt.Fprintf(stdout, "Already stored (identical): %s\n", strings.Join(result.Same, ", "))
	}
	if len(result.Skipped) > 0 {
		fmt.Fprintf(stdout, "Kept in project (differs from stored copy): %s\n", strings.Join(result.Skipped, ", "))
	}
	return nil
}

// runPurge deletes project copies that are identical to their stored copy.
// It needs --yes unless --dry-run is given.
func runPurge(stdout io.Writer, opts options) error {
	if !opts.dryRun && !opts.yes {
		return usageErr("purge deletes project env files; use --dry-run to preview, or --yes to confirm")
	}
	loc, err := store.Locate(opts.cwd, opts.branch)
	if err != nil {
		return err
	}
	rep, err := store.Purge(loc, opts.dryRun)
	if err != nil {
		return err
	}
	result := jsonapi.PurgeResult{
		Ok:      true,
		DryRun:  opts.dryRun,
		Matched: orEmpty(rep.Matched),
		Deleted: orEmpty(rep.Deleted),
		Kept:    orEmpty(rep.Kept),
	}
	if opts.json {
		return writeJSON(stdout, result)
	}
	if opts.dryRun {
		fmt.Fprintf(stdout, "Would delete from project: %s\n", joinOrNone(result.Matched))
	} else {
		fmt.Fprintf(stdout, "Deleted from project: %s\n", joinOrNone(result.Deleted))
	}
	if len(result.Kept) > 0 {
		fmt.Fprintf(stdout, "Kept in project (not stored, or differs from stored copy): %s\n", strings.Join(result.Kept, ", "))
	}
	return nil
}

func runSwap(stdout io.Writer, opts options, args []string) error {
	restore := opts.restore
	envSelector := ""
	forceCopy := false
	force := opts.force
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--copy":
			forceCopy = true
		case "--env":
			if i+1 >= len(args) {
				return usageErr("--env requires a value")
			}
			i++
			envSelector = args[i]
		}
	}

	loc, err := store.Locate(opts.cwd, opts.branch)
	if err != nil {
		return err
	}

	// Read swap state from sidecar.
	sidecar, err := metadata.Load(loc.Dir)
	if err != nil {
		return err
	}

	if restore {
		if sidecar.Swap == nil {
			return usageErr("no active swap to restore")
		}
		state := swap.State{
			Target: sidecar.Swap.Target,
			Backup: sidecar.Swap.Backup,
			Mode:   sidecar.Swap.Mode,
			Hash:   sidecar.Swap.Hash,
		}
		if err := swap.Restore(loc.ProjectPath, state, force); err != nil {
			return err
		}
		// Clear swap state.
		sidecar.Swap = nil
		if err := metadata.Save(loc.Dir, sidecar); err != nil {
			return err
		}
		if !opts.quiet {
			fmt.Fprintln(stdout, "Restored .env from backup.")
		}
		return nil
	}

	// Enable swap.
	p, err := project.Open(loc.Dir)
	if err != nil {
		return err
	}
	if envSelector == "" {
		if p.Resolved.Active == "" {
			return usageErr("no active environment; use --env <env>")
		}
		envSelector = p.Resolved.Active
	}
	env, err := p.Find(envSelector)
	if err != nil {
		return err
	}

	backupName := ".env.backup"
	if _, err := os.Stat(filepath.Join(loc.ProjectPath, backupName)); err == nil {
		backupName = ".env.backup." + strings.ReplaceAll(loc.Branch, "/", "-")
	}

	state, err := swap.Enable(loc.ProjectPath, env.File, backupName, forceCopy)
	if err != nil {
		return err
	}

	// Save swap state.
	sidecar.Swap = &metadata.SwapState{
		Target: state.Target,
		Backup: state.Backup,
		Mode:   state.Mode,
		Hash:   state.Hash,
	}
	if err := metadata.Save(loc.Dir, sidecar); err != nil {
		return err
	}

	if !opts.quiet {
		fmt.Fprintf(stdout, "Swapped .env -> %s (%s)\n", env.File, state.Mode)
		fmt.Fprintf(stdout, "Backup: %s\n", state.Backup)
		fmt.Fprintln(stdout, "Layering is bypassed while swap is active.")
	}
	return nil
}

func runGitignore(stdout io.Writer, opts options, args []string) error {
	all := false
	for _, arg := range args {
		if arg == "--all" {
			all = true
		}
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}

	var files []string
	if all {
		for _, env := range p.Resolved.Environments {
			files = append(files, env.File)
		}
	} else {
		if p.Resolved.Active != "" {
			files = append(files, p.Resolved.Active)
		}
		for _, layer := range layers.Files(p.Location.ProjectPath, p.Resolved.Active) {
			if layer != p.Resolved.Active {
				files = append(files, layer)
			}
		}
	}

	// Only warn about files that actually contain plaintext values.
	var plaintext []string
	for _, file := range files {
		full := filepath.Join(p.Location.Dir, filepath.FromSlash(file))
		if gitignore.HasPlaintext(full) {
			plaintext = append(plaintext, file)
		}
	}
	notIgnored := gitignore.Check(p.Location.ProjectPath, plaintext)
	if len(notIgnored) == 0 {
		if !opts.quiet && !opts.json {
			fmt.Fprintln(stdout, "All plaintext env files are gitignored.")
		}
		if opts.json {
			return writeJSON(stdout, map[string]any{"notIgnored": []string{}, "added": []string{}})
		}
		return nil
	}

	if opts.dryRun {
		if opts.json {
			return writeJSON(stdout, map[string]any{"notIgnored": notIgnored, "added": []string{}})
		}
		fmt.Fprintf(stdout, "Would add to .gitignore: %s\n", strings.Join(notIgnored, ", "))
		return nil
	}

	added, err := gitignore.Add(p.Location.ProjectPath, notIgnored)
	if err != nil {
		return err
	}
	if opts.json {
		return writeJSON(stdout, map[string]any{"notIgnored": notIgnored, "added": added})
	}
	if !opts.quiet {
		fmt.Fprintf(stdout, "Added to .gitignore: %s\n", strings.Join(added, ", "))
	}
	return nil
}

func runList(stdout io.Writer, opts options) error {
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	snap, err := snapshotFor(p)
	if err != nil {
		return err
	}
	if opts.json {
		return writeJSON(stdout, snap)
	}

	fmt.Fprintf(stdout, "Branch: %s\n", snap.Branch)
	if len(snap.Environments) == 0 {
		fmt.Fprintln(stdout, "No environment files found. Create one with: envh create <name>")
		return nil
	}
	for _, env := range snap.Environments {
		marker := " "
		if env.Active {
			marker = "*"
		}
		suffix := ""
		if n := countEncrypted(env); n > 0 {
			suffix = fmt.Sprintf(" (%d encrypted)", n)
		}
		fmt.Fprintf(stdout, "%s %-16s %-24s %d vars%s\n", marker, env.Name, env.File, len(env.Variables), suffix)
	}
	for _, warning := range snap.Warnings {
		fmt.Fprintf(stdout, "warning: %s\n", warning)
	}
	return nil
}

func runUse(stdout io.Writer, opts options, args []string) error {
	if len(args) != 1 {
		return usageErr("use requires <env>")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	env, err := p.Use(args[0])
	if err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("Active environment: %s on %s (%s)", env.Name, p.Location.Branch, env.File))
}

func runCreate(stdout io.Writer, opts options, args []string) error {
	if len(args) != 1 {
		return usageErr("create requires <name>")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	env, err := p.Create(args[0], opts.activate)
	if err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("Created %s (%s)", env.Name, env.File))
}

func runRename(stdout io.Writer, opts options, args []string) error {
	if len(args) != 2 {
		return usageErr("rename requires <env> <name>")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	env, err := p.Rename(args[0], args[1])
	if err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("Renamed to %s (%s)", env.Name, env.File))
}

func runRenameFile(stdout io.Writer, opts options, args []string) error {
	if len(args) != 2 {
		return usageErr("rename-file requires <env> <file>")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	before, err := p.Find(args[0])
	if err != nil {
		return err
	}
	env, err := p.RenameFile(args[0], args[1])
	if err != nil {
		return err
	}
	if err := moveActive(p.Location, before.File, env.File); err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("Renamed file to %s", env.File))
}

func runDuplicate(stdout io.Writer, opts options, args []string) error {
	if len(args) != 1 {
		return usageErr("duplicate requires <env>")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	env, err := p.Duplicate(args[0], opts.name)
	if err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("Duplicated to %s (%s)", env.Name, env.File))
}

func runRemove(stdout io.Writer, opts options, args []string) error {
	if len(args) != 1 {
		return usageErr("remove requires <env>")
	}
	if !opts.yes {
		return usageErr("remove deletes the env file; pass --yes to confirm")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	env, err := p.Remove(args[0])
	if err != nil {
		return err
	}
	if err := forgetActive(p.Location, env.File); err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("Deleted %s (%s)", env.Name, env.File))
}

func runRestore(stdout io.Writer, opts options, args []string) error {
	if len(args) != 1 {
		return usageErr("restore requires <file>")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	if err := p.Restore(args[0]); err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("Restored %s", args[0]))
}

func runTrash(stdout io.Writer, opts options, args []string) error {
	if len(args) == 0 || (len(args) == 1 && args[0] != "list" && args[0] != "empty") {
		return usageErr("trash requires: list | empty")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		files, err := p.ListTrash()
		if err != nil {
			return err
		}
		if opts.json {
			return writeJSON(stdout, map[string]any{"files": orEmpty(files)})
		}
		if len(files) == 0 {
			fmt.Fprintln(stdout, "Trash is empty.")
			return nil
		}
		for _, f := range files {
			fmt.Fprintf(stdout, "%s\n", f)
		}
		return nil
	case "empty":
		if !opts.yes {
			return usageErr("empty permanently removes trashed files; pass --yes to confirm")
		}
		count, err := p.EmptyTrash()
		if err != nil {
			return err
		}
		if !opts.quiet {
			fmt.Fprintf(stdout, "Removed %d file(s) from trash.\n", count)
		}
		return nil
	}
	return usageErr("trash requires: list | empty")
}

func runColor(stdout io.Writer, opts options, args []string) error {
	if len(args) != 2 {
		return usageErr("color requires <env> <color>")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	env, err := p.SetColor(args[0], args[1])
	if err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("%s is now %s", env.Name, env.Color))
}

func runMove(stdout io.Writer, opts options, args []string) error {
	if len(args) != 2 || (args[1] != "up" && args[1] != "down") {
		return usageErr("move requires <env> up|down")
	}
	delta := 1
	if args[1] == "up" {
		delta = -1
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	env, err := p.Move(args[0], delta)
	if err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("Moved %s %s", env.Name, args[1]))
}

func runImport(stdout io.Writer, opts options, args []string) error {
	if len(args) != 2 || args[0] != "postman" {
		return usageErr("import requires: postman <file>")
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		return err
	}
	envs, err := postman.Parse(data)
	if err != nil {
		return err
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	importOpts := project.ImportOptions{
		Force:        opts.force,
		DryRun:       opts.dryRun,
		SkipDisabled: opts.skipDisabled,
	}
	rep, err := p.ImportPostman(envs, importOpts)
	if err != nil {
		return err
	}

	result := jsonapi.ImportResult{
		Ok:          true,
		DryRun:      opts.dryRun,
		Imported:    orEmpty(rep.Imported),
		Conflicts:   orEmpty(rep.Conflicts),
		SkippedKeys: orEmpty(rep.SkippedKeys),
		Warnings:    orEmpty(rep.Warnings),
	}
	if !opts.dryRun {
		snap, err := snapshotFor(p)
		if err != nil {
			return err
		}
		result.Snapshot = &snap
	}

	if opts.json {
		return writeJSON(stdout, result)
	}
	if opts.dryRun {
		fmt.Fprintf(stdout, "Would import: %s\n", joinOrNone(result.Imported))
	} else {
		fmt.Fprintf(stdout, "Imported: %s\n", joinOrNone(result.Imported))
	}
	if len(result.Conflicts) > 0 {
		fmt.Fprintf(stdout, "Already exist (use --force to overwrite): %s\n", strings.Join(result.Conflicts, ", "))
	}
	if len(result.SkippedKeys) > 0 {
		fmt.Fprintf(stdout, "Skipped keys with unsafe names: %s\n", strings.Join(result.SkippedKeys, ", "))
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(stdout, "warning: %s\n", warning)
	}
	return nil
}

func runResolve(stdout io.Writer, opts options, args []string) error {
	envSelector := opts.env
	if envSelector == "" && len(args) >= 2 && args[0] == "--env" {
		envSelector = args[1]
		args = args[2:]
	}
	if envSelector == "" {
		p, err := openProject(opts)
		if err != nil {
			return err
		}
		if p.Resolved.Active == "" {
			return usageErr("no active environment; pass --env <env>")
		}
		envSelector = p.Resolved.Active
	}

	p, err := openProject(opts)
	if err != nil {
		return err
	}
	env, err := p.Find(envSelector)
	if err != nil {
		return err
	}

	_, _, err = p.Entries(env.File)
	if err != nil {
		return err
	}

	values := make(map[string]string)
	// Load base layer first, then active.
	for _, layer := range layers.Files(p.Location.ProjectPath, env.File) {
		layerEntries, _, err := p.Entries(layer)
		if err != nil {
			continue
		}
		for _, e := range layerEntries {
			if opts.skipEncrypted && e.Encrypted {
				continue
			}
			values[e.Key] = e.Value
		}
	}
	if opts.json {
		return writeJSON(stdout, map[string]any{"environment": env.File, "layers": layers.Files(p.Location.ProjectPath, env.File), "values": values})
	}
	for k, v := range values {
		fmt.Fprintf(stdout, "%s=%s\n", k, v)
	}
	return nil
}

func runValue(stdout io.Writer, opts options, args []string) error {
	if len(args) != 2 {
		return usageErr("value requires <env> <key>")
	}
	if !opts.reveal {
		return usageErr("value prints secrets; pass --reveal to confirm")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	entry, err := p.Value(args[0], args[1])
	if err != nil {
		return err
	}
	if opts.json {
		return writeJSON(stdout, jsonapi.Value{Key: entry.Key, Value: entry.Value})
	}
	fmt.Fprintln(stdout, entry.Value)
	return nil
}

func runSet(stdout io.Writer, opts options, args []string) error {
	if len(args) != 3 {
		return usageErr("set requires <env> <key> <value>")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	if err := p.Set(args[0], args[1], args[2]); err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("Set %s in %s", args[1], args[0]))
}

func runUnset(stdout io.Writer, opts options, args []string) error {
	if len(args) != 2 {
		return usageErr("unset requires <env> <key>")
	}
	p, err := openProject(opts)
	if err != nil {
		return err
	}
	if err := p.Unset(args[0], args[1]); err != nil {
		return err
	}
	return report(stdout, opts, p, fmt.Sprintf("Removed %s from %s", args[1], args[0]))
}

// report prints the refreshed snapshot as JSON, or a short message for humans.
func report(stdout io.Writer, opts options, p *project.Project, message string) error {
	snap, err := snapshotFor(p)
	if err != nil {
		return err
	}
	if opts.json {
		return writeJSON(stdout, jsonapi.Mutation{Ok: true, Snapshot: snap})
	}
	if !opts.quiet {
		fmt.Fprintln(stdout, message)
	}
	return nil
}

func fail(stderr io.Writer, asJSON bool, err error) int {
	code, exit := classify(err)
	if asJSON {
		_ = json.NewEncoder(stderr).Encode(jsonapi.Error{Ok: false, Code: code, Message: err.Error()})
	} else {
		fmt.Fprintf(stderr, "envh: %s\n", err)
	}
	return exit
}

func classify(err error) (string, int) {
	switch {
	case errors.Is(err, errUsage):
		return "USAGE", 1
	case errors.Is(err, project.ErrEnvNotFound):
		return "ENV_NOT_FOUND", 2
	case errors.Is(err, project.ErrKeyNotFound):
		return "KEY_NOT_FOUND", 2
	case errors.Is(err, project.ErrAmbiguous):
		return "ENV_AMBIGUOUS", 2
	case errors.Is(err, project.ErrFileExists):
		return "FILE_EXISTS", 4
	case errors.Is(err, project.ErrInvalidName):
		return "INVALID_NAME", 1
	case errors.Is(err, project.ErrInvalidFile):
		return "INVALID_FILE", 1
	case errors.Is(err, project.ErrInvalidColor):
		return "INVALID_COLOR", 1
	case errors.Is(err, envfile.ErrEncrypted):
		return "ENCRYPTED_READ_ONLY", 4
	case errors.Is(err, envfile.ErrInvalidKey):
		return "INVALID_KEY", 1
	case errors.Is(err, postman.ErrInvalid):
		return "POSTMAN_INVALID", 3
	case errors.Is(err, metadata.ErrInvalid):
		return "METADATA_INVALID", 3
	case errors.Is(err, os.ErrNotExist):
		return "NOT_FOUND", 2
	default:
		return "FAILED", 5
	}
}

func usageErr(message string) error {
	return fmt.Errorf("%w: %s", errUsage, message)
}

func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "nothing"
	}
	return strings.Join(values, ", ")
}

func countEncrypted(env jsonapi.Environment) int {
	count := 0
	for _, variable := range env.Variables {
		if variable.Encrypted {
			count++
		}
	}
	return count
}

func writeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func usage() string {
	return fmt.Sprintf(`envh %s
Standalone environment manager for .env files.

Usage:
  envh [--cwd <dir>] [--branch <name>] [--json] <command> [options]

Storage (env files live outside the project, one folder per branch):
  where                          Show where this branch's env files are stored
  branches                       List branches that have env files
  overview                       List every branch and its environments
  adopt                          Copy .env files from the project into storage (never deletes)
  purge --dry-run | --yes        Delete project copies that are identical to the stored copy
  gitignore [--all] [--dry-run]  Add env files to .gitignore (active + base, or all with --all)

Environments:
  list, snapshot                 Show environments and variable keys (no values)
  create <name> [--activate]     Create .env.<name>
  use <env>                      Set the active environment (--branch selects another branch)
  rename <env> <name>            Change the display name
  rename-file <env> <file>       Rename the file on disk (.env.<name>)
  duplicate <env> [--name <n>]   Copy an environment to a new file
  remove <env> --yes             Move an environment file to trash
  restore <file>                 Restore a file from trash
  trash list                     List trashed files
  trash empty --yes              Permanently remove trashed files
  color <env> <color>            gray, green, yellow, red, blue, purple, orange
  move <env> up|down             Change the order in the list
  swap [--env <env>] [--copy]    Replace .env with the active env (backup first)
  swap --restore [--force]       Restore .env from backup

Variables:
  value <env> <key> --reveal     Print a value
  set <env> <key> -- <value>     Set a value, adding the key when missing
  unset <env> <key>              Remove every occurrence of a key

Import:
  import postman <file> [--dry-run] [--force] [--skip-disabled]

Options:
  --cwd <dir>      Project directory (default: current directory)
  --branch <name>  Use another branch's env files instead of the current one
  --json           Machine-readable output; errors go to stderr as JSON
  --reveal         Allow value to print a secret
  --activate       Activate a newly created environment
  --force          Overwrite existing files during import
  --dry-run        Show what an import or purge would do without writing
  --skip-disabled  Leave out disabled Postman values
  --quiet, -q      Suppress confirmation text
  --help, -h       Show this help
  --version, -v    Show the version

Put -- before values that start with a dash:
  envh set Local KEY -- -value
`, version.Version)
}
