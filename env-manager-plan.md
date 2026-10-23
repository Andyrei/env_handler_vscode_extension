# Env Handler — Standalone `.env` Environment Manager

**Status:** Planned, not started
**Created:** 2026-10-23
**Updated:** 2026-10-23
**Repo:** `/Users/andydrei/Documents/MyADHD/running-tools/env_handler_vs_extension`
**Supersedes:** the earlier plan to patch the `dotenv-vscode` checkout

---

## 1. Constraints

These are hard requirements:

- **Standalone project.** Build here, in this folder. Do not patch or fork `dotenv-vscode`.
- **No dotenvx.** No `@dotenvx/dotenvx` dependency, no CLI calls, no decryption through it.
- **Core CLI + thin VS Code extension.** The core is a standalone Node library/CLI. The extension is a UI and debug/task adapter over it.
- **Real `.env` files are the source of truth.** No duplicated secret store.
- **Metadata only in the sidecar.** `.envh.json` contains names, paths, active state, colors, order, and swap state — never values.
- **Encrypted values are read-only.** Detect `encrypted:`, show a lock, preserve the line, never rewrite or decrypt.
- **Native runtime.** Our own Node runner loads and layers `.env` files and spawns commands.
- **Offline.** No network calls, no telemetry.

---

## 2. What the project is

A Postman-style environment manager for `.env` files:

- Named environments backed by real files: `.env`, `.env.local`, `.env.staging`, `.env.production`.
- One active environment per project root.
- A CLI (`envh`) to list, activate, edit, run, export, swap, and import.
- A thin VS Code extension with an Environments activity-bar panel, status-bar chip, debug integration, and run/swap commands.
- Base layering: runtime loads `.env` first, then the active environment (for example `.env.local`).
- Swap mode for single-file apps: `.env` becomes a symlink/copy of the active file, with a backup and restore command.

The official Dotenv extension can stay installed. It continues to provide cloaking, autocomplete, and its custom editor. This project does not contribute a language or editor.

---

## 3. Architecture

```text
env_handler_vs_extension/
├── package.json                    # npm workspaces root
├── README.md
├── CHANGELOG.md
├── DEVELOPMENT.md
├── LICENSE
├── .gitignore
├── .nvmrc
├── .github/workflows/ci.yml
├── scripts/
│   ├── build.js                    # esbuild: extension + bundled CLI
│   ├── package.js                  # vsce package
│   ├── install.js                  # code --install-extension
│   └── check-no-dotenvx.js         # hard dependency/source guard
├── packages/
│   ├── core/                       # @env-handler/core
│   │   ├── package.json
│   │   ├── bin/envh.js
│   │   ├── lib/
│   │   │   ├── index.js
│   │   │   ├── discovery.js
│   │   │   ├── env-vars.js
│   │   │   ├── metadata.js
│   │   │   ├── sidecar.js
│   │   │   ├── layers.js
│   │   │   ├── runner.js
│   │   │   ├── swap.js
│   │   │   ├── postman.js
│   │   │   ├── gitignore.js
│   │   │   ├── shell.js
│   │   │   ├── fsx.js
│   │   │   ├── paths.js
│   │   │   └── errors.js
│   │   └── test/
│   │       ├── env-vars.test.js
│   │       ├── metadata.test.js
│   │       ├── discovery.test.js
│   │       ├── layers.test.js
│   │       ├── runner.test.js
│   │       ├── swap.test.js
│   │       ├── postman.test.js
│   │       ├── gitignore.test.js
│   │       └── cli.test.js
│   └── vscode/                     # env-handler-vscode
│       ├── package.json
│       ├── extension.js
│       ├── lib/
│       │   ├── store.js
│       │   ├── tree-data.js         # pure node builders
│       │   ├── tree-provider.js
│       │   ├── status-bar.js
│       │   ├── commands.js
│       │   ├── debug-provider.js
│       │   ├── task-provider.js
│       │   ├── mask.js
│       │   └── config.js
│       ├── media/icon.svg
│       ├── dist/                    # generated, not committed
│       ├── .vscodeignore
│       └── README.md
```

Language/runtime choices:

