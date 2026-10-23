import * as vscode from 'vscode'
import type { EnvStore, FolderState } from './store'
import type { Environment, Variable } from './types'

export class FolderNode extends vscode.TreeItem {
  readonly folderPath: string

  constructor (folder: vscode.WorkspaceFolder, branch?: string) {
    super(folder.name, vscode.TreeItemCollapsibleState.Expanded)
    this.folderPath = folder.uri.fsPath
    this.description = branch === undefined ? undefined : `branch: ${branch}`
    this.contextValue = 'folder'
    this.iconPath = new vscode.ThemeIcon('folder')
  }
}

export class EnvNode extends vscode.TreeItem {
  constructor (readonly folderPath: string, readonly env: Environment) {
    super(
      env.name,
      env.variables.length > 0
        ? vscode.TreeItemCollapsibleState.Collapsed
        : vscode.TreeItemCollapsibleState.None
    )
    const encrypted = env.variables.filter(variable => variable.encrypted).length
    this.id = `${folderPath}::${env.file}`
    this.description = describe(env, encrypted)
    this.tooltip = `${env.file}\nLoads: ${env.layers.join(' → ')}`
    this.contextValue = env.active ? 'env.active' : 'env'
    this.iconPath = new vscode.ThemeIcon(
      env.active ? 'circle-filled' : 'circle-outline',
      themeColor(env.color)
    )
    this.command = { command: 'envHandler.setActive', title: 'Set Active', arguments: [this] }
  }
}

export class VariableNode extends vscode.TreeItem {
  constructor (readonly folderPath: string, readonly env: Environment, readonly variable: Variable) {
    super(variable.key, vscode.TreeItemCollapsibleState.None)
    this.id = `${folderPath}::${env.file}::${variable.key}::${variable.line}`
    this.description = variable.encrypted ? 'encrypted · read-only' : `line ${variable.line}`
    this.tooltip = variable.encrypted
      ? `${variable.key} is encrypted and read-only`
      : `${variable.key} · ${env.file}:${variable.line}`
    this.contextValue = variable.encrypted ? 'variable.encrypted' : 'variable'
    this.iconPath = new vscode.ThemeIcon(variable.encrypted ? 'lock' : 'symbol-key')
  }
}

export class MessageNode extends vscode.TreeItem {
  constructor (label: string, command?: vscode.Command) {
    super(label, vscode.TreeItemCollapsibleState.None)
    this.command = command
    this.iconPath = new vscode.ThemeIcon('info')
  }
}

export type TreeNode = FolderNode | EnvNode | VariableNode | MessageNode

export class EnvTreeProvider implements vscode.TreeDataProvider<TreeNode> {
  private readonly changed = new vscode.EventEmitter<TreeNode | undefined>()
  readonly onDidChangeTreeData = this.changed.event

  constructor (private readonly store: EnvStore) {}

  refresh (): void {
    this.changed.fire(undefined)
  }

  getTreeItem (node: TreeNode): vscode.TreeItem {
    return node
  }

  getChildren (node?: TreeNode): TreeNode[] {
    if (node instanceof FolderNode) {
      return this.environmentNodes(this.store.stateFor(node.folderPath))
    }
    if (node instanceof EnvNode) {
      return node.env.variables.map(variable => new VariableNode(node.folderPath, node.env, variable))
    }
    if (node !== undefined) {
      return []
    }

    const folders = this.store.folders()
    if (folders.length > 1) {
      return folders.map(state => new FolderNode(state.folder, state.snapshot?.branch))
    }
    return this.environmentNodes(folders[0])
  }

  private environmentNodes (state: FolderState | undefined): TreeNode[] {
    if (state === undefined) {
      return []
    }
    if (state.error !== undefined) {
      return [new MessageNode(`Could not read environments: ${state.error}`)]
    }
    const envs = state.snapshot?.environments ?? []
    if (envs.length === 0) {
      return [new MessageNode('No .env files yet. Create an environment.', {
        command: 'envHandler.createEnvironment',
        title: 'Create Environment'
      })]
    }
    const folderPath = state.folder.uri.fsPath
    return envs.map(env => new EnvNode(folderPath, env))
  }
}

function describe (env: Environment, encrypted: number): string {
  const parts: string[] = []
  if (env.active) {
    parts.push('active')
  }
  parts.push(`${env.variables.length} vars`)
  if (encrypted > 0) {
    parts.push(`${encrypted} encrypted`)
  }
  return parts.join(' · ')
}

function themeColor (color: string): vscode.ThemeColor | undefined {
  const named = new Set(['red', 'blue', 'yellow', 'orange', 'green', 'purple'])
  return named.has(color) ? new vscode.ThemeColor(`charts.${color}`) : undefined
}
