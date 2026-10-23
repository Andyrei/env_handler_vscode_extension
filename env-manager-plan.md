# Env Handler — Standalone `.env` Environment Manager

**Status:** Phase 0 complete; Phase 1 in progress (parser implemented)
**Created:** 2026-10-23
**Updated:** 2026-10-23
**Repo:** `/Users/andydrei/Documents/MyADHD/running-tools/env_handler_vs_extension`
**Supersedes:** the earlier JavaScript and `dotenv-vscode` plans

---

## 1. Constraints

- **Standalone project.** Build here. Do not patch or fork `dotenv-vscode`.
- **No dotenvx.** No dependency, import, CLI call, or decryption through it.
- **Secure core.** All `.env` parsing, editing, metadata, layering, running, swapping, importing, and gitignore checks live in Go.
- **Thin VS Code extension.** The extension is TypeScript because VS Code requires a Node-based extension host. It contains no `.env` parsing or secret-handling logic; it invokes the Go CLI.
- **Real `.env` files are the source of truth.** No duplicated secret store.
- **Metadata only in `.envh.json`.** Names, paths, active state, colors, order, and swap state. Never values.
- **Encrypted values are read-only.** Detect `encrypted:`, show a lock, preserve the line, never rewrite or decrypt.
- **Native runtime.** The Go runner loads layers and spawns commands.
- **Offline.** No network calls, no telemetry.

---

## 2. Architecture

```text
env_handler_vs_extension/
├── go.mod                          # module env-handler
├── cmd/envh/main.go                # CLI entry point
├── internal/
│   ├── cli/                        # command parsing, dispatch, JSON output
│   ├── project/                    # project root + sidecar orchestration
│   ├── discovery/                  # recursive .env discovery
│   ├── envfile/                    # offset-preserving parser/editor
│   ├── metadata/                   # .envh.json schema + reconciliation
│   ├── layers/                     # base + active resolution and merging
│   ├── runner/                     # os/exec + signal forwarding
│   ├── swap/                       # symlink/copy, backup, restore
│   ├── postman/                    # Postman environment import
│   ├── gitignore/                  # git check-ignore + fallback
│   ├── shell/                      # export formatting and quoting
│   ├── jsonapi/                    # DTOs consumed by the extension
│   ├── fsx/                        # atomic writes, paths, backup naming
│   ├── errs/                       # typed error codes
│   └── version/
├── packages/vscode/
│   ├── package.json
│   ├── tsconfig.json
│   ├── src/
│   │   ├── extension.ts
│   │   ├── cli.ts                  # spawn Go binary, JSON request/response
│   │   ├── store.ts
│   │   ├── tree.ts
│   │   ├── status.ts
│   │   ├── commands.ts
│   │   ├── debug.ts
│   │   ├── tasks.ts
│   │   ├── mask.ts
│   │   └── config.ts
│   ├── media/icon.svg
│   ├── bin/                        # generated envh binary, not committed
│   ├── dist/                       # generated extension bundle, not committed
│   ├── .vscodeignore
│   ├── README.md
│   └── LICENSE
├── scripts/
│   ├── build.js                    # go build + esbuild
│   ├── package.js                  # vsce package
│   ├── install.js                  # code --install-extension
│   ├── check-gofmt.js
│   └── check-no-dotenvx.js
├── .github/workflows/ci.yml
├── README.md
├── CHANGELOG.md
├── DEVELOPMENT.md
├── AGENTS.md
├── LICENSE
└── .gitignore
```

Language choices:

- **Go 1.23+** for the core and CLI. Installed development toolchain: Go 1.27.3.
- **TypeScript** for the VS Code extension, bundled with esbuild.
- Go tests use `go test ./cmd/... ./internal/...` so Node dependencies are never scanned.
- Extension type-checking uses `tsc --noEmit`.
- External Go dependency: `github.com/bmatcuk/doublestar/v4` for glob matching. Everything else is standard library.
- No Node runtime dependencies in the extension; it ships the Go binary.

---

## 3. Security boundary

The extension never parses or edits `.env` content itself. It calls the CLI and consumes JSON.

```text
VS Code extension (TypeScript)
        │  spawn envh ... --json
        ▼
Go binary (parsing, editing, metadata, runner, swap)
        │
        ▼
.env / .envh.json on disk
```

Consequences:

- No secret values cross into the extension unless an explicit reveal action requests them.
- All formatting-sensitive writes happen in one code path.
- The CLI is independently testable and usable in terminals and CI.
- The VSIX bundles the Go binary for the build platform.

