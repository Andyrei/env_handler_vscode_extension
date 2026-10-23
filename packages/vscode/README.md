# Env Handler for VS Code

Thin VS Code client for the standalone Env Handler CLI.

The extension manages named `.env` environments through a Go binary bundled
into the VSIX. It does not parse `.env` files in JavaScript, does not decrypt
values, and does not depend on dotenvx.

Phase 0 exposes the version command and build pipeline. The Environments panel
lands in a later phase.

## Development

From the repository root:

```sh
npm install
npm run build
npm run package:vsix
npm run install:vsix
```

Then run **Developer: Reload Window**.

## License

MIT.
