import { execFile } from 'node:child_process'
import * as fs from 'node:fs'
import * as path from 'node:path'
import type { AdoptResult, ImportResult, Mutation, Overview, PurgeResult, ValuePayload } from './types'

export class EnvhError extends Error {
  constructor (readonly code: string, message: string) {
    super(message)
  }
}

// Which project and branch a command targets. Without a branch, the current branch is used.
export interface Scope {
  cwd: string
  branch?: string
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

  overview (cwd: string): Promise<Overview> {
    return this.run<Overview>(cwd, undefined, ['overview'])
  }

  use (scope: Scope, env: string): Promise<Mutation> {
    return this.run<Mutation>(scope.cwd, scope.branch, ['use', env])
  }

  create (scope: Scope, name: string): Promise<Mutation> {
    return this.run<Mutation>(scope.cwd, scope.branch, ['create', '--', name])
  }

  rename (scope: Scope, env: string, name: string): Promise<Mutation> {
    return this.run<Mutation>(scope.cwd, scope.branch, ['rename', env, '--', name])
  }

  renameFile (scope: Scope, env: string, file: string): Promise<Mutation> {
    return this.run<Mutation>(scope.cwd, scope.branch, ['rename-file', env, '--', file])
  }

  duplicate (scope: Scope, env: string, name: string): Promise<Mutation> {
    return this.run<Mutation>(scope.cwd, scope.branch, ['duplicate', env, '--name', name])
  }

  remove (scope: Scope, env: string): Promise<Mutation> {
    return this.run<Mutation>(scope.cwd, scope.branch, ['remove', env, '--yes'])
  }

  color (scope: Scope, env: string, color: string): Promise<Mutation> {
    return this.run<Mutation>(scope.cwd, scope.branch, ['color', env, color])
  }

  move (scope: Scope, env: string, direction: 'up' | 'down'): Promise<Mutation> {
    return this.run<Mutation>(scope.cwd, scope.branch, ['move', env, direction])
  }

  set (scope: Scope, env: string, key: string, value: string): Promise<Mutation> {
    return this.run<Mutation>(scope.cwd, scope.branch, ['set', env, key, '--', value])
  }

  unset (scope: Scope, env: string, key: string): Promise<Mutation> {
    return this.run<Mutation>(scope.cwd, scope.branch, ['unset', env, key])
  }

  value (scope: Scope, env: string, key: string): Promise<ValuePayload> {
    return this.run<ValuePayload>(scope.cwd, scope.branch, ['value', env, key, '--reveal'])
  }

  // Storage commands always act on the current branch.
  gitignore (cwd: string, dryRun: boolean): Promise<{ notIgnored: string[], added: string[] }> {
    return this.run<{ notIgnored: string[], added: string[] }>(cwd, undefined, ['gitignore', dryRun ? '--dry-run' : '--all'])
  }

  adopt (cwd: string): Promise<AdoptResult> {
    return this.run<AdoptResult>(cwd, undefined, ['adopt'])
  }

  // dryRun only lists the project copies that are identical to storage. Nothing is deleted.
  purge (cwd: string, dryRun: boolean): Promise<PurgeResult> {
    return this.run<PurgeResult>(cwd, undefined, ['purge', dryRun ? '--dry-run' : '--yes'])
  }

  importPostman (cwd: string, file: string, options: { dryRun?: boolean, force?: boolean }): Promise<ImportResult> {
    const args = ['import', 'postman', file]
    if (options.dryRun === true) args.push('--dry-run')
    if (options.force === true) args.push('--force')
    return this.run<ImportResult>(cwd, undefined, args)
  }

  private run<T> (cwd: string, branch: string | undefined, args: string[]): Promise<T> {
    const branchArgs = branch === undefined ? [] : ['--branch', branch]
    return new Promise<T>((resolve, reject) => {
      execFile(
        this.binary,
        ['--cwd', cwd, ...branchArgs, '--json', ...args],
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
