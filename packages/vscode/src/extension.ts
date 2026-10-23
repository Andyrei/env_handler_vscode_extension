import { spawnSync } from 'node:child_process'
import path from 'node:path'
import * as vscode from 'vscode'

export function activate(context: vscode.ExtensionContext): void {
  context.subscriptions.push(
    vscode.commands.registerCommand('envHandler.showVersion', async () => {
      const version = readVersion(context)
      if (version === undefined) {
        await vscode.window.showErrorMessage(
          'Env Handler could not read the bundled envh binary. Rebuild the extension.'
        )
        return
      }
      await vscode.window.showInformationMessage(`Env Handler ${version}`)
    })
  )
}

export function deactivate(): void {}

function readVersion(context: vscode.ExtensionContext): string | undefined {
  const binary = path.join(
    context.extensionPath,
    'bin',
    process.platform === 'win32' ? 'envh.exe' : 'envh'
  )
  const result = spawnSync(binary, ['--version'], {
    encoding: 'utf8',
    shell: false
  })
  if (result.error !== undefined || result.status !== 0) {
    return undefined
  }
  const version = result.stdout.trim()
  return version.length > 0 ? version : undefined
}
