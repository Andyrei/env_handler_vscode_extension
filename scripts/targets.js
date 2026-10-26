'use strict'

// Marketplace target names and the Go toolchain pair used to build them.
const TARGETS = [
  { target: 'darwin-arm64', goos: 'darwin', goarch: 'arm64' },
  { target: 'darwin-x64', goos: 'darwin', goarch: 'amd64' },
  { target: 'linux-x64', goos: 'linux', goarch: 'amd64' },
  { target: 'linux-arm64', goos: 'linux', goarch: 'arm64' },
  { target: 'win32-x64', goos: 'windows', goarch: 'amd64' },
  { target: 'win32-arm64', goos: 'windows', goarch: 'arm64' }
]

function targetInfo (target) {
  const info = TARGETS.find(item => item.target === target)
  if (info === undefined) {
    throw new Error(`unknown target ${target}; expected one of ${TARGETS.map(item => item.target).join(', ')}`)
  }
  return info
}

function currentTarget (platform = process.platform, arch = process.arch) {
  const normalizedArch = arch === 'x64' ? 'x64' : arch === 'arm64' ? 'arm64' : arch
  const target = `${platform}-${normalizedArch}`
  return targetInfo(target).target
}

function binaryName (target) {
  return targetInfo(target).goos === 'windows' ? 'envh.exe' : 'envh'
}

module.exports = { TARGETS, targetInfo, currentTarget, binaryName }
