# Development

## Requirements

- Go 1.23+ (development machine currently uses Go 1.27.3)
- Node.js 22
- npm 9+
- VS Code 1.85+ for manual extension testing

## Setup

```sh
npm install
```

## Common commands

```sh
npm test                  # go test ./cmd/... ./internal/...
npm run lint              # go vet + gofmt check + tsc + standard
npm run check:no-dotenvx  # hard dependency/source guard
npm run build             # build Go binary + bundle extension
npm run package:vsix      # build and produce dist/*.vsix
npm run install:vsix      # install the VSIX with the `code` CLI
npm run verify            # lint + guard + test + build
```

## Running the CLI

```sh
go run ./cmd/envh --help
go run ./cmd/envh --version
```

After a build:

```sh
packages/vscode/bin/envh --version
```

To make `envh` available globally from this checkout:

```sh
go install ./cmd/envh
```

## Extension development

1. Run `npm run build`.
2. Press `F5` in VS Code with this folder open.
3. The extension source is `packages/vscode/src/extension.ts`.
4. The packaged entry point is `packages/vscode/dist/extension.js`.

The extension expects the Go binary at `packages/vscode/bin/envh` (or
`envh.exe` on Windows). Rebuild after changing Go code.

## Packaging

```sh
npm run package:vsix
npm run install:vsix
```

Then run **Developer: Reload Window**. The local publisher name is
`env-handler`; no Marketplace account is required for a local VSIX.

The VSIX contains a platform-specific Go binary. Build it on the target
platform or add a release matrix later.

## Testing

Go tests use `testing` and temporary directories. Tests must stay offline and
must not touch the real workspace. `TestMain` in `internal/cli` sets
`ENVH_HOME` to a temp folder so tests never touch the user's real store.

Before finishing a change:

```sh
gofmt -l cmd internal     # must print nothing
go vet ./cmd/... ./internal/...
go test ./cmd/... ./internal/...
npm run lint
npm run check:no-dotenvx
npm run build
```

## Rules

- No dotenvx dependency, import, CLI call, or decryption.
- Do not fork or patch `dotenv-vscode`.
- Metadata contains no values.
- `encrypted:` entries are read-only.
- The extension must not parse `.env` content; call the Go CLI.
- Keep changes small and phase-scoped. See `env-manager-plan.md`.
- Env files are stored outside the project. Tests must not touch the real store.
- Commit without pushing unless asked.
