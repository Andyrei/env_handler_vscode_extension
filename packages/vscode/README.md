# Env Handler

Manage named `.env` environments from VS Code while keeping real `.env` files
as the source of truth.

Env Handler is a standalone extension. A bundled Go CLI (`envh`) does the
security-sensitive work; the TypeScript extension only renders UI and calls it.
The extension does not parse `.env` content, does not decrypt values, and does
not depend on dotenvx.

## Features

- **Environments view** in the Activity Bar. Environments for the current git
  branch are shown first; other branches are grouped and collapsed.
- **Create, rename, duplicate, reorder, color, and delete** environments.
  Deleted environments move to trash and can be restored.
- **Variables**: add, edit, delete, copy, and reveal values. Values are masked
  by default.
- **One active environment** per project, shown in the status bar alongside the
  current branch.
- **Branch-aware storage**: env files live outside the workspace, one folder per
  project and git branch, so switching branches does not overwrite them.
- **Layering**: a `.env` file next to the active environment loads first, then
  the active environment wins.
- **Swap**: temporarily replace the workspace `.env` with the active environment.
  A backup is kept and restore is explicit.
- **Run with environment**: start a terminal command with the active environment
  loaded.
- **Debug integration** (opt-in): inject the active environment into supported
  debug configurations.
- **Postman import**: import a Postman environment JSON file.
- **Gitignore guard**: warn when a plaintext env file is not ignored.
  `.gitignore` is only edited when you ask.
- **Encrypted values are read-only**: `encrypted:` lines are detected, shown
  with a lock, preserved, and never decrypted or rewritten.

## Safety

- Values live only in `.env` files. Metadata (`.envh.json`) stores names, paths,
  colors, order, and active/swap state, never values.
- No network calls and no telemetry.
- The extension is read-only in untrusted workspaces.
- Virtual workspaces are not supported.

## Storage

Env files and metadata live in the local Env Handler store, not in your project
folder. Use **Copy Project Env Files to Local Storage** to adopt existing project
env files safely, and **Delete Project Env Copies** once the stored copies are
confirmed.

## Requirements

- VS Code 1.85 or newer.
- A platform-specific `envh` binary is bundled in the VSIX. Install a build that
  matches your OS and CPU (for example, macOS Apple Silicon).

## Getting started

1. Open a folder or workspace.
2. Open the **Env Handler** view in the Activity Bar.
3. Create an environment, or import an existing Postman environment.
4. Add variables and choose **Set Active**.
5. Run or debug with the active environment:
   - **Env Handler: Run With Environment**
   - Enable `envHandler.applyToDebug` for debug sessions.

## Settings

| Setting | Default | Description |
| --- | --- | --- |
| `envHandler.applyToDebug` | `false` | Inject the active environment into debug sessions. |
| `envHandler.debugTypes` | Common Node, Python, Go, Rust, C++, PHP, Ruby, and .NET types | Debug types that receive environment injection. |
| `envHandler.runCommand` | empty | Default command for **Run With Environment**. |

## Known limitations

- The bundled `envh` binary targets one platform per VSIX. Use a build for your
  OS and CPU.
- Virtual workspaces are unsupported; untrusted workspaces are read-only.
- Encrypted `encrypted:` values are never decrypted by this extension.

## Development

Env Handler is part of the
[Env Handler repository](https://github.com/Andyrei/env_handler_vscode_extension).
Build, package, and install from the repository root:

```sh
npm install
npm run package:vsix
npm run install:vsix
```

## License

MIT.
