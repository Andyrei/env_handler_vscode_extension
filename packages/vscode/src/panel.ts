import * as crypto from 'node:crypto'
import * as vscode from 'vscode'
import type { EnvActions } from './actions'
import type { EnvStore } from './store'
import { COLORS, type Environment } from './types'

interface PanelBranch {
  branch: string
  current: boolean
  environments: Environment[]
}

interface PanelFolder {
  name: string
  path: string
  error?: string
  currentBranch: string
  branches: PanelBranch[]
}

// The webview only ever receives keys and metadata. Values never leave the CLI.
export class EnvPanel {
  private static current: EnvPanel | undefined
  private readonly disposables: vscode.Disposable[] = []

  static show (store: EnvStore, actions: EnvActions): void {
    if (EnvPanel.current !== undefined) {
      EnvPanel.current.panel.reveal(vscode.ViewColumn.Active)
      return
    }
    const panel = vscode.window.createWebviewPanel(
      'envHandler.manager',
      'Environments',
      vscode.ViewColumn.Active,
      { enableScripts: true, retainContextWhenHidden: true }
    )
    EnvPanel.current = new EnvPanel(panel, store, actions)
  }

  private constructor (
    private readonly panel: vscode.WebviewPanel,
    private readonly store: EnvStore,
    private readonly actions: EnvActions
  ) {
    const nonce = crypto.randomBytes(16).toString('base64')
    panel.webview.html = renderHtml(panel.webview.cspSource, nonce)
    this.disposables.push(
      panel.webview.onDidReceiveMessage(message => {
        void this.handle(message)
      }),
      store.onDidChange(() => this.post()),
      panel.onDidDispose(() => this.dispose())
    )
  }

  private dispose (): void {
    EnvPanel.current = undefined
    for (const disposable of this.disposables) {
      disposable.dispose()
    }
  }

  private post (): void {
    const folders: PanelFolder[] = this.store.folders().map(state => {
      const path = state.folder.uri.fsPath
      return {
        name: state.folder.name,
        path,
        error: state.error,
        currentBranch: this.store.currentBranch(path) ?? '',
        branches: this.store.branches(path).map(branch => ({
          branch: branch.branch,
          current: branch.branch === this.store.currentBranch(path),
          environments: branch.environments
        }))
      }
    })
    void this.panel.webview.postMessage({ type: 'state', folders })
  }

  private async handle (message: unknown): Promise<void> {
    if (typeof message !== 'object' || message === null) return
    const { action, folder, branch, file, value } = message as Record<string, unknown>

    if (action === 'ready') {
      this.post()
      return
    }
    if (action === 'refresh') {
      await this.store.refresh()
      return
    }
    if (typeof action !== 'string' || typeof folder !== 'string') return
    if (this.store.stateFor(folder) === undefined) return

    if (action === 'create') {
      await this.actions.create(folder)
      return
    }
    if (action === 'import') {
      await this.actions.importPostman(folder)
      return
    }

    const env = this.findEnv(folder, branch, file)
    if (env === undefined) return

    switch (action) {
      case 'activate':
        await this.actions.activate(folder, env)
        return
      case 'rename':
        await this.actions.rename(folder, env)
        return
      case 'renameFile':
        await this.actions.renameFile(folder, env)
        return
      case 'duplicate':
        await this.actions.duplicate(folder, env)
        return
      case 'color':
        if (typeof value === 'string' && (COLORS as readonly string[]).includes(value)) {
          await this.actions.setColor(folder, env, value)
        }
        return
      case 'up':
        await this.actions.move(folder, env, 'up')
        return
      case 'down':
        await this.actions.move(folder, env, 'down')
        return
      case 'delete':
        await this.actions.remove(folder, env)
        return
      default:
        return
    }
  }

  // Look the environment up from the store, so a webview message cannot name arbitrary files.
  private findEnv (folder: string, branch: unknown, file: unknown): Environment | undefined {
    if (typeof branch !== 'string' || typeof file !== 'string') return undefined
    return this.store.branches(folder)
      .find(item => item.branch === branch)
      ?.environments.find(env => env.file === file)
  }
}

