import { execFile } from 'node:child_process'
import * as fs from 'node:fs'
import * as path from 'node:path'
import type { AdoptResult, ImportResult, Mutation, PurgeResult, Snapshot, ValuePayload } from './types'

export class EnvhError extends Error {
  constructor (readonly code: string, message: string) {
    super(message)
  }
}

export function binaryPath (extensionPath: string): string {
  return path.join(extensionPath, 'bin', process.platform === 'win32' ? 'envh.exe' : 'envh')
}

export function binaryExists (extensionPath: string): boolean {
  return fs.existsSync(binaryPath(extensionPath))
}

// Client for the bundled Go CLI. Arguments go straight to execFile; no shell is involved.
export class Envh {
  constructor (private readonly binary: string) {}

  snapshot (cwd: string): Promise<Snapshot> {
    return this.run<Snapshot>(cwd, ['snapshot'])
  }

  use (cwd: string, env: string): Promise<Mutation> {
    return this.run<Mutation>(cwd, ['use', env])
  }

  create (cwd: string, name: string): Promise<Mutation> {
    return this.run<Mutation>(cwd, ['create', '--', name])
  }

  rename (cwd: string, env: string, name: string): Promise<Mutation> {
    return this.run<Mutation>(cwd, ['rename', env, '--', name])
  }

  renameFile (cwd: string, env: string, file: string): Promise<Mutation> {
    return this.run<Mutation>(cwd, ['rename-file', env, '--', file])
  }

  duplicate (cwd: string, env: string, name: string): Promise<Mutation> {
    return this.run<Mutation>(cwd, ['duplicate', env, '--name', name])
  }

  remove (cwd: string, env: string): Promise<Mutation> {
    return this.run<Mutation>(cwd, ['remove', env, '--yes'])
  }

  color (cwd: string, env: string, color: string): Promise<Mutation> {
    return this.run<Mutation>(cwd, ['color', env, color])
  }

  move (cwd: string, env: string, direction: 'up' | 'down'): Promise<Mutation> {
    return this.run<Mutation>(cwd, ['move', env, direction])
  }

  importPostman (cwd: string, file: string, options: { dryRun?: boolean, force?: boolean }): Promise<ImportResult> {
    const args = ['import', 'postman', file]
    if (options.dryRun === true) args.push('--dry-run')
    if (options.force === true) args.push('--force')
    return this.run<ImportResult>(cwd, args)
  }

  adopt (cwd: string): Promise<AdoptResult> {
    return this.run<AdoptResult>(cwd, ['adopt'])
  }

  // dryRun only lists the project copies that are identical to storage. Nothing is deleted.
  purge (cwd: string, dryRun: boolean): Promise<PurgeResult> {
    return this.run<PurgeResult>(cwd, ['purge', dryRun ? '--dry-run' : '--yes'])
  }

  set (cwd: string, env: string, key: string, value: string): Promise<Mutation> {
    return this.run<Mutation>(cwd, ['set', env, key, '--', value])
  }

  unset (cwd: string, env: string, key: string): Promise<Mutation> {
    return this.run<Mutation>(cwd, ['unset', env, key])
  }

  value (cwd: string, env: string, key: string): Promise<ValuePayload> {
    return this.run<ValuePayload>(cwd, ['value', env, key, '--reveal'])
  }

  private run<T> (cwd: string, args: string[]): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      execFile(
        this.binary,
        ['--cwd', cwd, '--json', ...args],
        { encoding: 'utf8', shell: false, maxBuffer: 16 * 1024 * 1024 },
        (error, stdout, stderr) => {
          if (error !== null) {
            reject(toError(stderr, error))
            return
          }
          try {
            resolve(JSON.parse(stdout) as T)
          } catch {
            reject(new EnvhError('FAILED', 'envh returned unexpected output'))
          }
        }
      )
    })
  }
}

const MESSAGES: Record<string, string> = {
  ENV_NOT_FOUND: 'That environment no longer exists. Refresh and try again.',
  FILE_EXISTS: 'A file with that name already exists.',
  INVALID_NAME: 'Enter a name.',
  INVALID_FILE: 'Use a file name like .env.staging (templates such as .env.example are not allowed).',
  INVALID_COLOR: 'Pick a color from the list.',
  POSTMAN_INVALID: 'That file is not a Postman environment export.',
  ENCRYPTED_READ_ONLY: 'Encrypted values are read-only here.'
}

function toError (stderr: string, error: Error): EnvhError {
  try {
    const payload = JSON.parse(stderr) as { code?: unknown, message?: unknown }
    if (typeof payload.code === 'string' && typeof payload.message === 'string') {
      return new EnvhError(payload.code, MESSAGES[payload.code] ?? payload.message)
    }
  } catch {
    // Fall through to the process error when stderr is not JSON.
  }
  return new EnvhError('FAILED', error.message)
}
