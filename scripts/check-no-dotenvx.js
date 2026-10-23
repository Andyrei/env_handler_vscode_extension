'use strict'

const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const forbiddenDependencies = new Set(['dotenv', 'dotenvx', 'dotenv-vscode'])
const forbiddenPattern = /\bdotenvx\b|dotenv-vscode|node_modules\/dotenv\b|@dotenvx\//
const failures = []

checkManifest(path.join(root, 'package.json'))
checkManifest(path.join(root, 'packages', 'vscode', 'package.json'))
checkPackageLock(path.join(root, 'package-lock.json'))
checkTextFile(path.join(root, 'go.mod'))
checkGoSum(path.join(root, 'go.sum'))

for (const directory of ['cmd', 'internal', 'scripts', path.join('packages', 'vscode', 'src')]) {
  walk(path.join(root, directory))
}

if (failures.length > 0) {
  console.error('Forbidden dotenvx/dotenv references found:')
  for (const failure of failures) {
    console.error(`  ${failure}`)
  }
  process.exit(1)
}

console.log('No dotenvx or dotenv parser dependency found.')

function checkManifest (file) {
  if (!fs.existsSync(file)) return

  let manifest
  try {
    manifest = JSON.parse(fs.readFileSync(file, 'utf8'))
  } catch (error) {
    failures.push(`${relative(file)}: invalid JSON (${error.message})`)
    return
  }

  const sections = [
    'dependencies',
    'devDependencies',
    'peerDependencies',
    'optionalDependencies',
    'bundledDependencies',
    'bundleDependencies'
  ]

  for (const section of sections) {
    const entries = manifest[section]
    if (entries === undefined || entries === null) continue
    const names = Array.isArray(entries) ? entries : Object.keys(entries)
    for (const name of names) {
      if (forbiddenDependencies.has(name) || name.startsWith('@dotenvx/')) {
        failures.push(`${relative(file)}: forbidden dependency ${name}`)
      }
    }
  }
}

function checkPackageLock (file) {
  if (!fs.existsSync(file)) return
  checkTextFile(file, forbiddenPattern)
}

function checkTextFile (file, pattern = forbiddenPattern) {
  if (!fs.existsSync(file)) return
  const text = fs.readFileSync(file, 'utf8')
  if (pattern.test(text)) {
    failures.push(`${relative(file)}: forbidden dotenv reference`)
  }
}

function checkGoSum (file) {
  checkTextFile(file)
}

function walk (directory) {
  if (!fs.existsSync(directory)) return
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const full = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === 'node_modules' || entry.name === 'dist' || entry.name === 'bin') continue
      walk(full)
      continue
    }
    if (full === __filename) continue
    if (!/\.(go|ts|js)$/.test(entry.name)) continue
    checkTextFile(full)
  }
}

function relative (file) {
  return path.relative(root, file)
}
