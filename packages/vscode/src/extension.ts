import { spawnSync } from 'node:child_process'
import * as fs from 'node:fs'
import * as path from 'node:path'
import * as vscode from 'vscode'
import { EnvActions } from './actions'
import { binaryPath, Envh } from './cli'
import { EnvPanel } from './panel'
import { errorMessage, EnvStore } from './store'
import { registerDebugProvider } from './debug'
import { EnvNode, EnvTreeProvider, FolderNode, VariableNode } from './tree'
import type { Environment } from './types'

interface FolderPick extends vscode.QuickPickItem {
  folderPath: string
}

interface EnvPick extends vscode.QuickPickItem {
  folderPath: string
  env: Environment
}

export function activate (context: vscode.ExtensionContext): void {
  const binary = binaryPath(context.extensionPath)
  const envh = new Envh(binary)
  const store = new EnvStore(envh)
  const actions = new EnvActions(envh, store)
  const provider = new EnvTreeProvider(store)
  const view = vscode.window.createTreeView('envHandler.tree', {
    treeDataProvider: provider,
    showCollapseAll: true
  })
  const statusItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100)
  statusItem.command = 'envHandler.switchEnvironment'

  const updateStatus = (): void => {
    const folder = activeFolderPath() ?? store.folders()[0]?.folder.uri.fsPath
    if (folder === undefined || store.stateFor(folder)?.overview === undefined) {
      statusItem.hide()
      return
    }
    const active = store.activeEnvironment(folder)
    const current = store.currentBranch(folder) ?? ''
    statusItem.text = active === undefined
      ? `$(symbol-variable) No environment · $(git-branch) ${current}`
      : `$(symbol-variable) ${active.name} (${active.branch}) · $(git-branch) ${current}`
    statusItem.tooltip = 'Active environment for this project. Click to switch.'
    statusItem.show()
  }

  context.subscriptions.push(
    store,
    view,
    statusItem,
    store.onDidChange(() => {
      provider.refresh()
      updateStatus()
    }),
    vscode.window.onDidChangeActiveTextEditor(() => updateStatus()),
    vscode.window.onDidChangeWindowState(state => {
      if (state.focused) void store.refresh()
    }),
    vscode.workspace.onDidChangeWorkspaceFolders(() => {
      void store.refresh()
    }),
    vscode.commands.registerCommand('envHandler.showVersion', () => showVersion(context)),
    vscode.commands.registerCommand('envHandler.refresh', () => store.refresh()),
    vscode.commands.registerCommand('envHandler.openPanel', () => EnvPanel.show(store, actions)),
    vscode.commands.registerCommand('envHandler.switchEnvironment', () => switchEnvironment(store, actions)),
    vscode.commands.registerCommand('envHandler.createEnvironment', async (arg?: unknown) => {
      const folderPath = await resolveFolder(arg)
      if (folderPath !== undefined) await actions.create(folderPath)
    }),
    vscode.commands.registerCommand('envHandler.importPostman', async (arg?: unknown) => {
      const folderPath = await resolveFolder(arg)
      if (folderPath !== undefined) await actions.importPostman(folderPath)
    }),
    vscode.commands.registerCommand('envHandler.adoptProjectFiles', async (arg?: unknown) => {
      const folderPath = await resolveFolder(arg)
      if (folderPath !== undefined) await actions.adoptProjectFiles(folderPath)
    }),
    vscode.commands.registerCommand('envHandler.addToGitignore', async (arg?: unknown) => {
      const folderPath = await resolveFolder(arg)
      if (folderPath !== undefined) await actions.addToGitignore(folderPath)
    }),
    vscode.commands.registerCommand('envHandler.swapActiveEnvironment', async (arg?: unknown) => {
      const folderPath = await resolveFolder(arg)
      if (folderPath !== undefined) await actions.swapActiveEnvironment(folderPath)
    }),
    vscode.commands.registerCommand('envHandler.restoreSwap', async (arg?: unknown) => {
      const folderPath = await resolveFolder(arg)
      if (folderPath !== undefined) await actions.restoreSwap(folderPath)
    }),
    vscode.commands.registerCommand('envHandler.restoreFromTrash', async (arg?: unknown) => {
      const folderPath = await resolveFolder(arg)
      if (folderPath === undefined) return
      const files = await vscode.window.showInputBox({ prompt: 'File to restore from trash (e.g. .env.local)' })
      if (files === undefined || files.trim().length === 0) return
      try {
        const result = await spawnSync(binaryPath(context.extensionPath), ['--cwd', folderPath, 'restore', files.trim()], { encoding: 'utf8', shell: false })
        if (result.status !== 0) {
          await vscode.window.showErrorMessage(result.stderr.trim() || 'Restore failed')
          return
        }
        await store.refresh()
        vscode.window.setStatusBarMessage(`Restored ${files.trim()}`, 3000)
      } catch (error) {
        await vscode.window.showErrorMessage(errorMessage(error))
      }
    }),
    vscode.commands.registerCommand('envHandler.deleteProjectCopies', async (arg?: unknown) => {
      const folderPath = await resolveFolder(arg)
      if (folderPath !== undefined) await actions.deleteProjectCopies(folderPath)
    }),
    vscode.commands.registerCommand('envHandler.setActive', (arg?: unknown) => withEnv(arg, (folder, env) =>
      actions.activate(folder, env)
    )),
    vscode.commands.registerCommand('envHandler.renameEnvironment', (arg?: unknown) => withEnv(arg, (folder, env) =>
      actions.rename(folder, env)
    )),
    vscode.commands.registerCommand('envHandler.renameEnvironmentFile', (arg?: unknown) => withEnv(arg, (folder, env) =>
      actions.renameFile(folder, env)
    )),
    vscode.commands.registerCommand('envHandler.duplicateEnvironment', (arg?: unknown) => withEnv(arg, (folder, env) =>
      actions.duplicate(folder, env)
    )),
    vscode.commands.registerCommand('envHandler.deleteEnvironment', (arg?: unknown) => withEnv(arg, (folder, env) =>
      actions.remove(folder, env)
    )),
    vscode.commands.registerCommand('envHandler.setColor', (arg?: unknown) => withEnv(arg, (folder, env) =>
      actions.pickColor(folder, env)
    )),
    vscode.commands.registerCommand('envHandler.moveUp', (arg?: unknown) => withEnv(arg, (folder, env) =>
      actions.move(folder, env, 'up')
    )),
    vscode.commands.registerCommand('envHandler.moveDown', (arg?: unknown) => withEnv(arg, (folder, env) =>
      actions.move(folder, env, 'down')
    )),
    vscode.commands.registerCommand('envHandler.addVariable', (arg?: unknown) => withEnv(arg, (folder, env) =>
      actions.addVariable(folder, env)
    )),
    vscode.commands.registerCommand('envHandler.editVariable', (arg?: unknown) => withVariable(arg, (folder, env, variable) =>
      actions.editVariable(folder, env, variable.key, variable.encrypted)
    )),
    vscode.commands.registerCommand('envHandler.deleteVariable', (arg?: unknown) => withVariable(arg, (folder, env, variable) =>
      actions.deleteVariable(folder, env, variable.key, variable.encrypted)
    )),
    vscode.commands.registerCommand('envHandler.copyValue', (arg?: unknown) => withVariable(arg, (folder, env, variable) =>
      actions.copyValue(folder, env, variable.key, variable.encrypted)
    )),
    vscode.commands.registerCommand('envHandler.openFile', (arg?: unknown) => withEnv(arg, async (folder, env) => {
      const root = store.rootFor(folder, env.branch)
      await vscode.window.showTextDocument(vscode.Uri.file(path.join(root, ...env.file.split('/'))))
    })),
    vscode.commands.registerCommand('envHandler.runWithEnvironment', async (arg?: unknown) => {
      const folderPath = await resolveFolder(arg)
      if (folderPath === undefined) return
      const active = store.activeEnvironment(folderPath)
      if (active === undefined) {
        await vscode.window.showInformationMessage('Activate an environment first.')
        return
      }
      const command = await vscode.window.showInputBox({
        prompt: `Command to run with ${active.name} (${active.branch})`,
        value: vscode.workspace.getConfiguration('envHandler').get<string>('runCommand', ''),
        placeHolder: 'npm start',
        ignoreFocusOut: true
      })
      if (command === undefined || command.trim().length === 0) return

      const terminal = vscode.window.createTerminal({
        name: `Env Handler: ${active.name}`,
        cwd: folderPath
      })
      const binary = binaryPath(context.extensionPath)
      terminal.sendText(runLine(binary, folderPath, active.branch, active.file, command))
      terminal.show()
    })
  )

  if (!fs.existsSync(binary)) {
    void vscode.window.showErrorMessage('Env Handler: the bundled envh binary is missing. Run npm run build.')
  }
  registerDebugProvider(context, envh, store)
  void store.refresh()
  void vscode.window.showInformationMessage('Env Handler activated')
}

