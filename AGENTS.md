# AGENTS.md

Instructions for agents working in this repository.

## Project

Standalone Postman-style environment manager for `.env` files.

- Real `.env` files are the source of truth.
- Metadata lives in `.envh.json` and never contains values.
- Security-sensitive core: **Go** in `cmd/` and `internal/`.
- Thin VS Code client: **TypeScript** in `packages/vscode`.
- Authoritative design: `env-manager-plan.md`.

Current state: Phase 0 complete (Go scaffold, TypeScript extension shell, build/package pipeline). Phase 1 is in progress; `internal/envfile` is implemented. Verify the actual files before assuming a phase is complete.

## Hard rules

1. **No dotenvx.** No dependency, import, CLI call, or decryption through dotenvx.
2. **Do not patch or fork `dotenv-vscode`.** This is a standalone project.
3. **No values in metadata or logs.** `.envh.json` stores names, paths, colors, order, active state, and swap state only.
4. **Encrypted values are read-only.** Detect `encrypted:`, show a lock, preserve the line, never rewrite or decrypt.
5. **No network calls or telemetry.**
6. **The extension must not parse `.env` content.** It invokes the Go CLI and consumes JSON.
7. **Core code must not depend on Node.** Only the extension and build scripts use Node.
8. **No `.env` parser dependency.** The offset-preserving parser is local Go code.
9. Go 1.23+; TypeScript for VS Code; Node 18+ for the extension host and build scripts.

## Layout

```text
cmd/envh/                 Go CLI entry point
internal/
  cli/                    command dispatch, terminal/JSON output
  project/                discovery + metadata + layers orchestration
  discovery/              recursive .env discovery
  envfile/                offset-preserving parser/editor
  metadata/               .envh.json schema and reconciliation
  layers/                 base + active resolution and merging
  runner/                 os/exec + signal forwarding
  swap/                   symlink/copy, backup, restore
  postman/                Postman environment import
  gitignore/              git check-ignore + conservative fallback
  shell/                  exports and quoting
  jsonapi/                stable DTOs for the extension
  fsx/                    atomic writes, paths, backups
  errs/                   typed error codes
packages/vscode/
  src/                    TypeScript extension
  media/icon.svg
  bin/                    generated Go binary, not committed
  dist/                   generated extension bundle, not committed
scripts/                  build/package/install/guards (Node)
```

Keep pure logic separate from filesystem and process code.

## Commands

```sh
go test ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
gofmt -l cmd internal     # must print nothing
go build -o packages/vscode/bin/envh ./cmd/envh

npm install
npm test
npm run lint
npm run check:no-dotenvx
npm run build
npm run package:vsix
npm run install:vsix
npm run verify
```

Run the CLI from the checkout:

```sh
go run ./cmd/envh --help
# or after build:
packages/vscode/bin/envh --version
```

Install the extension locally:

```sh
code --install-extension dist/env-handler-vscode-0.1.0.vsix --force
```

Then run **Developer: Reload Window**.

## Architecture

- The Go CLI is the only code that reads, parses, edits, merges, or writes `.env` files.
- The TypeScript extension is UI + adapters: tree, status bar, debug provider, commands, task execution.
- The extension spawns the bundled binary with `execFile` and `--json`; never use a shell for CLI calls.
- The binary is built to `packages/vscode/bin/envh` (or `envh.exe`) and included in the VSIX.
- Build scripts use esbuild for `src/extension.ts` and `vscode` is external.

## CLI JSON contract

- `envh snapshot --json`: one branch's metadata, environments, layers, variable keys, encrypted flags, line numbers. **No values.** Each environment has a `branch` field.
- `envh overview --json`: every branch of the project, plus the single active environment (`activeBranch`, `activeFile`). The extension uses this for the panel and tree.
- Commands act on the current branch unless `--branch <name>` is given. The extension always passes the branch an environment belongs to.
- The active environment is stored once per project in `project.json` (`active.branch`, `active.file`); it can belong to any branch.
- `envh value <env> <key> --reveal --json`: explicit single value reveal.
- `envh resolve --env <env> --json --skip-encrypted`: merged values for debug/run.
- Mutations (`use`, `create`, `set`, `unset`) return `{ "ok": true, "snapshot": { ... } }` when `--json`.
- Put `--` before values that may start with a dash (the extension always does: `set <env> <key> -- <value>`).
- Errors return `{ "ok": false, "code": "...", "message": "..." }` on stderr; no values.
- DTO changes require updating both Go structs and TypeScript types.

