import { spawnSync } from 'node:child_process'
import * as fs from 'node:fs'
import * as path from 'node:path'
import * as vscode from 'vscode'
import { EnvActions } from './actions'
import { binaryPath, Envh } from './cli'
import { EnvPanel } from './panel'
import { EnvStore } from './store'
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
    const state = folder === undefined ? undefined : store.stateFor(folder)
    if (state?.snapshot === undefined) {
      statusItem.hide()
      return
    }
    const active = state.snapshot.environments.find(env => env.active)
    statusItem.text = `$(symbol-variable) ${active?.name ?? 'No environment'} · $(git-branch) ${state.snapshot.branch}`
    statusItem.tooltip = `Active environment on branch ${state.snapshot.branch}. Click to switch.`
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
      const root = store.rootFor(folder)
      await vscode.window.showTextDocument(vscode.Uri.file(path.join(root, ...env.file.split('/'))))
    }))
  )

  if (!fs.existsSync(binary)) {
    void vscode.window.showErrorMessage('Env Handler: the bundled envh binary is missing. Run npm run build.')
  }
  void store.refresh()
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
    for (const env of state.snapshot?.environments ?? []) {
      picks.push({
        label: env.name,
        description: env.file,
        detail: store.folders().length > 1 ? state.folder.name : undefined,
        folderPath: state.folder.uri.fsPath,
        env
      })
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