- Plain JavaScript (CommonJS), Node 22 for development, `standard` for lint.
- `node:test` for all offline tests.
- `esbuild` bundles the extension and a bundled `envh` runtime into `packages/vscode/dist`.
- Core runtime dependencies: `ignore` and `picomatch` only.
- No `dotenv`, no `dotenvx`. The parser is implemented locally so edits preserve exact offsets.

---

## 4. Data model

### 4.1 Source of truth

```text
.env
.env.local
.env.staging
.env.production
```

Each file is an environment. Values live only in these files.

### 4.2 Sidecar — `<project-root>/.envh.json`

```jsonc
{
  "version": 1,
  "active": ".env.staging",
  "environments": [
    { "file": ".env", "name": "Default", "color": "gray", "order": 0 },
    { "file": ".env.local", "name": "Local", "color": "green", "order": 1 },
    { "file": ".env.staging", "name": "Staging", "color": "yellow", "order": 2 },
    { "file": ".env.production", "name": "Production", "color": "red", "order": 3 }
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

- `swap` is optional and exists only while swap is active.
- No `hidden` field in v1.
- No values, no ciphertext, no tokens.
- Sidecar is safe to commit.
- Created lazily on the first metadata change.
- Files discovered on disk but absent from the sidecar appear with defaults.
- Entries whose files disappear are dropped during reconciliation.
- Missing `active` falls back to `.env`, then the first environment, with one warning.
- Corrupt sidecar: report the path, do not overwrite, let the user fix it.
- Paths are project-relative POSIX paths so nested and monorepo envs work.
- Swap state is validated on load; a stale target/backup produces a warning, not a write.

### 4.3 Discovery

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

Notes:

- `.envrc` is not matched by `**/.env.*` and is therefore excluded naturally.
- Templates and examples are excluded by default; users can override `include`/`exclude`.
- Symlinked files are included; symlinked directories are skipped to avoid loops.
- `node_modules` and `.git` are skipped during traversal.
- Discovery is project-root scoped. CLI uses `cwd` (or `--cwd`); the extension uses the workspace folder.

### 4.4 Layering

For an active file:

1. If the active file is `.env`, load just it.
2. Otherwise, look for `.env` in the **same directory** as the active file.
3. Load base `.env` first, then the active file.
4. Later values win.
5. If swap state is active, load only the swap target; swap bypasses layering.
6. If no base exists, load only the active file.

Examples:

- `.env.local` at the root → `.env`, `.env.local`.
- `packages/api/.env.staging` → `packages/api/.env`, `packages/api/.env.staging`.
- `.env.staging` with swap active → `.env.staging` only.

Precedence:

- **Runner:** file values override the parent process environment by default. `--no-override` makes the parent win.
- **Debug:** merged file values are applied, then `config.env` is applied last, so explicit launch configuration wins.
- **Exports:** base + active merged; later wins.

---

## 5. Core library

### 5.1 Modules

| Module | Responsibility |
| --- | --- |
| `env-vars.js` | Pure parser and offset-preserving editor. No filesystem, no VS Code. |
| `metadata.js` | Pure normalize/merge/reconcile/name/color/order logic. |
| `sidecar.js` | Read/write `.envh.json`, atomic write, corruption handling. |
| `discovery.js` | Recursive discovery with include/exclude globs, symlink safety. |
| `layers.js` | Resolve base + active, merge values, detect encrypted entries. |
| `runner.js` | Spawn a command with a merged environment; forward signals and exit code. |
| `swap.js` | Symlink/copy swap, backup, hash validation, restore. |
| `postman.js` | Parse Postman environment exports (single object or array). |
| `gitignore.js` | `git check-ignore` when available; pure fallback parser. |
| `shell.js` | Shell exports, env-file output, quoting, envh command generation. |
| `fsx.js` | Atomic text write, backup naming, safe rename/delete. |
| `paths.js` | POSIX/relative path normalization and containment checks. |
| `errors.js` | Typed errors with codes and safe messages (no values). |

### 5.2 Public API (indicative)

```js
const envh = require('@env-handler/core')

