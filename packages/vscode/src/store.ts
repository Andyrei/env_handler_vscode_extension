import * as vscode from 'vscode'
import type { Envh } from './cli'
import type { Snapshot } from './types'

export interface FolderState {
  folder: vscode.WorkspaceFolder
  snapshot?: Snapshot
  error?: string
}

// Holds one snapshot per workspace folder, fetched through the Go CLI.
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

  // Where a folder's env files really live (the local store, not the project).
  rootFor (folderPath: string): string {
    return this.stateFor(folderPath)?.snapshot?.root ?? folderPath
  }

  async refresh (): Promise<void> {
    const folders = vscode.workspace.workspaceFolders ?? []
    this.states = await Promise.all(folders.map(async (folder): Promise<FolderState> => {
      try {
        return { folder, snapshot: await this.envh.snapshot(folder.uri.fsPath) }
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
      const root = state.snapshot?.root
      if (root !== undefined) {
        this.watchFolder(new vscode.RelativePattern(root, '**/*'))
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