---

## 4. Data model

### 4.1 Sidecar — `<project-root>/.envh.json`

```json
{
  "version": 1,
  "active": ".env.staging",
  "environments": [
    { "file": ".env", "name": "Default", "color": "gray", "order": 0 },
    { "file": ".env.local", "name": "Local", "color": "green", "order": 1 },
    { "file": ".env.staging", "name": "Staging", "color": "yellow", "order": 2 }
  ],
  "swap": {
    "target": ".env.staging",
    "backup": ".env.backup.20261023T120000",
    "mode": "symlink",
    "hash": null
  }
}
```

Rules:

- No values, ciphertext, or tokens.
- `swap` is optional and present only while swap is active.
- Created lazily on the first metadata change.
- Files discovered on disk but absent from the sidecar appear with defaults.
- Entries whose files disappear are dropped during reconciliation.
- Missing `active` falls back to `.env`, then the first environment, with one warning.
- Corrupt sidecar is reported, never overwritten.
- Paths are project-relative POSIX paths.
- No `hidden` field in v1.

### 4.2 Discovery

Default include:

```json
["**/.env", "**/.env.*"]
```

Default exclude:

```text
**/node_modules/**
**/.git/**
**/.env.example
**/.env.*.example
**/.env.sample
**/.env.template
**/.env.dist
**/.env.keys*
**/.env.vault
**/.envh.json
```

`.envrc` is not matched by `**/.env.*`. Symlinked files are included; symlinked directories are skipped to avoid loops.

### 4.3 Layering

1. Active `.env` → load only it.
2. Otherwise, load `.env` in the **same directory** as the active file.
3. Then load the active file.
4. Later values win.
5. Swap active → load only the swap target; layering is bypassed.
6. Runner: file values override the parent environment by default; `--no-override` reverses.
7. Debug: merged file values, then `config.env` wins.
8. Exports: base + active merged.

---

## 5. Go packages

| Package | Responsibility |
| --- | --- |
| `internal/envfile` | Pure offset-preserving parser/editor; no filesystem. |
| `internal/metadata` | Sidecar schema, normalize, merge, reconcile, names, colors, order. |
| `internal/discovery` | Recursive walk, include/exclude, symlink safety. |
| `internal/layers` | Base + active resolution, merge, encrypted detection. |
| `internal/runner` | Spawn command with merged environment, forward signals and exit code. |
| `internal/swap` | Symlink/copy swap, backup, hash validation, restore. |
| `internal/postman` | Postman environment JSON import. |
| `internal/gitignore` | `git check-ignore` and a conservative fallback matcher. |
| `internal/shell` | Shell exports, env-file output, quoting, command generation. |
| `internal/fsx` | Atomic text writes, backup names, safe rename/delete. |
| `internal/project` | Orchestrates discovery + metadata + layers into snapshots. |
| `internal/jsonapi` | Stable JSON DTOs for the extension. |
| `internal/cli` | Command dispatch, terminal output, JSON output, exit codes. |

Pure logic stays separate from filesystem and process code.

---

## 6. CLI contract

Global flags: `--cwd <dir>`, `--json`, `--quiet`, `--help`, `--version`.

```sh
envh init
envh snapshot [--json]
envh list [--json]
envh status
envh use <name|file>
envh create <name> [--file .env.foo] [--activate] [--template KEY=,OTHER=]
envh rename <env> <name>
envh rename-file <env> <file>
envh duplicate <env> [--name <name>]
envh remove <env> [--delete-file --yes]

envh vars [env] [--reveal] [--json]
envh value <env> <key> [--reveal] [--json]
envh set <env> <key> <value> [--json]
envh unset <env> <key> [--json]
envh rename-var <env> <old> <new> [--json]

envh resolve [--env <name|file>] [--no-base] [--skip-encrypted] [--json]
envh run [--env <name|file>] [--no-base] [--no-override] [--quiet] -- <command ...>
envh export [--env <env>] [--no-base] [--format shell|json|env]

envh swap [--env <env>] [--copy] [--force]
envh restore [--force]
envh import postman <file> [--name <name>] [--force] [--activate] [--skip-disabled]
envh gitignore [--all] [--dry-run]
envh doctor [--json]
```

### 6.1 Snapshot JSON (no values)

