'use strict'

const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const esbuild = require('esbuild')

const root = path.resolve(__dirname, '..')
const vscodeDir = path.join(root, 'packages', 'vscode')
const binDir = path.join(vscodeDir, 'bin')
const distDir = path.join(vscodeDir, 'dist')
const version = require(path.join(root, 'package.json')).version
const binaryName = process.platform === 'win32' ? 'envh.exe' : 'envh'
const binaryPath = path.join(binDir, binaryName)

fs.mkdirSync(binDir, { recursive: true })
fs.mkdirSync(distDir, { recursive: true })

const goBuild = spawnSync(
  'go',
  [
    'build',
    '-trimpath',
    '-ldflags',
    `-s -w -X env-handler/internal/version.Version=${version}`,
    '-o',
    binaryPath,
    './cmd/envh'
  ],
  { cwd: root, stdio: 'inherit', shell: false }
)

if (goBuild.error !== undefined) {
  console.error(`envh: could not run go: ${goBuild.error.message}`)
  process.exit(1)
}
if (goBuild.status !== 0) {
  process.exit(goBuild.status ?? 1)
}

if (process.platform !== 'win32') {
  fs.chmodSync(binaryPath, 0o755)
}

esbuild.buildSync({
  entryPoints: [path.join(vscodeDir, 'src', 'extension.ts')],
  outfile: path.join(distDir, 'extension.js'),
  bundle: true,
  platform: 'node',
  format: 'cjs',
  target: 'node18',
  external: ['vscode'],
  sourcemap: false,
  logLevel: 'silent'
})

console.log(`built ${path.relative(root, binaryPath)} (${formatBytes(fs.statSync(binaryPath).size)})`)
console.log(`built ${path.relative(root, path.join(distDir, 'extension.js'))}`)

function formatBytes (bytes) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`
}