export function deactivate (): void {}

async function withEnv (
  arg: unknown,
  run: (folderPath: string, env: Environment) => Promise<void>
): Promise<void> {
  if (arg instanceof EnvNode) {
    await run(arg.folderPath, arg.env)
    return
  }
  // Called from the Command Palette: nothing to act on without a picker.
  await vscode.window.showInformationMessage('Use this command from the Environments view or the Environment Manager.')
}

async function withVariable (
  arg: unknown,
  run: (folderPath: string, env: Environment, variable: { key: string, encrypted: boolean }) => Promise<void>
): Promise<void> {
  if (arg instanceof VariableNode) {
    await run(arg.folderPath, arg.env, { key: arg.variable.key, encrypted: arg.variable.encrypted })
  }
}

async function switchEnvironment (store: EnvStore, actions: EnvActions): Promise<void> {
  const picks: EnvPick[] = []
  for (const state of store.folders()) {
    const folderPath = state.folder.uri.fsPath
    for (const branch of store.branches(folderPath)) {
      for (const env of branch.environments) {
        picks.push({
          label: env.name,
          description: `${env.branch} · ${env.file}`,
          detail: store.folders().length > 1 ? state.folder.name : undefined,
          folderPath,
          env
        })
      }
    }
  }
  if (picks.length === 0) {
    await vscode.window.showInformationMessage('No environments yet. Create one from the Environments view.')
    return
  }
  const picked = await vscode.window.showQuickPick(picks, { placeHolder: 'Choose the active environment' })
  if (picked !== undefined) {
    await actions.activate(picked.folderPath, picked.env)
  }
}