```json
{
  "root": "/project",
  "sidecar": ".envh.json",
  "active": ".env.local",
  "environments": [
    {
      "file": ".env.local",
      "name": "Local",
      "color": "green",
      "order": 1,
      "active": true,
      "swapped": false,
      "layers": [".env", ".env.local"],
      "variables": [
        { "key": "API_URL", "line": 2, "encrypted": false, "multiline": false },
        { "key": "TOKEN", "line": 3, "encrypted": true, "multiline": false }
      ]
    }
  ],
  "warnings": []
}
```

### 6.2 Other JSON

- `envh value ... --reveal --json` → `{ "key": "...", "value": "..." }`.
- `envh resolve --json` → `{ "environment": ".env.local", "layers": [...], "values": { ... } }`.
- Mutations return `{ "ok": true, ... }` plus the refreshed snapshot when `--json`.
- Errors return `{ "ok": false, "code": "ENV_NOT_FOUND", "message": "..." }` on stderr; never values.

### 6.3 Exit codes

- `0` success
- `1` usage error
- `2` environment or file not found
- `3` metadata invalid
- `4` refused unsafe operation (encrypted edit, swap conflict, existing backup)
- `5` command failed

### 6.4 Behavior

- `list` prints active marker, display name, relative file, variable count, encrypted count. Never values.
- `use` accepts display name, filename, or relative path.
- `create` writes an empty file by default; `--template` seeds lines.
- `remove` removes metadata only by default; `--delete-file --yes` deletes.
- `vars` masks by default; `--reveal` prints values explicitly.
- Edits preserve comments, blank lines, quoting, `export` prefixes, CRLF, BOM, and trailing newlines.
- Encrypted entries reject edits, renames, and deletes.
- `resolve` is the debug/env-injection path; it prints values only when explicitly requested (or `--json`), and skips encrypted keys with `--skip-encrypted`.
- `run` loads layers, warns about encrypted values, inherits stdio, forwards signals, and exits with the child code.
- `swap` backs up `.env`, creates a relative symlink or a copy, and warns that layering is bypassed.
- `restore` validates the generated file/hash and restores the backup.
- `import postman` supports one environment or an array, comments out disabled values by default, sanitizes keys, warns about plaintext, and writes metadata.
- `gitignore` appends specific files or all env files; `--dry-run` prints only.
- `doctor` reports sidecar validity, active file, layers, gitignore coverage, encrypted counts, and swap state.

---

## 7. VS Code extension

### 7.1 Scope

Adds:

- Activity-bar container **Environments** (`envHandler`), view `envHandler.tree`.
- Folder → environment → variable tree.
- Active marker and click-to-activate.
- Masked values with transient reveal.
- Encrypted lock, read-only.
- Environment and variable CRUD.
- Color and order controls.
- Filter, refresh, empty states.
- Multi-root support with one active environment per folder.
- Status-bar chip scoped to the active editor’s folder.
- Debug env injection.
- Run/task integration.
- Swap/restore.
- Postman import.
- Gitignore guard.

Does **not** add: language, custom editor, cloaking, autocomplete, syntax highlighting.

### 7.2 CLI client

`src/cli.ts`:

- Resolves `bin/envh` or `bin/envh.exe` from the extension directory.
- Spawns with `execFile` (no shell).
- Passes `--json` and parses stdout.
- Captures stderr; never logs values.
- Applies timeouts to snapshot calls and a longer/no timeout to `run`.
- Surfaces typed errors to user-facing messages.

### 7.3 Tree

- Single workspace folder → flat environment list.
- Multi-root → folder nodes, independent active state.
- Environment item: color icon, name, `ACTIVE`, variable count, encrypted count, `SWAPPED`.
- Click activates; twisty expands variables.
- Variable: key, masked value, encrypted lock.
- Mask character/length from settings.
- Reveal calls `envh value ... --reveal --json` and clears on refresh.
- Dirty open document → abort mutation and ask to save.
- External changes → watchers refresh snapshot.

### 7.4 Commands

```text
envHandler.refresh
envHandler.createEnvironment
envHandler.renameEnvironment
envHandler.renameEnvironmentFile
envHandler.duplicateEnvironment
envHandler.deleteEnvironment
envHandler.setActive
envHandler.setColor
envHandler.moveEnvironmentUp
envHandler.moveEnvironmentDown
envHandler.revealFile
envHandler.openInEditor
envHandler.openMetadata

envHandler.addVariable
envHandler.editVariable
envHandler.renameVariable
envHandler.deleteVariable
envHandler.revealValue
envHandler.copyValue

envHandler.copyAsShellExports
envHandler.copyRunCommand
envHandler.filter
envHandler.more

envHandler.toggleDebugIntegration
envHandler.runWithEnvironment
envHandler.swapActiveEnvironment
envHandler.restoreSwap
envHandler.importPostman
envHandler.addToGitignore
```

