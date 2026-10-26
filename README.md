# Env Handler

Standalone environment manager for `.env` files.

The security-sensitive core is Go. The VS Code extension is a thin TypeScript
client that invokes the bundled Go binary. Real `.env` files remain the source
of truth.

**No dotenvx. No fork of `dotenv-vscode`.**

## Status

Core CLI and extension panel are functional. Debug integration, run-with-environment,
Postman import, branch-aware storage, and gitignore guard are implemented.

See [`env-manager-plan.md`](./env-manager-plan.md) for the full design and
[`AGENTS.md`](./AGENTS.md) for repository rules.

## Why Go

- Single static binary, easy to bundle in a VSIX.
- No Python runtime dependency for users.
- Strong standard library and memory safety for parsing and process control.
- The extension remains a thin adapter; it never parses `.env` content.

## Layout

```text
cmd/envh          Go CLI entry point
internal/         Go core: parser, metadata, discovery, layers, runner, swap
packages/vscode   Thin TypeScript extension
scripts           Build, package, install, and guard scripts
```

## Requirements

- Go 1.23 or newer
- Node.js 18 or newer
- npm 9 or newer
- VS Code 1.85 or newer for the extension

VS Code extension builds are available for macOS (Intel and Apple Silicon), Linux (x64 and arm64), and Windows (x64 and arm64).

## Development

```sh
go test ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
gofmt -l cmd internal

npm install
npm run lint
npm run check:no-dotenvx
npm test
npm run build
```

## CLI usage

```sh
envh list                          # show environments and variable keys
envh use <env>                     # set the active environment
envh create <name> [--activate]    # create .env.<name>
envh set <env> <key> -- <value>   # set a value
envh unset <env> <key>             # remove a key
envh value <env> <key> --reveal    # print a value
envh resolve --env <env> --json    # merged env for debug/run
envh run -- <command>              # run with active env loaded
envh export --format shell         # print shell exports

# Storage (env files live outside the project, one folder per branch)
envh where                         # show where env files are stored
envh branches                      # list branches with env files
envh overview                      # list every branch and its environments
envh adopt                         # copy project env files to local storage
envh purge --dry-run | --yes       # delete project copies (identical only)
envh gitignore [--all] [--dry-run] # add env files to .gitignore

# Management
envh rename <env> <name>           # change display name
envh rename-file <env> <file>      # rename the file on disk
envh duplicate <env> [--name <n>]  # copy an environment
envh remove <env> --yes            # move to trash
envh restore <file>                # restore from trash
envh trash list                    # list trashed files
envh trash empty --yes             # empty trash
envh color <env> <color>           # set color
envh move <env> up|down            # reorder
envh import postman <file>         # import a Postman environment
```

## Package and install the extension

```sh
npm run package:vsix
npm run install:vsix
```

Then run **Developer: Reload Window** in VS Code.

The VSIX is written to `dist/env-handler-vscode-0.1.0.vsix`. It contains the Go
binary for the current platform (macOS Apple Silicon).

## Design highlights

- Metadata lives in `.envh.json` and never contains values.
- Base `.env` in the same directory as the active file is loaded first.
- Swap bypasses base layering and keeps a timestamped backup.
- `encrypted:` values are detected, shown read-only, and never decrypted.
- Debug integration is opt-in and injects `env` without editing `launch.json`.
- The extension bundles its own `envh` runtime; no global CLI install is required.
- Env files are stored outside the project, one folder per git branch.

## License

MIT. See [`LICENSE`](./LICENSE).