await envh.init(root)
const snapshot = await envh.snapshot(root)
await envh.use(root, 'staging')
await envh.create(root, 'Local', { activate: true })
await envh.setValue(root, '.env.local', 'API_URL', 'http://localhost:3000')
await envh.run(root, { env: '.env.local', command: 'node', args: ['app.js'] })
await envh.exportEnv(root, { env: '.env.local', format: 'shell' })
await envh.swap(root, { env: '.env.local' })
await envh.restore(root)
await envh.importPostman(root, json, { activate: true })
```

The API accepts absolute project roots and uses Node `fs` internally. The extension runs in the workspace extension host, so local and remote workspaces are supported; virtual filesystems are declared unsupported.

---

## 6. Core CLI — `envh`

Global flags: `--cwd <dir>`, `--json`, `--quiet`, `--help`, `--version`.

```sh
envh init
envh list [--json]
envh status
envh use <name|file>
envh create <name> [--file .env.foo] [--activate] [--template KEY=,OTHER=]
envh rename <env> <name>
envh rename-file <env> <file>
envh duplicate <env> [--name <name>]
envh remove <env> [--delete-file --yes]

envh vars [env] [--reveal] [--json]
envh set <env> <key> <value>
envh unset <env> <key>
envh rename-var <env> <old> <new>

envh run [--env <name|file>] [--no-base] [--no-override] [--quiet] -- <command ...>
envh export [--env <env>] [--no-base] [--format shell|json|env]

envh swap [--env <env>] [--copy] [--force]
envh restore [--force]

envh import postman <file> [--name <name>] [--force] [--activate] [--skip-disabled]
envh gitignore [--all] [--dry-run]
envh doctor [--json]
```

Behavior:

- `list` prints active marker, display name, relative file, variable count, encrypted count. Never values.
- `use` accepts display name, filename, or relative path; ambiguous names error with choices.
- `create` writes an empty file by default; `--template` seeds lines. It warns if the file is not gitignored.
- `remove` removes metadata only by default; `--delete-file --yes` moves the file to trash.
- `vars` masks values by default; `--reveal` prints them explicitly.
- `set`/`unset`/`rename-var` preserve comments, blank lines, quoting, `export` prefixes, CRLF, and trailing newlines.
- Encrypted entries are read-only: edits fail with a clear error.
- `run` loads layers, warns about encrypted values, spawns the command with inherited stdio, forwards signals, and exits with the child’s code. `--no-override` keeps parent environment values.
- `export` writes shell exports, JSON, or env-file output. It prints values by definition and warns on stderr.
- `swap` backs up `.env`, creates a relative symlink to the active file, or copies when `--copy`/Windows fallback applies. It warns that layering is bypassed.
- `restore` removes the generated `.env` after hash validation and restores the backup.
- `import postman` supports one environment or an array, comments out disabled values by default, sanitizes keys, warns about plaintext secrets, and creates a sidecar entry.
- `gitignore` appends specific files or all env files with `--all`; `--dry-run` prints the lines.
- `doctor` checks sidecar validity, active file existence, layers, gitignore coverage, encrypted counts, and swap state. It never prints values.

Exit codes:

- `0` success
- `1` usage error
- `2` environment or file not found
- `3` metadata invalid
- `4` refused unsafe operation (encrypted edit, swap conflict, existing backup)
- `5` command failed

---

## 7. VS Code extension

### 7.1 Scope

The extension is a thin client. It adds:

- Activity-bar container **Environments** (`envHandler`), view `envHandler.tree`.
- Folder → environment → variable tree.
- Active environment marker and click-to-activate.
- Masked values with transient reveal.
- Encrypted values shown as 🔒 and read-only.
- Create/rename/duplicate/delete environments; rename files; set color; reorder.
- Add/edit/rename/delete variables with format preservation.
- Filter, refresh, empty states.
- Status-bar chip scoped to the active editor’s workspace folder.
- Debug env injection.
- Run-with-environment and task provider.
- Swap/restore.
- Postman import.
- Gitignore guard.

It does **not** contribute a language, custom editor, cloaking, autocomplete, or syntax highlighting. The official Dotenv extension can remain installed.

### 7.2 Tree

```text
ENVIRONMENTS                                  +  ⚲  ⟳  ⋯
▾ api-service
   ● Local              ACTIVE      14 vars
   ○ Staging                         14 vars
   ○ Production                       9 vars  🔒 2
     DATABASE_URL       ████████████   ⤢  ✎  ⧉
     PORT               3000           ⤢  ✎  ⧉