function activeFolderPath (): string | undefined {
  const uri = vscode.window.activeTextEditor?.document.uri
  if (uri === undefined) return undefined
  return vscode.workspace.getWorkspaceFolder(uri)?.uri.fsPath
}

async function resolveFolder (arg: unknown): Promise<string | undefined> {
  if (arg instanceof FolderNode || arg instanceof EnvNode || arg instanceof VariableNode) {
    return arg.folderPath
  }
  const folders = vscode.workspace.workspaceFolders ?? []
  if (folders.length === 0) {
    await vscode.window.showWarningMessage('Open a folder to use Env Handler.')
    return undefined
  }
  if (folders.length === 1) {
    return folders[0].uri.fsPath
  }
  const picked = await vscode.window.showQuickPick<FolderPick>(
    folders.map(folder => ({
      label: folder.name,
      description: folder.uri.fsPath,
      folderPath: folder.uri.fsPath
    })),
    { placeHolder: 'Choose a workspace folder' }
  )
  return picked?.folderPath
}

function runLine (binary: string, folderPath: string, branch: string, file: string, command: string): string {
  const windows = process.platform === 'win32'
  const powershell = windows && /(?:powershell|pwsh)(?:\.exe)?$/i.test(vscode.env.shell)
  const quote = powershell
    ? (value: string): string => `'${value.replace(/'/g, "''")}'`
    : windows
      ? (value: string): string => `"${value.replace(/"/g, '""')}"`
      : (value: string): string => `'${value.replace(/'/g, `'\\''`)}'`
  const prefix = powershell ? '& ' : ''
  return `${prefix}${quote(binary)} run --cwd ${quote(folderPath)} --branch ${quote(branch)} --env ${quote(file)} -- ${command}`
}

function showVersion (context: vscode.ExtensionContext): void {
  const result = spawnSync(binaryPath(context.extensionPath), ['--version'], {
    encoding: 'utf8',
    shell: false
  })
  if (result.error !== undefined || result.status !== 0) {
    void vscode.window.showErrorMessage('Env Handler could not read the bundled envh binary. Rebuild the extension.')
    return
  }
  void vscode.window.showInformationMessage(`Env Handler ${result.stdout.trim()}`)
}
