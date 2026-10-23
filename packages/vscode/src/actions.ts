import * as path from 'node:path'
import * as vscode from 'vscode'
import type { Envh, Scope } from './cli'
import { EnvhError } from './cli'
import type { EnvStore } from './store'
import { errorMessage, FILE_PATTERN, KEY_PATTERN } from './helpers'
import { type AdoptResult, COLORS, type Environment, type ImportResult, type Mutation, type PurgeResult } from './types'

// Every environment and variable change goes through here, and through the Go CLI.
// Each change targets the branch the environment belongs to.
export class EnvActions {
  constructor (private readonly envh: Envh, private readonly store: EnvStore) {}

  async create (folderPath: string): Promise<void> {
    const name = await this.prompt({ prompt: 'Environment name', placeHolder: 'For example Local, Staging, Production' }, true)
    if (name === undefined) return
    await this.run(folderPath, undefined, () => this.envh.create({ cwd: folderPath }, name), `Created ${name}`)
  }

  async rename (folderPath: string, env: Environment): Promise<void> {
    const name = await this.prompt({ prompt: 'New display name', value: env.name }, true)
    if (name === undefined || name === env.name) return
    await this.run(folderPath, env, () => this.envh.rename(scopeOf(folderPath, env), env.file, name), `Renamed to ${name}`)
  }

  async renameFile (folderPath: string, env: Environment): Promise<void> {
    const file = await this.prompt({
      prompt: 'New file name (for example .env.staging)',
      value: env.file,
      validateInput: value => (FILE_PATTERN.test(value.trim()) ? undefined : 'Use a name like .env.staging')
    }, true)
    if (file === undefined || file === env.file) return
    await this.run(folderPath, env, () => this.envh.renameFile(scopeOf(folderPath, env), env.file, file), `Renamed file to ${file}`)
  }

  async duplicate (folderPath: string, env: Environment): Promise<void> {
    const name = await this.prompt({ prompt: 'Name for the copy', value: `${env.name} Copy` }, true)
    if (name === undefined) return
    await this.run(folderPath, env, () => this.envh.duplicate(scopeOf(folderPath, env), env.file, name), `Duplicated as ${name}`)
  }

  async activate (folderPath: string, env: Environment): Promise<void> {
    if (env.active) return
    await this.run(folderPath, env, () => this.envh.use(scopeOf(folderPath, env), env.file), `Active: ${env.name} on ${env.branch}`)
  }

  async pickColor (folderPath: string, env: Environment): Promise<void> {
    const picked = await vscode.window.showQuickPick(
      COLORS.map(color => ({ label: color, picked: color === env.color })),
      { placeHolder: `Color for ${env.name}` }
    )
    if (picked === undefined) return
    await this.setColor(folderPath, env, picked.label)
  }

  async setColor (folderPath: string, env: Environment, color: string): Promise<void> {
    if (color === env.color) return
    await this.run(folderPath, env, () => this.envh.color(scopeOf(folderPath, env), env.file, color), `${env.name} is now ${color}`)
  }

  async move (folderPath: string, env: Environment, direction: 'up' | 'down'): Promise<void> {
    await this.run(folderPath, env, () => this.envh.move(scopeOf(folderPath, env), env.file, direction), `Moved ${env.name} ${direction}`)
  }

  async remove (folderPath: string, env: Environment): Promise<void> {
    if (!this.canEdit()) return
    const choice = await vscode.window.showWarningMessage(
      `Delete ${env.file} (${env.name}) on ${env.branch}? The file is removed from local storage.`,
      { modal: true },
      'Delete'
    )
    if (choice !== 'Delete') return
    await this.run(folderPath, env, () => this.envh.remove(scopeOf(folderPath, env), env.file), `Moved ${env.name} to trash`)
  }

  async addVariable (folderPath: string, env: Environment): Promise<void> {
    const key = await this.prompt({
      prompt: `New variable in ${env.name}`,
      placeHolder: 'KEY_NAME',
      validateInput: value => (KEY_PATTERN.test(value) ? undefined : 'Use no spaces, =, :, or #')
    }, true)
    if (key === undefined) return
    if (env.variables.some(variable => variable.key === key)) {
      const choice = await vscode.window.showWarningMessage(
        `${key} already exists in ${env.name}. Replace its value?`,
        { modal: true },
        'Replace'
      )
      if (choice !== 'Replace') return
    }
    const value = await vscode.window.showInputBox({ prompt: `Value for ${key}`, ignoreFocusOut: true })
    if (value === undefined) return
    await this.run(folderPath, env, () => this.envh.set(scopeOf(folderPath, env), env.file, key, value), `Set ${key}`)
  }

  async editVariable (folderPath: string, env: Environment, key: string, encrypted: boolean): Promise<void> {
    if (encrypted) {
      await vscode.window.showInformationMessage('Encrypted values are read-only here. Edit the file directly.')
      return
    }
    let current: string
    try {
      current = (await this.envh.value(scopeOf(folderPath, env), env.file, key)).value
    } catch (error) {
      await vscode.window.showErrorMessage(errorMessage(error))
      return
    }
    const value = await vscode.window.showInputBox({ prompt: `New value for ${key}`, value: current, ignoreFocusOut: true })
    if (value === undefined || value === current) return
    await this.run(folderPath, env, () => this.envh.set(scopeOf(folderPath, env), env.file, key, value), `Updated ${key}`)
  }