▾ web-app
   ○ Development                      6 vars
```

Details:

- Single workspace folder → flat environment list.
- Multi-root workspace → folder nodes, independent active environment per folder.
- Environment item: colored circle icon, display name, `ACTIVE`, variable count, encrypted count, `SWAPPED` when applicable.
- Clicking an environment activates it. The twisty expands variables.
- Variable item: key, masked value, lock for encrypted entries.
- Mask character `envHandler.maskIcon`, capped at `envHandler.maskLength` characters.
- Reveal is transient and clears on refresh/activation change.
- Empty states: no folder open, no environments found, filter returned nothing.

### 7.3 Commands

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

Menus:

- `view/title`: create, filter, refresh, more.
- Environment inline: set active, add variable, reveal file.
- Environment context: rename, rename file, duplicate, delete, color, move up/down, copy exports, copy run command, open metadata.
- Variable inline: edit, reveal, copy.
- Variable context: rename, delete.
- Folder context: create environment, reveal folder, open metadata.
- Context-only commands hidden from the command palette unless also useful globally.

### 7.4 Settings

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `envHandler.applyToDebug` | boolean | `false` | Inject active environment into debug sessions |
| `envHandler.terminalMode` | enum | `none` | `none`, `task`, `swap` |
| `envHandler.maskValues` | boolean | `true` | Mask values in the panel |
| `envHandler.maskIcon` | string | `█` | Mask character |
| `envHandler.maskLength` | number | `32` | Maximum mask length |
| `envHandler.defaultTemplate` | string[] | `[]` | Lines seeded into new environments |
| `envHandler.include` | string[] | `["**/.env","**/.env.*"]` | Discovery include |
| `envHandler.exclude` | string[] | see discovery | Discovery exclude |
| `envHandler.debugTypes` | string[] | common runtime types | Debug types eligible for env injection |
| `envHandler.runCommand` | string | `""` | Default command for run/task |
| `envHandler.layerBase` | boolean | `true` | Load base `.env` before the active file |
| `envHandler.showStatusBar` | boolean | `true` | Show the active-environment chip |
| `envHandler.confirmSwap` | boolean | `true` | Confirm before replacing `.env` |

### 7.5 Debug integration

- Registers a `DebugConfigurationProvider` for a broad set of runtime debug types; the setting filters which ones are modified.
- Loads base + active, merges values, and injects `config.env`.
- Applies `config.env` last, so explicit launch configuration wins.
- Removes a managed `envFile` reference to avoid stale loading; leaves custom `envFile` paths alone.
- Skips encrypted values and warns once.
- Never edits `launch.json`.
- Disabled in untrusted workspaces.
- Default is off; the user enables it explicitly.

### 7.6 Run/task integration

- `envHandler.runWithEnvironment` picks an environment, prompts for a command if `runCommand` is empty, and executes the bundled `envh run --env <file> -- <command>` in a VS Code task/terminal.
- `terminalMode: task` registers a task provider of type `envh` and exposes tasks for environments with a configured command.
- `terminalMode: none` shows an informational message instead of running.
- `terminalMode: swap` activates swap behavior on active-environment changes.
- The bundled runtime is `dist/envh.js`, so no global CLI install is required for the extension.

### 7.7 Swap

- Opt-in via `terminalMode: swap` plus confirmation.
- Backs up an existing `.env` to `.env.backup.<timestamp>`.
- Creates a relative symlink `.env` → active file; on Windows/EPERM, copies the active file instead and stores a content hash.
- Stores swap state in `.envh.json`.
- Hides the generated `.env` from the tree while swap is active and marks the target as `SWAPPED`.
- Warns that base layering is bypassed and that a tracked `.env` will show a git change.
- `envHandler.restoreSwap` removes the generated file after hash validation and restores the backup.
- Switching away from swap mode prompts to restore.
- The extension does not auto-restore on deactivate.

### 7.8 Postman import

- Opens a JSON file; accepts a single environment or an array.
- Maps `name`, `values`, `enabled`, and `type`.
- Creates `.env.<slug>` and a metadata entry.
- Disabled values are written as comments by default; `--skip-disabled` skips them in the CLI.
- Invalid keys are reported; values are not printed.
- Warns that Postman secrets become plaintext at rest and runs the gitignore guard.
- Optionally activates the imported environment.

### 7.9 Gitignore guard

- Checks layers with `git check-ignore` when inside a trusted git repo.
- Falls back to a pure `.gitignore` matcher (including negation and nested files) when git is unavailable or the workspace is untrusted.
- Warns on create/activate/import when a plaintext layer is not ignored.
- Offers “Add this file”, “Add all env files”, and “Not now”.
- Never appends automatically.
- Remembers dismissal per file per session.

---

## 8. Format-preserving variable editor

Parser output per entry:

```js
{
  key,
  value,        // decoded
  raw,          // exact source value
  quote,        // '"' | "'" | '`' | null
  start,        // value start offset
  end,          // value end offset
  keyStart,
  keyEnd,
  line,
  index,
  encrypted
}
```

Rules:

- Ignore comments (`# ...`) and blank lines.
- Support `KEY=value`, `KEY: value`, `export KEY=value`, quoted, unquoted, and multiline quoted values.
- Support CRLF and preserve it.
- Preserve BOM.
- Support duplicate keys; edit targets a specific occurrence and shows its line number.
- Preserve inline comments when changing a value.
- Preserve `export` and spacing when renaming a key.
- Add/remove whole lines; keep surrounding comments and blank lines.
- Remove the correct newline at EOF.
- Detect `encrypted:` and mark the entry read-only.
- Route multiline value edits to “Open in Editor” instead of an input box.
- Abort writes when the document is dirty; ask the user to save first.
- Apply edits through `WorkspaceEdit` when the file is open, otherwise atomic file write.

