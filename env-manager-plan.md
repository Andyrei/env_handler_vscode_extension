# Env Handler — Standalone `.env` Environment Manager

**Status:** Phase 2 in progress (core + extension functional, tests partial)
**Created:** 2026-10-23
**Updated:** 2026-10-23
**Repo:** `/Users/andydrei/Documents/MyADHD/running-tools/env_handler_vs_extension`

---

## 1. What it is

A standalone environment manager for `.env` files. Real `.env` files are the source of truth.
Metadata lives in `.envh.json` and never contains values. Env files are stored **outside the project**,
one folder per git branch, so they don't get committed or carried across branches.

- Core: Go CLI (`envh`) in `cmd/` and `internal/`.
- Extension: thin TypeScript client in `packages/vscode`.
- No dotenvx. No fork of `dotenv-vscode`.

## 2. Implemented

### Core (Go)

- `envfile`: offset-preserving parser/editor with formatting tests.
- `metadata`: sidecar schema, reconcile, names, colors, order, swap state.
- `discovery`: recursive `.env` discovery with include/exclude.
- `layers`: base + active resolution, merge, encrypted detection.
- `runner`: native `os/exec` runner with signal forwarding.
- `swap`: symlink/copy swap, backup, hash validation, restore.
- `postman`: Postman environment import.
- `gitignore`: git check-ignore + fallback matcher, HasPlaintext.
- `store`: per-project, per-branch storage outside the repo. Adopt (copy), purge (delete identical copies).
- `trash`: move deleted files to trash, restore, empty.
- `project`: orchestration, snapshot, CRUD, import, swap, purge.
- `cli`: all commands with JSON output and exit codes.

### CLI commands

`where`, `branches`, `overview`, `adopt`, `purge`, `list`, `snapshot`, `use`, `create`,
`rename`, `rename-file`, `duplicate`, `remove`, `restore`, `trash`, `color`, `move`,
`import postman`, `gitignore`, `value`, `set`, `unset`, `resolve`, `run`, `export`, `swap`.

### Extension (TypeScript)

- Activity-bar panel with branch-grouped environments.
- Tree view with envs and variables.
- Status bar with active env + branch, click to switch.
- Panel: inline rename, color picker, variable CRUD, create, import Postman.
- Adopt, purge, gitignore, swap, restore commands.
- Debug integration (opt-in, injects merged env).
- Run with environment (terminal).
- Cross-branch activation.

### Tests

- Go: parser, metadata, discovery, CLI integration, store, postman, gitignore, branches.
- TypeScript: type-check only.

## 3. Not yet done

- **Docs**: env-manager-plan.md is out of date.
- **Extension tests**: panel and tree not functionally tested.
- **Repo move/relink**: envs keyed by folder path.
- **Multi-platform VSIX**: only macOS arm64.
- **Sync across machines**: storage is local only.
- **Swap UI**: CLI only, no extension panel integration.
- **Layering at runtime**: resolve is built but F5 injection needs testing.
- **Search/filter** in panel.
- **Environment templates**.
- **Diff between environments**.

## 4. Key decisions

- Core: Go. Extension: TypeScript.
- Env files outside project, one folder per branch.
- Active env stored once per project in `project.json`.
- No dotenvx. No fork of dotenv-vscode.
- Trash instead of permanent delete.
- Debug integration opt-in, default off.
- `encrypted:` values read-only.

## 5. Verification

```sh
go test ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
gofmt -l cmd internal
npm run lint
npm run check:no-dotenvx
npm run build
npm run package:vsix
```
