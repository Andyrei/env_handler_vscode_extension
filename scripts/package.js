'use strict'

const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const vscodeDir = path.join(root, 'packages', 'vscode')
const outDir = path.join(root, 'dist')
const version = require(path.join(root, 'package.json')).version
const outFile = path.join(outDir, `env-handler-vscode-${version}.vsix`)
const vsce = path.join(
  root,
  'node_modules',
  '.bin',
  process.platform === 'win32' ? 'vsce.cmd' : 'vsce'
)

if (!fs.existsSync(vsce)) {
  console.error('envh: vsce is not installed. Run npm install first.')
  process.exit(1)
}

fs.mkdirSync(outDir, { recursive: true })

const result = spawnSync(
  vsce,
  ['package', '--no-dependencies', '--allow-missing-repository', '--out', outFile],
  { cwd: vscodeDir, stdio: 'inherit', shell: false }
)

if (result.error !== undefined) {
  console.error(`envh: could not run vsce: ${result.error.message}`)
  process.exit(1)
}

process.exit(result.status ?? 1)
