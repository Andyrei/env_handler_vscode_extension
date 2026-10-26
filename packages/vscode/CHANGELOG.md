# Changelog

## [0.1.1] - 2026-10-26

### Added

- `envh run` and `envh export --format shell` are now implemented.
- Multi-platform VSIX builds for macOS Intel/Apple Silicon, Linux x64/arm64, and Windows x64/arm64.
- Layered environment resolution now uses the local store correctly.
- Project-wide active environment support for run, swap, and debug.

### Fixed

- `Run With Environment` now uses the project-wide active environment and passes its branch.
- Debug injection now resolves the active environment from any branch.
- Swap and restore now target the branch that owns the active environment.
- Extension TypeScript deprecation setting corrected.

## [0.1.0] - 2026-10-26

### Added

- Environments view with branch-aware grouping and status bar selection.
- Environment management: create, rename, rename file, duplicate, color,
  reorder, delete to trash, and restore.
- Variable management with masked values, explicit reveal, copy, and encrypted
  read-only detection.
- Branch-aware local storage outside the project folder.
- Base `.env` layering and swap with backup/restore.
- Run With Environment and opt-in debug environment injection.
- Postman environment import.
- Gitignore guard for plaintext env files.
- Bundled Go `envh` binary for the extension's platform.