function renderHtml (cspSource: string, nonce: string): string {
  const colors = JSON.stringify(COLORS)
  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${cspSource} 'unsafe-inline'; script-src 'nonce-${nonce}';">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Environments</title>
<style>
  body { font-family: var(--vscode-font-family); color: var(--vscode-foreground); padding: 16px; }
  header { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin-bottom: 12px; }
  h1 { font-size: 1.3em; font-weight: 600; margin: 0 12px 0 0; }
  button { background: var(--vscode-button-background); color: var(--vscode-button-foreground); border: none; padding: 4px 10px; cursor: pointer; border-radius: 2px; }
  button:hover { background: var(--vscode-button-hoverBackground); }
  button.secondary { background: var(--vscode-button-secondaryBackground); color: var(--vscode-button-secondaryForeground); }
  select { background: var(--vscode-dropdown-background); color: var(--vscode-dropdown-foreground); border: 1px solid var(--vscode-dropdown-border); padding: 2px; }
  details { margin: 12px 0; border: 1px solid var(--vscode-panel-border); border-radius: 4px; }
  summary { cursor: pointer; padding: 8px 10px; font-weight: 600; }
  .badge { font-weight: 400; color: var(--vscode-descriptionForeground); margin-left: 8px; }
  .body { padding: 0 10px 10px 10px; }
  table { border-collapse: collapse; width: 100%; }
  th, td { text-align: left; padding: 6px 8px; border-bottom: 1px solid var(--vscode-panel-border); vertical-align: middle; }
  th { color: var(--vscode-descriptionForeground); font-weight: 500; }
  .mono { font-family: var(--vscode-editor-font-family); }
  .active { color: var(--vscode-charts-green); white-space: nowrap; }
  .actions { display: flex; gap: 6px; flex-wrap: wrap; }
  .actions button { padding: 2px 8px; }
  .actions button.danger { background: var(--vscode-inputValidation-errorBackground); color: var(--vscode-foreground); }
  #message { color: var(--vscode-descriptionForeground); }
</style>
</head>
<body>
<header>
  <h1>Environments</h1>
  <select id="folder" hidden></select>
  <button id="create">New environment</button>
  <button id="import">Import Postman…</button>
  <button id="refresh" class="secondary">Refresh</button>
</header>
<p id="message"></p>
<div id="groups"></div>
<script nonce="${nonce}">
(function () {
  const vscode = acquireVsCodeApi();
  const colors = ${colors};
  let folders = [];
  let selected = 0;

  const folderSelect = document.getElementById('folder');
  const groups = document.getElementById('groups');
  const message = document.getElementById('message');

  document.getElementById('create').addEventListener('click', function () { send('create'); });
  document.getElementById('import').addEventListener('click', function () { send('import'); });
  document.getElementById('refresh').addEventListener('click', function () { send('refresh'); });
  folderSelect.addEventListener('change', function () {
    selected = Number(folderSelect.value);
    render();
  });

  window.addEventListener('message', function (event) {
    const data = event.data;
    if (data && data.type === 'state') {
      folders = data.folders;
      if (selected >= folders.length) selected = 0;
      render();
    }
  });

  function send (action, extra) {
    const folder = folders[selected];
    const payload = { action: action, folder: folder ? folder.path : '' };
    if (extra) {
      Object.keys(extra).forEach(function (key) { payload[key] = extra[key]; });
    }
    vscode.postMessage(payload);
  }

  function cell (content, className) {
    const td = document.createElement('td');
    td.textContent = content;
    if (className) td.className = className;
    return td;
  }

  function button (label, action, env, extraClass) {
    const b = document.createElement('button');
    b.textContent = label;
    if (extraClass) b.className = extraClass;
    b.addEventListener('click', function () { send(action, { branch: env.branch, file: env.file }); });
    return b;
  }

  function row (env, index, count) {
    const tr = document.createElement('tr');
    tr.appendChild(cell(env.active ? 'Active' : '', 'active'));
    tr.appendChild(cell(env.name));
    tr.appendChild(cell(env.file, 'mono'));

    const encrypted = env.variables.filter(function (v) { return v.encrypted; }).length;
    let keys = env.variables.length + (env.variables.length === 1 ? ' key' : ' keys');
    if (encrypted > 0) keys += ' · ' + encrypted + ' encrypted';
    tr.appendChild(cell(keys));

    const colorTd = document.createElement('td');
    const select = document.createElement('select');
    colors.forEach(function (c) {
      const option = document.createElement('option');
      option.value = c;
      option.textContent = c;
      option.selected = c === env.color;
      select.appendChild(option);
    });
    select.addEventListener('change', function () {
      send('color', { branch: env.branch, file: env.file, value: select.value });
    });
    colorTd.appendChild(select);
    tr.appendChild(colorTd);

    const actions = document.createElement('td');
    const toolbar = document.createElement('div');
    toolbar.className = 'actions';
    if (!env.active) toolbar.appendChild(button('Activate', 'activate', env));
    toolbar.appendChild(button('Rename', 'rename', env));
    toolbar.appendChild(button('Rename file', 'renameFile', env));
    toolbar.appendChild(button('Duplicate', 'duplicate', env));
    if (index > 0) toolbar.appendChild(button('Up', 'up', env));
    if (index < count - 1) toolbar.appendChild(button('Down', 'down', env));
    toolbar.appendChild(button('Delete', 'delete', env, 'danger'));
    actions.appendChild(toolbar);
    tr.appendChild(actions);
    return tr;
  }

  function branchSection (branch) {
    const details = document.createElement('details');
    details.open = branch.current;
    const summary = document.createElement('summary');
    summary.textContent = branch.branch;
    const badge = document.createElement('span');
    badge.className = 'badge';
    badge.textContent = branch.current
      ? 'current branch'
      : branch.environments.length + (branch.environments.length === 1 ? ' env' : ' envs');
    summary.appendChild(badge);
    details.appendChild(summary);

    const body = document.createElement('div');
    body.className = 'body';
    if (branch.environments.length === 0) {
      const empty = document.createElement('p');
      empty.textContent = 'No environments on this branch.';
      body.appendChild(empty);
    } else {
      const table = document.createElement('table');
      const thead = document.createElement('thead');
      const headRow = document.createElement('tr');
      ['Status', 'Name', 'File', 'Keys', 'Color', 'Actions'].forEach(function (label) {
        const th = document.createElement('th');
        th.textContent = label;
        headRow.appendChild(th);
      });
      thead.appendChild(headRow);
      table.appendChild(thead);
      const tbody = document.createElement('tbody');
      branch.environments.forEach(function (env, index) {
        tbody.appendChild(row(env, index, branch.environments.length));
      });
      table.appendChild(tbody);
      body.appendChild(table);
    }
    details.appendChild(body);
    return details;
  }

  function render () {
    folderSelect.textContent = '';
    folders.forEach(function (folder, index) {
      const option = document.createElement('option');
      option.value = String(index);
      option.textContent = folder.name;
      option.selected = index === selected;
      folderSelect.appendChild(option);
    });
    folderSelect.hidden = folders.length < 2;

    groups.textContent = '';
    const folder = folders[selected];
    if (!folder) {
      message.textContent = 'Open a folder to manage its environments.';
      return;
    }
    if (folder.error) {
      message.textContent = folder.error;
      return;
    }
    message.textContent = 'Branch: ' + folder.currentBranch + '. Other branches are listed below; any env can be activated.';
    if (folder.branches.every(function (b) { return b.environments.length === 0; })) {
      message.textContent = 'No environments yet. Create one, or import a Postman environment.';
    }
    folder.branches.forEach(function (branch) {
      groups.appendChild(branchSection(branch));
    });
  }

  vscode.postMessage({ action: 'ready' });
})();
</script>
</body>
</html>`
}
