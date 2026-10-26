import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import * as vscode from 'vscode'
import type { EnvStore } from './store'

const exec = promisify(execFile)

export function registerTerminalEnv (
  context: vscode.ExtensionContext,
  binary: string,
  store: EnvStore
): void {
  const collection = context.environmentVariableCollection
  collection.description = 'Env Handler: active environment'
  collection.persistent = false // don't keep secrets around after VS Code closes

  const apply = async (): Promise<void> => {
    collection.clear()
    const enabled = vscode.workspace
      .getConfiguration('envHandler')
      .get<boolean>('injectIntoTerminals', false)
    const folder = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath
    if (!enabled || folder === undefined) return

    const active = store.activeEnvironment(folder)
    if (active === undefined) return

    try {
      const { stdout } = await exec(
        binary,
        ['--cwd', folder, 'resolve', '--branch', active.branch, '--env', active.file, '--json'],
        { maxBuffer: 10 * 1024 * 1024 }
      )
      const parsed = JSON.parse(stdout) as Record<string, unknown>
      const vars = (parsed.env ?? parsed) as Record<string, unknown>
      for (const [key, value] of Object.entries(vars)) {
        if (typeof value === 'string') collection.replace(key, value)
      }
    } catch (error) {
      void vscode.window.showWarningMessage(
        `Env Handler: could not load env for terminals (${error instanceof Error ? error.message : String(error)})`
      )
    }
  }

  context.subscriptions.push(
    store.onDidChange(() => { void apply() }),
    vscode.workspace.onDidChangeConfiguration(e => {
      if (e.affectsConfiguration('envHandler.injectIntoTerminals')) void apply()
    })
  )
  void apply()
}