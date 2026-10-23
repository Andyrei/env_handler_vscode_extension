# Env Handler

Standalone environment manager for `.env` files.

The security-sensitive core is Go. The VS Code extension is a thin TypeScript
client that invokes the bundled Go binary. Real `.env` files remain the source
of truth.

**No dotenvx. No fork of `dotenv-vscode`.**

## Status

Phase 0 complete: Go CLI scaffold, TypeScript extension shell, build pipeline,
no-dotenvx guard, and VSIX packaging. Environment commands and the panel arrive
in the following phases.

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

Run the CLI from the checkout:

```sh
go run ./cmd/envh --help
```

After a build:

```sh
packages/vscode/bin/envh --version
```

## Package and install the extension

```sh
npm run package:vsix
npm run install:vsix
```

Then run **Developer: Reload Window** in VS Code.

The VSIX is written to `dist/env-handler-vscode-0.1.0.vsix`. It contains the Go
binary for the current platform.

## Design highlights

- Metadata lives in `.envh.json` and never contains values.
- Base `.env` in the same directory as the active file is loaded first.
- Swap bypasses base layering and keeps a timestamped backup.
- `encrypted:` values are detected, shown read-only, and never decrypted.
- Debug integration is opt-in and injects `env` without editing `launch.json`.
- The extension bundles its own `envh` runtime; no global CLI install is
  required for run/task support.

## License

MIT. See [`LICENSE`](./LICENSE).
