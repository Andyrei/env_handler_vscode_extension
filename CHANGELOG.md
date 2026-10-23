# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Changed

- Pivoted the security-sensitive core from JavaScript to Go.
- The VS Code extension is now a thin TypeScript CLI client.
- The VSIX bundles a platform-specific `envh` binary.

### Added

- Offset-preserving Go `.env` parser/editor with formatting tests.
- Go module and Phase 0 CLI help/version command.
- TypeScript extension shell with a version command.
- esbuild bundle pipeline for the extension.
- VSIX packaging and local install scripts.
- gofmt guard, no-dotenvx guard, and CI workflow.
- Activity-bar icon asset.
