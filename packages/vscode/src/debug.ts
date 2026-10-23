import * as vscode from 'vscode'
import type { Envh } from './cli'
import type { EnvStore } from './store'
import { errorMessage } from './store'

const DEFAULT_DEBUG_TYPES = [
  'node', 'pwa-node', 'node-terminal', 'python', 'debugpy',
  'go', 'delve', 'rust', 'lldb', 'cppdbg', 'gdb',
  'php', 'xdebug', 'ruby', 'coreclr', 'dotnet'
]

// Registers a DebugConfigurationProvider that injects the active environment
// into debug sessions when applyToDebug is enabled.
export function registerDebugProvider (
  context: vscode.ExtensionContext,
  envh: Envh,
  store: EnvStore
): void {
  const provider: vscode.DebugConfigurationProvider = {
    async resolveDebugConfiguration (
      folder: vscode.WorkspaceFolder | undefined,
      config: vscode.DebugConfiguration
    ): Promise<vscode.DebugConfiguration | undefined> {
      const settings = vscode.workspace.getConfiguration('envHandler')
      if (!settings.get<boolean>('applyToDebug', false)) {
        return config
      }
      if (!vscode.workspace.isTrusted) {
        return config
      }
      if (folder === undefined) {
        return config
      }
      const folderPath = folder.uri.fsPath
      const state = store.stateFor(folderPath)
      const snapshot = state?.overview?.branches.find(b => b.branch === state.overview?.currentBranch)
      if (snapshot === undefined || snapshot.environments.length === 0) {
        return config
      }
      const active = snapshot.environments.find(e => e.active)
      if (active === undefined) {
        return config
      }

      // Don't inject if the debug type isn't in the allowed list.
      const debugTypes = settings.get<string[]>('debugTypes', DEFAULT_DEBUG_TYPES)
      if (!debugTypes.includes(config.type)) {
        return config
      }

      // Remove a managed envFile reference so it doesn't override our injection.
      if (typeof config.envFile === 'string') {
        const envFiles = snapshot.environments.map(e => e.file)
        if (envFiles.includes(config.envFile)) {
          delete config.envFile
        }
      }

      let values: Record<string, string>
      try {
        const result = await envh.resolve(folderPath, active.file, true)
        values = result.values
      } catch {
        return config
      }

      // Inject env, with explicit config.env winning.
      const injected: Record<string, string> = { ...values }
      if (config.env !== undefined && typeof config.env === 'object') {
        Object.assign(injected, config.env as Record<string, string>)
      }
      config.env = injected
      return config
    }
  }

  // Register for all supported types. The setting filters at resolve time.
  for (const type of DEFAULT_DEBUG_TYPES) {
    context.subscriptions.push(
      vscode.debug.registerDebugConfigurationProvider(type, provider, vscode.DebugConfigurationProviderTriggerKind.Initial)
    )
  }
}

// resolve is called on the CLI wrapper. Add it to cli.ts.
declare module './cli' {
  interface Envh {
    resolve (cwd: string, env: string, skipEncrypted: boolean): Promise<{ values: Record<string, string> }>
  }
}
