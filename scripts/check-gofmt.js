'use strict'

const { spawnSync } = require('node:child_process')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const result = spawnSync('gofmt', ['-l', 'cmd', 'internal'], {
  cwd: root,
  encoding: 'utf8',
  shell: false
})

if (result.error !== undefined) {
  console.error(`envh: could not run gofmt: ${result.error.message}`)
  process.exit(1)
}
if (result.status !== 0) {
  process.exit(result.status ?? 1)
}

const files = result.stdout.trim()
if (files.length > 0) {
  console.error('gofmt required for:')
  console.error(files)
  process.exit(1)
}