### 7.5 Settings

| Key | Type | Default |
| --- | --- | --- |
| `envHandler.applyToDebug` | boolean | `false` |
| `envHandler.terminalMode` | enum | `none` |
| `envHandler.maskValues` | boolean | `true` |
| `envHandler.maskIcon` | string | `█` |
| `envHandler.maskLength` | number | `32` |
| `envHandler.defaultTemplate` | string[] | `[]` |
| `envHandler.include` | string[] | `["**/.env","**/.env.*"]` |
| `envHandler.exclude` | string[] | discovery exclusions |
| `envHandler.debugTypes` | string[] | common runtime types |
| `envHandler.runCommand` | string | `""` |
| `envHandler.layerBase` | boolean | `true` |
| `envHandler.showStatusBar` | boolean | `true` |
| `envHandler.confirmSwap` | boolean | `true` |

`terminalMode`: `none` | `task` | `swap`.

### 7.6 Debug

- `DebugConfigurationProvider` for common runtime types, filtered by `envHandler.debugTypes`.
- Calls `envh resolve --env <file> --json --skip-encrypted`.
- Injects `config.env`; explicit `config.env` wins.
- Removes a managed `envFile` reference; leaves custom paths alone.
- Never edits `launch.json`.
- Disabled in untrusted workspaces.
- Default off.

### 7.7 Run/task

- `envHandler.runWithEnvironment` prompts for an environment and command.
- Executes the bundled `envh run --env <file> -- <command>` in a VS Code task/terminal.
- `terminalMode: task` registers an `envh` task provider.
- `terminalMode: none` shows guidance instead.
- `terminalMode: swap` controls swap behavior instead.
- No global CLI install required.

### 7.8 Swap

- Opt-in, confirmed, backed up.
- Symlink with copy fallback; content hash for copies.
- State in `.envh.json`; target marked `SWAPPED`.
- Warns about layering bypass and tracked `.env` changes.
- Restore validates and restores.
- Switching away from swap mode prompts to restore.
- No auto-restore on deactivate.

### 7.9 Gitignore guard

- Uses `git check-ignore` for trusted repos.
- Conservative fallback when git is unavailable/untrusted.
- Warns on create/activate/import when a plaintext layer is not ignored.
- Offers add-this, add-all, or not-now; never appends automatically.

---

## 8. Format-preserving parser

Entry model:

```go
type Entry struct {
    Key       string
    Value     string
    Raw       string
    Quote     byte
    KeyStart  int
    KeyEnd    int
    Start     int
    End       int
    Line      int
    Index     int
    Encrypted bool
    Multiline bool
}
```

Rules:

- Ignore comments and blank lines.
- Support `KEY=value`, `KEY: value`, `export KEY=value`, quoted/unquoted/multiline.
- Preserve CRLF, BOM, export prefix, spacing, inline comments, and trailing newline.
- Duplicate keys edit a specific occurrence by line/index.
- Add/remove whole lines with surrounding blank-line handling.
- `encrypted:` entries are read-only.
- Multiline edits use “Open in Editor”.
- Dirty documents abort writes.

---

## 9. Testing

Go:

```text
internal/envfile/*_test.go
internal/metadata/*_test.go
internal/discovery/*_test.go
internal/layers/*_test.go
internal/runner/*_test.go
internal/swap/*_test.go
internal/postman/*_test.go
internal/gitignore/*_test.go
internal/cli/*_test.go
```

- Temporary directories only.
- Offline.
- CLI integration tests invoke the built test binary or `go run`.
- Encrypted entries, formatting, swap conflicts, and JSON contracts are covered.

Extension:

- `tsc --noEmit` must pass.
- Pure helpers may be tested with Node once they exist.
- Manual panel/debug/swap checklist per phase.

CI:

```sh
go vet ./cmd/... ./internal/...
gofmt -l cmd internal # must be empty
go test ./cmd/... ./internal/...
npm ci
npm run lint:ts
npm run lint:scripts
npm run check:no-dotenvx
npm run build
```

---

## 10. Build phases

### Phase 0 — Scaffold and language pivot (complete)