---

## 9. Testing

All core tests are offline and do not download VS Code.

```text
packages/core/test/
  env-vars.test.js      # quoting, comments, CRLF, BOM, export, colon, duplicates
  metadata.test.js      # defaults, merge, reconcile, active fallback, swap state
  discovery.test.js     # include/exclude, nested dirs, symlinks, skipped dirs
  layers.test.js        # base resolution, precedence, swap bypass
  runner.test.js        # env merge, override rules, exit code, signals
  swap.test.js          # symlink, copy fallback, backup, restore, conflicts
  postman.test.js       # single/array, disabled values, secrets, invalid keys
  gitignore.test.js     # roots, negations, nested files, non-git fallback
  cli.test.js           # init/list/use/run/export/import in temp projects
```

Extension-side pure helpers (no `vscode` import):

```text
packages/vscode/test/unit/
  tree-data.test.js
  debug-config.test.js
  mask.test.js
```

CI runs:

```sh
npm run lint
npm run check:no-dotenvx
npm test
npm run build
```

No network access is required except `npm ci`.

---

## 10. Build phases

### Phase 0 — Scaffold (~1 session)

- npm workspaces, core and extension manifests.
- LICENSE, .gitignore, .nvmrc, README skeleton, CHANGELOG, DEVELOPMENT.
- Activity-bar icon.
- `scripts/build.js`, `scripts/package.js`, `scripts/install.js`.
- `scripts/check-no-dotenvx.js` and CI workflow.
- Minimal extension activation and CLI help.

Acceptance: `npm install`, `npx standard`, `npm test`, `npm run check:no-dotenvx`, and `npm run build` pass.

### Phase 1 — Core read path

- `env-vars.js` parser and tests.
- `metadata.js`, `sidecar.js`, `discovery.js`, `layers.js` and tests.
- CLI `init`, `list`, `status`, `use`, `vars`.
- `.envh.json` read/write, active fallback, reconciliation.

Acceptance: activate persists across CLI runs; multi-directory envs resolve correctly; metadata contains no values.

### Phase 2 — Core write/runtime path

- Variable edits, environment CRUD, rename/duplicate/delete.
- `runner.js`, `shell.js`, `swap.js`, `postman.js`, `gitignore.js`.
- CLI `create`, `rename`, `rename-file`, `duplicate`, `remove`, `set`, `unset`, `rename-var`, `run`, `export`, `swap`, `restore`, `import`, `gitignore`, `doctor`.
- Tests for each.

Acceptance: `envh run` loads base + active; swap backs up and restores; Postman import produces a working env and metadata; all formatting tests pass.

### Phase 3 — Extension panel