## Data and layering

- Env files and the sidecar are NOT in the project. They live in the local store: `<config dir>/env-handler/projects/<project-id>/branches/<escaped branch>/` (`ENVH_HOME` overrides the root; tests use it). The sidecar is `.envh.json` in that branch folder.
- The branch comes from git (`--branch` overrides it). Detached HEAD uses `detached-<sha>`; no git uses `default`.
- Snapshot `root` is the store folder; the extension must use it for file paths, not the workspace folder.
- Tests must never write to the real store: `internal/cli/main_test.go` sets `ENVH_HOME` to a temp folder.
- Discovery: `**/.env`, `**/.env.*`; exclude examples, templates, `.env.keys*`, `.env.vault`, `.envh.json`, `node_modules`, `.git`.
- Base layer: `.env` in the same directory as the active file; active loads after it.
- Swap loads only the swap target and bypasses layering.
- Runner: file values override the parent environment by default; `--no-override` reverses.
- Debug: merged file values, then `config.env` wins.
- Encrypted values: runner passes them through with a warning; debug skips them and warns.

## Go style

- `gofmt` and `go vet` are mandatory.
- Prefer the standard library. Only add a dependency when clearly justified.
- Glob matching may use `github.com/bmatcuk/doublestar/v4`; nothing else is pre-approved.
- Typed error codes live in `internal/errs`; messages must not contain values.
- Atomic writes: temp file in the same directory + rename.
- Preserve comments, blank lines, quoting, `export` prefixes, CRLF, BOM, and trailing newline.
- Keep JSON DTOs in `internal/jsonapi`, not ad hoc maps.
- Avoid global mutable state; pass a project root/config explicitly.

## TypeScript style

- `strict` TypeScript; `tsc --noEmit` must pass.
- No runtime npm dependencies in the extension.
- No `shell: true`; use `execFile`.
- Never log stdout/stderr from a command that may contain values.
- All mutations go through the CLI; do not duplicate parser or metadata logic.
- Settings use the `envHandler.*` prefix.
- Values are masked by default; reveal is transient.
- Abort mutations when the open document is dirty.

## Testing

- Go tests use the standard `testing` package and temporary directories.
- Tests must not touch the real workspace and must stay offline.
- Cover parser formatting, metadata reconciliation, layering, runner env precedence, swap conflicts, Postman import, gitignore, and CLI JSON.
- Type-check the extension with `npm run lint:ts`.
- No VS Code download in CI.
- Before finishing: `gofmt -l cmd internal` empty, `go vet ./cmd/... ./internal/...`, `go test ./cmd/... ./internal/...`, `npm run lint`, `npm run check:no-dotenvx`, and `npm run build` when extension/binary code changed.

## Extension conventions

- Settings prefix: `envHandler.*`.
- Debug integration defaults to off.
- `terminalMode`: `none` | `task` | `swap`.
- `extensionKind`: workspace; virtual workspaces are unsupported.
- Untrusted workspaces are read-only: no edits, runner, swap, git, or import.
- Multi-root: one independent active environment per workspace folder.
- Single-root: flat environment list.
- Do not add a custom editor, language contribution, cloaking, or autocomplete; the official Dotenv extension may coexist.
- The VSIX contains a platform-specific Go binary. Document that limitation.

## Safety checklist

- No secrets in metadata, tree tooltips, errors, or logs by default.
- Warn when a plaintext layer is not gitignored; never edit `.gitignore` without explicit user action.
- Swap must confirm, back up, store state, and support restore.
- Do not auto-restore swap on deactivate.
- Do not overwrite a corrupt `.envh.json`; report it.

## Workflow

- Work in small phases (see `env-manager-plan.md`).
- Update `README.md`, `CHANGELOG.md`, and `env-manager-plan.md` when behavior or commands change.
- If a requirement is ambiguous or conflicts with these rules, ask before implementing.
