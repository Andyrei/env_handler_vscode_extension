# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Changed

- Pivoted the security-sensitive core from JavaScript to Go.
- The VS Code extension is now a thin TypeScript CLI client.
- The VSIX bundles a platform-specific `envh` binary.
- Env files are stored outside the project, one folder per git branch.
- Deleted environments move to trash instead of being removed permanently.
- `envh run` and `envh export --format shell` now implement the documented runner and shell export behavior.
- Multi-platform VSIX builds now cover macOS Intel/Apple Silicon, Linux x64/arm64, and Windows x64/arm64.
- The project-wide active environment is used for run, swap, and debug, even when it belongs to another branch.

### Added

- Offset-preserving Go `.env` parser/editor with formatting tests.
- Go module and Phase 0 CLI help/version command.
- TypeScript extension shell with a version command.
- esbuild bundle pipeline for the extension.
- VSIX packaging and local install scripts.
- gofmt guard, no-dotenvx guard, and CI workflow.
- Activity-bar icon asset.
- Environment CRUD, activation, and management in CLI and panel.
- Branch-aware storage with `where`, `branches`, `overview` commands.
- Adopt and purge for moving project env files to local storage.
- Postman environment import.
- Gitignore guard with fallback matcher.
- Debug integration and run-with-environment.
- Trash and restore for deleted environments.
- Panel inline editing: rename, color, variables.
- `internal/runner` and `internal/shell` packages with tests.
- `envh run` and `envh export --format shell`.
- Project-wide layered value resolution.
- Multi-platform build and packaging scripts.
