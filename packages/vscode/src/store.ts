import * as vscode from 'vscode'
import type { Envh } from './cli'
import type { Environment, Overview, Snapshot } from './types'

export interface FolderState {
  folder: vscode.WorkspaceFolder
  overview?: Overview
  error?: string
}

// Holds one overview per workspace folder, fetched through the Go CLI.
// Env files live in the local store, so the store folders and the git HEAD file are watched.
export class EnvStore implements vscode.Disposable {
  private states: FolderState[] = []
  private watchers: vscode.Disposable[] = []
  private timer: NodeJS.Timeout | undefined
  private readonly changed = new vscode.EventEmitter<void>()
  readonly onDidChange = this.changed.event

  constructor (private readonly envh: Envh) {}

  folders (): FolderState[] {
    return this.states
  }

  stateFor (folderPath: string): FolderState | undefined {
    return this.states.find(state => state.folder.uri.fsPath === folderPath)
  }

  // Every branch of a folder, with the current branch first.
  branches (folderPath: string): Snapshot[] {
    const overview = this.stateFor(folderPath)?.overview
    if (overview === undefined) return []
    return [...overview.branches].sort((a, b) => {
      if (a.branch === overview.currentBranch) return -1
      if (b.branch === overview.currentBranch) return 1
      return a.branch.localeCompare(b.branch)
    })
  }

  // The branch currently checked out in the folder.
  currentSnapshot (folderPath: string): Snapshot | undefined {
    const overview = this.stateFor(folderPath)?.overview
    return overview?.branches.find(branch => branch.branch === overview.currentBranch)
  }

  currentBranch (folderPath: string): string | undefined {
    return this.stateFor(folderPath)?.overview?.currentBranch
  }

  // Where a branch's env files really live (the local store, not the project).
  rootFor (folderPath: string, branch?: string): string {
    const snapshot = branch === undefined
      ? this.currentSnapshot(folderPath)
      : this.branches(folderPath).find(item => item.branch === branch)
    return snapshot?.root ?? folderPath
  }

  // The project-wide active environment, if it is set and still exists.
  activeEnvironment (folderPath: string): Environment | undefined {
    const overview = this.stateFor(folderPath)?.overview
    if (overview === undefined || overview.activeFile === '') return undefined
    const branch = overview.branches.find(item => item.branch === overview.activeBranch)
    return branch?.environments.find(env => env.file === overview.activeFile)
  }

  async refresh (): Promise<void> {
    const folders = vscode.workspace.workspaceFolders ?? []
    this.states = await Promise.all(folders.map(async (folder): Promise<FolderState> => {
      try {
        return { folder, overview: await this.envh.overview(folder.uri.fsPath) }
      } catch (error) {
        return { folder, error: errorMessage(error) }
      }
    }))
    this.syncWatchers()
    this.changed.fire()
  }

  dispose (): void {
    this.clearWatchers()
    this.changed.dispose()
  }

  private syncWatchers (): void {
    this.clearWatchers()
    for (const state of this.states) {
      for (const branch of state.overview?.branches ?? []) {
        this.watchFolder(new vscode.RelativePattern(branch.root, '**/*'))
      }
      this.watchFolder(new vscode.RelativePattern(state.folder.uri.fsPath, '.git/HEAD'))
    }
  }

  private watchFolder (pattern: vscode.RelativePattern): void {
    const watcher = vscode.workspace.createFileSystemWatcher(pattern)
    const schedule = (): void => this.scheduleRefresh()
    this.watchers.push(
      watcher,
      watcher.onDidCreate(schedule),
      watcher.onDidChange(schedule),
      watcher.onDidDelete(schedule)
    )
  }

  private clearWatchers (): void {
    for (const watcher of this.watchers) {
      watcher.dispose()
    }
    this.watchers = []
  }

  private scheduleRefresh (): void {
    if (this.timer !== undefined) {
      clearTimeout(this.timer)
    }
    this.timer = setTimeout(() => {
      this.timer = undefined
      void this.refresh()
    }, 200)
  }
}

export function errorMessage (error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}
