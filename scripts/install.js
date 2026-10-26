'use strict'

const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const { currentTarget } = require('./targets')

const root = path.resolve(__dirname, '..')
const version = require(path.join(root, 'package.json')).version
const target = currentTarget()
const candidates = [
  path.join(root, 'dist', `env-handler-vscode-${version}-${target}.vsix`),
  path.join(root, 'dist', `env-handler-vscode-${version}.vsix`)
]
const vsix = candidates.find(file => fs.existsSync(file))
const code = process.platform === 'win32' ? 'code.cmd' : 'code'

if (vsix === undefined) {
  console.error(`envh: no VSIX for ${target} was found in dist/.`)
  console.error('Run npm run package:vsix first.')
  process.exit(1)
}

const result = spawnSync(code, ['--install-extension', vsix, '--force'], {
  stdio: 'inherit',
  shell: false
})

if (result.error !== undefined) {
  console.error(`envh: could not run the VS Code CLI: ${result.error.message}`)
  console.error('Install the `code` command from the Command Palette and try again.')
  process.exit(1)
}

process.exit(result.status ?? 1)
