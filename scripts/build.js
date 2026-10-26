'use strict'

const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const esbuild = require('esbuild')
const { binaryName, currentTarget, targetInfo } = require('./targets')

const root = path.resolve(__dirname, '..')
const vscodeDir = path.join(root, 'packages', 'vscode')
const binDir = path.join(vscodeDir, 'bin')
const distDir = path.join(vscodeDir, 'dist')
const version = require(path.join(root, 'package.json')).version

const target = readTarget()
const info = targetInfo(target)
const binaryPath = path.join(binDir, binaryName(target))

// Keep exactly one binary in bin/ so a VSIX can only contain this target.
fs.rmSync(binDir, { recursive: true, force: true })
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
  {
    cwd: root,
    stdio: 'inherit',
    shell: false,
    env: {
      ...process.env,
      GOOS: info.goos,
      GOARCH: info.goarch,
      CGO_ENABLED: '0'
    }
  }
)

if (goBuild.error !== undefined) {
  console.error(`envh: could not run go: ${goBuild.error.message}`)
  process.exit(1)
}
if (goBuild.status !== 0) {
  process.exit(goBuild.status ?? 1)
}

if (info.goos !== 'windows') {
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

console.log(`built ${path.relative(root, binaryPath)} for ${target} (${formatBytes(fs.statSync(binaryPath).size)})`)
console.log(`built ${path.relative(root, path.join(distDir, 'extension.js'))}`)

function readTarget () {
  const at = process.argv.indexOf('--target')
  if (at !== -1 && process.argv[at + 1] !== undefined) {
    return process.argv[at + 1]
  }
  return process.env.ENVH_TARGET ?? currentTarget()
}

function formatBytes (bytes) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`
}
