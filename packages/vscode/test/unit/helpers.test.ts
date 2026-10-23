import { describeEnvironment, errorMessage, FILE_PATTERN, KEY_PATTERN, themeColorName } from '../../src/helpers'
import assert from 'node:assert/strict'
import { test } from 'node:test'

test('KEY_PATTERN accepts valid keys', () => {
  assert.equal(KEY_PATTERN.test('API_KEY'), true)
  assert.equal(KEY_PATTERN.test('_PRIVATE'), true)
  assert.equal(KEY_PATTERN.test('a.b'), true)
})

test('KEY_PATTERN rejects invalid keys', () => {
  assert.equal(KEY_PATTERN.test(''), false)
  assert.equal(KEY_PATTERN.test('A B'), false)
  assert.equal(KEY_PATTERN.test('A=B'), false)
  assert.equal(KEY_PATTERN.test('A:B'), false)
  assert.equal(KEY_PATTERN.test('A#B'), false)
})

test('FILE_PATTERN accepts env file names', () => {
  assert.equal(FILE_PATTERN.test('.env.local'), true)
  assert.equal(FILE_PATTERN.test('.env.staging'), true)
  assert.equal(FILE_PATTERN.test('.env.development.local'), true)
})

test('FILE_PATTERN rejects non-env names', () => {
  assert.equal(FILE_PATTERN.test('.env'), false)
  assert.equal(FILE_PATTERN.test('.env.example'), true)
  assert.equal(FILE_PATTERN.test('README.md'), false)
})

test('describeEnvironment formats correctly', () => {
  assert.equal(describeEnvironment(0, 0, false), '0 vars')
  assert.equal(describeEnvironment(1, 0, true), 'active · 1 var')
  assert.equal(describeEnvironment(5, 2, false), '5 vars · 2 encrypted')
  assert.equal(describeEnvironment(3, 0, true), 'active · 3 vars')
})

test('themeColorName returns theme color for known colors', () => {
  assert.equal(themeColorName('red'), 'charts.red')
  assert.equal(themeColorName('blue'), 'charts.blue')
  assert.equal(themeColorName('unknown'), undefined)
  assert.equal(themeColorName(''), undefined)
})

test('errorMessage extracts message from Error', () => {
  assert.equal(errorMessage(new Error('boom')), 'boom')
  assert.equal(errorMessage('string error'), 'string error')
  assert.equal(errorMessage(42), '42')
})