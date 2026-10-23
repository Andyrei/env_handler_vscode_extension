// Pure helpers shared by the extension and its tests. No vscode import.

export const KEY_PATTERN = /^[^\s=#:]+$/
export const FILE_PATTERN = /^\.env(\.[A-Za-z0-9_-]+)+$/

export function describeEnvironment (variableCount: number, encrypted: number, active: boolean): string {
  const parts: string[] = []
  if (active) parts.push('active')
  parts.push(`${variableCount} var${variableCount === 1 ? '' : 's'}`)
  if (encrypted > 0) parts.push(`${encrypted} encrypted`)
  return parts.join(' · ')
}

export function errorMessage (error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

export function themeColorName (color: string): string | undefined {
  const named = new Set(['red', 'blue', 'yellow', 'orange', 'green', 'purple'])
  return named.has(color) ? `charts.${color}` : undefined
}
