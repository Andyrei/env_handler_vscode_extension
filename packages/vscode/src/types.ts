export interface Variable {
  key: string
  line: number
  encrypted: boolean
  multiline: boolean
}

export interface Environment {
  file: string
  name: string
  color: string
  order: number
  branch: string
  active: boolean
  layers: string[]
  variables: Variable[]
}

// One branch of the project. Root is that branch's env folder, outside the project.
export interface Snapshot {
  root: string
  sidecar: string
  project: string
  projectPath: string
  branch: string
  branches: string[]
  active: string
  environments: Environment[]
  warnings: string[]
}

export interface Overview {
  ok: boolean
  project: string
  projectPath: string
  currentBranch: string
  activeBranch: string
  activeFile: string
  branches: Snapshot[]
}

export interface Mutation {
  ok: boolean
  snapshot: Snapshot
}

export interface ImportResult {
  ok: boolean
  dryRun: boolean
  imported: string[]
  conflicts: string[]
  skippedKeys: string[]
  warnings: string[]
  snapshot?: Snapshot
}

export interface AdoptResult {
  ok: boolean
  dir: string
  copied: string[]
  same: string[]
  skipped: string[]
}

export interface PurgeResult {
  ok: boolean
  dryRun: boolean
  matched: string[]
  deleted: string[]
  kept: string[]
}

export interface ValuePayload {
  key: string
  value: string
}

export const COLORS = ['gray', 'green', 'yellow', 'red', 'blue', 'purple', 'orange'] as const