- `store.js`, tree data/provider, status bar, masking, watchers.
- Create/rename/duplicate/delete env and variable commands.
- Filter, empty states, metadata open.
- Multi-root grouping and active folder selection.

Acceptance: create/activate/edit in the panel; reload preserves active; a second workspace folder is independent; external file changes appear live; values are masked by default.

### Phase 4 — Extension runtime + docs

- Debug provider.
- Run/task provider.
- Swap UI and restore.
- Postman import UI.
- Gitignore guard UI.
- README, CHANGELOG, DEVELOPMENT.
- VSIX build, install, reload, manual checklist.

Acceptance: F5 uses base + active; explicit `config.env` wins; swap/restore works; import works; packaged VSIX installs over nothing and reloads cleanly.

---

## 11. Build, install, verify

```sh
cd /Users/andydrei/Documents/MyADHD/running-tools/env_handler_vs_extension

npm install
npx standard
npm test
npm run check:no-dotenvx
npm run build
npm run package:vsix
code --install-extension dist/env-handler-vscode-0.1.0.vsix --force
```

Then **Developer: Reload Window** and open a temp project with:

```text
.env
.env.local
.env.staging
```

Manual checklist:

- The Environments icon appears.
- Environments are discovered and metadata is created lazily.
- Activating Local persists after reload.
- A second workspace folder has its own active environment.
- Values are masked; reveal is transient.
- Add/edit/rename/delete preserves comments, blank lines, quoting, CRLF.
- Encrypted entries are locked and read-only.
- F5 injects base + active when enabled.
- Run with Environment executes the native runner.
- Swap backs up, symlinks/copies, and restores.
- Postman import creates a file and metadata.
- Gitignore warning offers a one-click fix.
- No dotenvx dependency exists.

---

## 12. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Parser edge cases (multiline, duplicates, CRLF) | Offset-preserving parser plus dedicated unit tests. |
| Runner cross-platform behavior | Spawn via Node; Windows shell handling; `--copy` swap tests. |
| Swap damaging a tracked `.env` | Opt-in, confirmation, timestamped backup, hash validation, restore command. |
| Base layering vs swap | Swap explicitly bypasses layering and warns. |
| Debug providers per type | Static broad registration plus `debugTypes` filter; no launch.json writes. |
| Encrypted values | Read-only detection; no decrypt; runner warns; debug skips. |
| Sidecar drift | Reconcile on read; watcher refresh; corrupt file is never overwritten. |
| Dirty editor vs panel edits | Display dirty text; abort writes and ask to save. |
| Gitignore coverage | git CLI when trusted; pure fallback; explicit user action only. |
| Virtual workspaces | Declared unsupported; `extensionKind: ["workspace"]`; clear message. |
| Coexistence with official Dotenv extension | No language/editor contributions; panel-only extension. |
| Accidental dotenvx reintroduction | `check:no-dotenvx` scans manifests, lockfile, and source imports in CI. |

---

## 13. Decisions locked

| Decision | Choice |
| --- | --- |
| Project shape | Standalone monorepo: core CLI/library + thin VS Code extension |
| Location | `/Users/andydrei/Documents/MyADHD/running-tools/env_handler_vs_extension` |
| dotenvx | Prohibited |
| dotenv-vscode fork | Not used |
| Metadata | `.envh.json` in project root, no values |
| Active runtime | Native Node runner; no external binary |
| Debug | Opt-in `DebugConfigurationProvider`, default off |
| Encrypted values | Detected, read-only, never decrypted |
| Layering | Base `.env` in same directory + active; swap bypasses |
| Runner precedence | File values override parent env by default; `--no-override` |
| Swap | Symlink, Windows copy fallback, backup/restore, opt-in |
| Discovery | `.env` + `.env.*`; exclude examples/templates/keys/vault |
| New env template | Empty by default |
| Version | `0.1.0` |
| Language | Plain JavaScript (CommonJS) |

---

## 14. Out of scope for v1

- Decryption of `encrypted:` values.
- A custom `.env` editor, cloaking, autocomplete, or syntax highlighting.
- Virtual workspace support.
- Cloud sync, telemetry, or network calls.
- Importing Postman collections (only environments).
- Drag-and-drop reordering (move up/down commands instead).
- Multiple sidecars per workspace folder.