  async deleteVariable (folderPath: string, env: Environment, key: string, encrypted: boolean): Promise<void> {
    if (encrypted) {
      await vscode.window.showInformationMessage('Encrypted values are read-only here. Edit the file directly.')
      return
    }
    const choice = await vscode.window.showWarningMessage(`Delete ${key} from ${env.name}? (moved to trash)`, { modal: true }, 'Delete')
    if (choice !== 'Delete') return
    await this.run(folderPath, env, () => this.envh.unset(scopeOf(folderPath, env), env.file, key), `Deleted ${key}`)
  }

  async copyValue (folderPath: string, env: Environment, key: string, encrypted: boolean): Promise<void> {
    if (encrypted) {
      await vscode.window.showInformationMessage('Encrypted values are read-only here.')
      return
    }
    try {
      const payload = await this.envh.value(scopeOf(folderPath, env), env.file, key)
      await vscode.env.clipboard.writeText(payload.value)
      vscode.window.setStatusBarMessage(`Copied ${key}`, 2000)
    } catch (error) {
      await vscode.window.showErrorMessage(errorMessage(error))
    }
  }

  async importPostman (folderPath: string): Promise<void> {
    if (!this.canEdit()) return
    const picked = await vscode.window.showOpenDialog({
      canSelectMany: false,
      openLabel: 'Import Postman environment',
      filters: { 'Postman environment': ['json'], 'All files': ['*'] }
    })
    const file = picked?.[0]?.fsPath
    if (file === undefined) return

    let preview: ImportResult
    try {
      preview = await this.envh.importPostman(folderPath, file, { dryRun: true })
    } catch (error) {
      await vscode.window.showErrorMessage(errorMessage(error))
      return
    }
    if (preview.imported.length === 0 && preview.conflicts.length === 0) {
      await vscode.window.showWarningMessage('No environments were found in that file.')
      return
    }

    let force = false
    if (preview.conflicts.length > 0) {
      const choice = await vscode.window.showWarningMessage(
        `${preview.conflicts.join(', ')} already exist on this branch. Overwrite them?`,
        { modal: true },
        'Overwrite',
        'Import new only'
      )
      if (choice === undefined) return
      force = choice === 'Overwrite'
      if (force && preview.conflicts.some(name => this.isDirty(this.store.rootFor(folderPath), name))) {
        await vscode.window.showWarningMessage('Save the open files that would be overwritten first.')
        return
      }
    }

    const toWrite = force ? [...preview.imported] : preview.imported
    if (toWrite.length === 0) return
    const confirm = await vscode.window.showInformationMessage(
      `Import ${toWrite.join(', ')} on this branch? Values are stored as plaintext in these files.`,
      { modal: true },
      'Import'
    )
    if (confirm !== 'Import') return

    try {
      const result = await this.envh.importPostman(folderPath, file, { force })
      await this.store.refresh()
      const notes = [`Imported ${result.imported.join(', ')}.`]
      if (result.skippedKeys.length > 0) notes.push(`Skipped unsafe keys: ${result.skippedKeys.join(', ')}.`)
      await vscode.window.showInformationMessage(notes.join(' '))
      for (const warning of result.warnings) {
        void vscode.window.showWarningMessage(warning)
      }
    } catch (error) {
      await vscode.window.showErrorMessage(errorMessage(error))
    }
  }

  // Enables swap mode: .env becomes a symlink/copy of the active env file.
  async swapActiveEnvironment (folderPath: string): Promise<void> {
    if (!this.canEdit()) return
    const state = this.store.stateFor(folderPath)
    const snapshot = state?.overview?.branches.find(b => b.branch === state.overview?.currentBranch)
    const active = snapshot?.environments.find(e => e.active)
    if (active === undefined) {
      await vscode.window.showWarningMessage('Activate an environment first.')
      return
    }
    const confirm = await vscode.window.showWarningMessage(
      `Swap will replace .env with ${active.file} (${active.name}) and back up the current .env. Layering is bypassed while swap is active.`,
      { modal: true },
      'Swap'
    )
    if (confirm !== 'Swap') return
    try {
      await this.envh.runCommand(folderPath, undefined, ['swap', '--env', active.file])
      await this.store.refresh()
      vscode.window.setStatusBarMessage(`Swapped .env → ${active.file}`, 4000)
    } catch (error) {
      await vscode.window.showErrorMessage(errorMessage(error))
    }
  }

  // Restores .env from the swap backup.
  async restoreSwap (folderPath: string): Promise<void> {
    if (!this.canEdit()) return
    try {
      await this.envh.runCommand(folderPath, undefined, ['swap', '--restore', '--force'])
      await this.store.refresh()
      vscode.window.setStatusBarMessage('Restored .env from backup', 4000)
    } catch (error) {
      await vscode.window.showErrorMessage(errorMessage(error))
    }
  }

