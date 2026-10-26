'use strict'

const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const { TARGETS, currentTarget } = require('./targets')

const root = path.resolve(__dirname, '..')
const vscodeDir = path.join(root, 'packages', 'vscode')
const outDir = path.join(root, 'dist')
const version = require(path.join(root, 'package.json')).version
const buildScript = path.join(__dirname, 'build.js')
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

const args = process.argv.slice(2)
const all = args.includes('--all')
const requested = readTarget(args)
const targets = all ? TARGETS.map(item => item.target) : [requested ?? currentTarget()]

for (const target of targets) {
  const built = spawnSync(process.execPath, [buildScript, '--target', target], {
    cwd: root,
    stdio: 'inherit',
    shell: false
  })
  if (built.error !== undefined) {
    console.error(`envh: could not build ${target}: ${built.error.message}`)
    process.exit(1)
  }
  if (built.status !== 0) {
    process.exit(built.status ?? 1)
  }

  const outFile = path.join(outDir, `env-handler-vscode-${version}-${target}.vsix`)
  const result = spawnSync(
    vsce,
    ['package', '--no-dependencies', '--allow-missing-repository', '--target', target, '--out', outFile],
    { cwd: vscodeDir, stdio: 'inherit', shell: false }
  )
  if (result.error !== undefined) {
    console.error(`envh: could not run vsce for ${target}: ${result.error.message}`)
    process.exit(1)
  }
  if (result.status !== 0) {
    process.exit(result.status ?? 1)
  }
  console.log(`packaged ${path.relative(root, outFile)}`)
}

// Leave a runnable binary for the current machine after packaging every target.
if (all && targets[targets.length - 1] !== currentTarget()) {
  const rebuilt = spawnSync(process.execPath, [buildScript], {
    cwd: root,
    stdio: 'inherit',
    shell: false
  })
  if (rebuilt.status !== 0) {
    process.exit(rebuilt.status ?? 1)
  }
}

function readTarget (argv) {
  const at = argv.indexOf('--target')
  if (at !== -1 && argv[at + 1] !== undefined) {
    return argv[at + 1]
  }
  return undefined
}