- Go module, CLI help/version, Go test.
- TypeScript extension shell with version command.
- Root npm scripts, esbuild build, VSIX package/install.
- No-dotenvx guard, gofmt guard, CI.
- Activity-bar icon asset and extension manifest.

Acceptance: `go test ./cmd/... ./internal/...`, `go vet ./cmd/... ./internal/...`, `gofmt -l cmd internal` empty, `npm install`, `npm run lint`, `npm run check:no-dotenvx`, and `npm run build` pass.

### Phase 1 — Go read path (in progress)

- `envfile` parser + tests. **Done.**
- `metadata`, `discovery`, `layers`, `project` snapshot.
- CLI `init`, `snapshot`, `list`, `status`, `use`, `vars`, `value`.
- Sidecar read/write, active fallback, reconciliation.

### Phase 2 — Go write/runtime path

- Variable edits and environment CRUD.
- Runner, exports, swap/restore, Postman import, gitignore.
- CLI `create`, `rename`, `rename-file`, `duplicate`, `remove`, `set`, `unset`, `rename-var`, `resolve`, `run`, `export`, `swap`, `restore`, `import`, `gitignore`, `doctor`.

### Phase 3 — Extension panel

- CLI client, store, tree, status bar, masking, watchers.
- Environment/variable commands.
- Filter, empty states, multi-root.

### Phase 4 — Extension runtime + docs

- Debug provider, run/task, swap UI, Postman UI, gitignore guard.
- README, CHANGELOG, DEVELOPMENT, manual checklist, VSIX install/reload.

---

## 11. Build, install, verify

```sh
cd /Users/andydrei/Documents/MyADHD/running-tools/env_handler_vs_extension

go test ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
gofmt -l cmd internal

npm install
npm run lint
npm run check:no-dotenvx
npm test
npm run build
npm run package:vsix
npm run install:vsix
```

Then run **Developer: Reload Window**.

Manual checklist:

- Environments icon/command appears.
- Snapshot has no values.
- Activating persists across reloads.
- Multi-root folders are independent.
- Masking and transient reveal work.
- Edits preserve formatting.
- Encrypted entries are locked.
- F5 injects base + active when enabled.
- Run uses the bundled Go binary.
- Swap backs up and restores.
- Postman import creates a file + metadata.
- Gitignore warning offers a fix.
- No dotenvx dependency exists.

---

## 12. Risks

| Risk | Mitigation |
| --- | --- |
| Go binary platform mismatch in VSIX | Build for the current platform in Phase 0; release matrix later. |
| Parser edge cases | Offset-preserving Go parser + extensive tests. |
| Runner cross-platform | `os/exec`, Windows shell handling, signal forwarding tests. |
| Swap damage | Opt-in, confirm, timestamped backup, hash validation, restore. |
| Layering vs swap | Swap explicitly bypasses and warns. |
| Debug provider coverage | Broad registration + `debugTypes` filter + no launch.json writes. |
| Encrypted values | Read-only detection; never decrypt; debug skips; runner warns. |
| Sidecar drift | Reconcile on read; watcher refresh; never overwrite corrupt file. |
| Dirty editor | Abort mutations and ask to save. |
| Virtual workspaces | Declared unsupported; workspace extension kind. |
| dotenvx reintroduction | CI guard scans Go, TS, manifests, and lockfile. |
| Extension/CLI drift | JSON DTOs in `internal/jsonapi`; contract tests. |

---

## 13. Decisions locked

| Decision | Choice |
| --- | --- |
| Core language | **Go** |
| Extension language | **TypeScript** |
| Project shape | Standalone monorepo: Go CLI/library + thin extension |
| Location | Current folder |
| dotenvx | Prohibited |
| dotenv-vscode fork | Not used |
| Metadata | `.envh.json`, no values |
| Runtime | Go runner |
| Debug | Opt-in provider, default off |
| Encrypted values | Read-only, no decryption |
| Layering | Base `.env` same directory + active; swap bypasses |
| Runner precedence | File values override parent env by default |
| Sidecar path | Project root |
| Version | `0.1.0` |
| Node in extension | Node 18+; no runtime npm dependencies |
| Go | 1.23+ |

---

## 14. Out of scope for v1

- Decryption.
- Custom editor/cloaking/autocomplete/syntax highlighting.
- Virtual workspace support.
- Cloud sync/telemetry/network calls.
- Postman collections (environments only).
- Drag-and-drop ordering.
- Multiple sidecars per workspace folder.
- Cross-platform VSIX matrix (current platform first).