  // Checks if plaintext env files are gitignored and offers to add them.
  async addToGitignore (folderPath: string): Promise<void> {
    if (!this.canEdit()) return
    try {
      const preview = await this.envh.gitignore(folderPath, true)
      if (preview.notIgnored.length === 0) {
        await vscode.window.showInformationMessage('All plaintext env files are already gitignored.')
        return
      }
      const choice = await vscode.window.showWarningMessage(
        `These env files with plaintext values are not gitignored:\\n\\n${preview.notIgnored.join('\\n')}\\n\\nAdd them to .gitignore?`,
        { modal: true },
        'Add to .gitignore'
      )
      if (choice !== 'Add to .gitignore') return
      const result = await this.envh.gitignore(folderPath, false)
      await vscode.window.showInformationMessage(`Added to .gitignore: ${result.added.join(', ')}`)
    } catch (error) {
      await vscode.window.showErrorMessage(errorMessage(error))
    }
  }

  // Copies the current branch's project env files into local storage. It never deletes anything.
  async adoptProjectFiles (folderPath: string): Promise<void> {
    if (!this.canEdit()) return
    let result: AdoptResult
    try {
      result = await this.envh.adopt(folderPath)
    } catch (error) {
      await vscode.window.showErrorMessage(errorMessage(error))
      return
    }
    await this.store.refresh()

    const stored = result.copied.length + result.same.length
    if (stored === 0) {
      await vscode.window.showInformationMessage('No env files were found in the project folder.')
      return
    }
    const notes = [`${stored} env file(s) are stored locally.`]
    if (result.skipped.length > 0) {
      notes.push(`${result.skipped.length} file(s) differ from the stored copy and were not copied.`)
    }
    notes.push('Project files were not changed. Use "Delete Project Env Copies" when you are ready.')
    vscode.window.setStatusBarMessage(notes.join(' '), 6000)
  }

  // Deletes project copies only when the stored copy is identical. Cancel is the first item.
  async deleteProjectCopies (folderPath: string): Promise<void> {
    if (!this.canEdit()) return
    let preview: PurgeResult
    try {
      preview = await this.envh.purge(folderPath, true)
    } catch (error) {
      await vscode.window.showErrorMessage(errorMessage(error))
      return
    }
    if (preview.matched.length === 0) {
      await vscode.window.showInformationMessage('No project env files match a stored copy, so nothing can be deleted safely.')
      return
    }
    const picked = await vscode.window.showQuickPick(
      [
        { label: 'Cancel' },
        { label: `Delete ${preview.matched.length} project file(s)`, detail: preview.matched.join(', ') }
      ],
      {
        placeHolder: `Each file has an identical copy in local storage. ${preview.kept.length} other file(s) stay in the project.`,
        ignoreFocusOut: true
      }
    )
    if (picked === undefined || picked.label === 'Cancel') return
    try {
      const result = await this.envh.purge(folderPath, false)
      await this.store.refresh()
      vscode.window.setStatusBarMessage(`Deleted ${result.deleted.length} project file(s)`, 4000)
    } catch (error) {
      await vscode.window.showErrorMessage(errorMessage(error))
    }
  }

  // Shared path for every mutation: trust check, unsaved-edit check, CLI call, refresh.
  private async run (
    folderPath: string,
    env: Environment | undefined,
    task: () => Promise<Mutation>,
    done: string
  ): Promise<void> {
    if (!this.canEdit()) return
    if (env !== undefined && this.isDirty(this.store.rootFor(folderPath, env.branch), env.file)) {
      await vscode.window.showWarningMessage(`Save ${env.file} before changing it from Env Handler.`)
      return
    }
    try {
      await task()
      vscode.window.setStatusBarMessage(done, 3000)
      await this.store.refresh()
    } catch (error) {
      if (error instanceof EnvhError && error.code === 'FILE_EXISTS') {
        await vscode.window.showErrorMessage(error.message)
        return
      }
      await vscode.window.showErrorMessage(errorMessage(error))
    }
  }

  private async prompt (options: vscode.InputBoxOptions, required: boolean): Promise<string | undefined> {
    const { validateInput, ...rest } = options
    const value = await vscode.window.showInputBox({
      ignoreFocusOut: true,
      ...rest,
      validateInput: input => {
        if (required && input.trim().length === 0) return 'Enter a value'
        return validateInput?.(input)
      }
    })
    return value?.trim()
  }

  private canEdit (): boolean {
    if (vscode.workspace.isTrusted) return true
    void vscode.window.showWarningMessage('Env Handler cannot change files in an untrusted workspace.')
    return false
  }

  private isDirty (root: string, file: string): boolean {
    const target = path.join(root, ...file.split('/'))
    return vscode.workspace.textDocuments.some(
      document => document.isDirty && document.uri.scheme === 'file' && document.uri.fsPath === target
    )
  }
}

// Commands run in the project folder, against the branch the environment belongs to.
function scopeOf (folderPath: string, env: Environment): Scope {
  return { cwd: folderPath, branch: env.branch }
}
