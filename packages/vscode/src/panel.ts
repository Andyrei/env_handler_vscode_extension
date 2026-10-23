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

  private findEnv (folder: string, branch: unknown, file: unknown): Environment | undefined {
    if (typeof branch !== 'string' || typeof file !== 'string') return undefined
    return this.store.branches(folder)
      .find(item => item.branch === branch)
      ?.environments.find(env => env.file === file)
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
      case 'addVariable':
        await this.actions.addVariable(folder, env)
        return
      case 'editVariable':
        if (typeof value !== 'string') return
        await this.actions.editVariable(folder, env, value, false)
        return
      case 'deleteVariable':
        if (typeof value !== 'string') return
        await this.actions.deleteVariable(folder, env, value, false)
        return
      default:
        return
    }
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
  button.danger { background: var(--vscode-inputValidation-errorBackground); color: var(--vscode-foreground); }
  button.small { padding: 2px 8px; font-size: 0.9em; }
  select { background: var(--vscode-dropdown-background); color: var(--vscode-dropdown-foreground); border: 1px solid var(--vscode-dropdown-border); padding: 2px; }
  details { margin: 12px 0; border: 1px solid var(--vscode-panel-border); border-radius: 4px; }
  summary { cursor: pointer; padding: 8px 10px; font-weight: 600; display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .badge { font-weight: 400; color: var(--vscode-descriptionForeground); }
  .body { padding: 0 10px 10px 10px; }
  table { border-collapse: collapse; width: 100%; }
  th, td { text-align: left; padding: 6px 8px; border-bottom: 1px solid var(--vscode-panel-border); vertical-align: middle; }
  th { color: var(--vscode-descriptionForeground); font-weight: 500; }
  .mono { font-family: var(--vscode-editor-font-family); }
  .active { color: var(--vscode-charts-green); white-space: nowrap; }
  .actions { display: flex; gap: 6px; flex-wrap: wrap; }
  .color-dot { width: 16px; height: 16px; border-radius: 50%; display: inline-block; cursor: pointer; border: 2px solid transparent; }
  .color-dot:hover { border-color: var(--vscode-focusBorder); }
  .color-picker { display: flex; gap: 4px; padding: 4px 0; }
  .color-picker button { width: 20px; height: 20px; border-radius: 50%; padding: 0; border: 2px solid transparent; }
  .color-picker button:hover { border-color: var(--vscode-focusBorder); }
  .color-picker button.selected { border-color: var(--vscode-foreground); }
  .name-input { background: var(--vscode-input-background); color: var(--vscode-input-foreground); border: 1px solid var(--vscode-input-border); padding: 2px 4px; font: inherit; width: 140px; }
  .key-list { margin: 8px 0 0 0; padding: 0; list-style: none; }
  .key-list li { display: flex; align-items: center; gap: 8px; padding: 4px 0; border-bottom: 1px solid var(--vscode-panel-border); }
  .key-list .key { font-family: var(--vscode-editor-font-family); flex: 1; }
  .key-list .line { color: var(--vscode-descriptionForeground); font-size: 0.85em; }
  .key-list .lock { color: var(--vscode-charts-yellow); }
  #message { color: var(--vscode-descriptionForeground); }
  .empty { color: var(--vscode-descriptionForeground); font-style: italic; padding: 8px 0; }
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

  function colorDot (env) {
    const dot = document.createElement('span');
    dot.className = 'color-dot';
    dot.style.background = env.color;
    dot.title = 'Click to change color';
    dot.addEventListener('click', function (e) {
      e.stopPropagation();
      showColorPicker(dot, env);
    });
    return dot;
  }

  function showColorPicker (anchor, env) {
    // Remove any existing picker
    const existing = document.querySelector('.color-picker');
    if (existing) existing.remove();
    const picker = document.createElement('div');
    picker.className = 'color-picker';
    colors.forEach(function (c) {
      const b = document.createElement('button');
      b.style.background = c;
      if (c === env.color) b.className = 'selected';
      b.title = c;
      b.addEventListener('click', function () {
        send('color', { branch: env.branch, file: env.file, value: c });
        picker.remove();
      });
      picker.appendChild(b);
    });
    anchor.parentNode.insertBefore(picker, anchor.nextSibling);
    // Close on outside click
    setTimeout(function () {
      document.addEventListener('click', function close (e) {
        if (!picker.contains(e.target) && e.target !== anchor) {
          picker.remove();
          document.removeEventListener('click', close);
        }
      });
    }, 0);
  }

  function editableName (env) {
    const span = document.createElement('span');
    span.textContent = env.name;
    span.style.cursor = 'pointer';
    span.title = 'Click to rename';
    span.addEventListener('click', function (e) {
      e.stopPropagation();
      const input = document.createElement('input');
      input.className = 'name-input';
      input.value = env.name;
      span.replaceWith(input);
      input.focus();
      input.select();
      let done = false;
      function finish () {
        if (done) return;
        done = true;
        const val = input.value.trim();
        if (val.length > 0 && val !== env.name) {
          send('rename', { branch: env.branch, file: env.file, value: val });
        } else {
          input.replaceWith(span);
        }
      }
      input.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') { e.preventDefault(); finish(); }
        if (e.key === 'Escape') { done = true; input.replaceWith(span); }
      });
      input.addEventListener('blur', finish);
    });
    return span;
  }

  function button (label, action, env, extraClass, onClick) {
    const b = document.createElement('button');
    b.textContent = label;
    if (extraClass) b.className = 'small ' + extraClass;
    else b.className = 'small';
    b.addEventListener('click', function (e) {
      e.stopPropagation();
      if (onClick) onClick();
      else send(action, { branch: env.branch, file: env.file });
    });
    return b;
  }

  function variableList (env) {
    const section = document.createElement('div');
    const title = document.createElement('div');
    title.style.cssText = 'display:flex;justify-content:space-between;align-items:center;margin:8px 0 4px 0;';
    const label = document.createElement('strong');
    label.textContent = 'Variables';
    title.appendChild(label);
    const addBtn = document.createElement('button');
    addBtn.className = 'small';
    addBtn.textContent = '+ Add';
    addBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      send('addVariable', { branch: env.branch, file: env.file });
    });
    title.appendChild(addBtn);
    section.appendChild(title);

    if (env.variables.length === 0) {
      const empty = document.createElement('p');
      empty.className = 'empty';
      empty.textContent = 'No variables yet.';
      section.appendChild(empty);
      return section;
    }

    const list = document.createElement('ul');
    list.className = 'key-list';
    env.variables.forEach(function (v) {
      const li = document.createElement('li');
      const key = document.createElement('span');
      key.className = 'key';
      key.textContent = v.key;
      li.appendChild(key);

      const line = document.createElement('span');
      line.className = 'line';
      line.textContent = 'line ' + v.line;
      li.appendChild(line);

      if (v.encrypted) {
        const lock = document.createElement('span');
        lock.className = 'lock';
        lock.textContent = 'encrypted';
        li.appendChild(lock);
      }

      const actions = document.createElement('div');
      actions.className = 'actions';
      if (!v.encrypted) {
        const editBtn = document.createElement('button');
        editBtn.className = 'small';
        editBtn.textContent = 'Edit';
        editBtn.addEventListener('click', function (e) {
          e.stopPropagation();
          send('editVariable', { branch: env.branch, file: env.file, value: v.key });
        });
        actions.appendChild(editBtn);
      }
      const delBtn = document.createElement('button');
      delBtn.className = 'small danger';
      delBtn.textContent = 'Delete';
      delBtn.addEventListener('click', function (e) {
        e.stopPropagation();
        send('deleteVariable', { branch: env.branch, file: env.file, value: v.key });
      });
      actions.appendChild(delBtn);
      li.appendChild(actions);
      list.appendChild(li);
    });
    section.appendChild(list);
    return section;
  }

  function row (env, index, count) {
    const tr = document.createElement('tr');
    tr.appendChild(cell(env.active ? 'Active' : '', 'active'));

    const nameTd = document.createElement('td');
    nameTd.appendChild(editableName(env));
    tr.appendChild(nameTd);

    tr.appendChild(cell(env.file, 'mono'));

    const encrypted = env.variables.filter(function (v) { return v.encrypted; }).length;
    let keys = env.variables.length + (env.variables.length === 1 ? ' key' : ' keys');
    if (encrypted > 0) keys += ' · ' + encrypted + ' encrypted';
    tr.appendChild(cell(keys));

    const colorTd = document.createElement('td');
    colorTd.appendChild(colorDot(env));
    tr.appendChild(colorTd);

    const actions = document.createElement('td');
    const toolbar = document.createElement('div');
    toolbar.className = 'actions';
    if (!env.active) toolbar.appendChild(button('Activate', 'activate', env));
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
    const title = document.createElement('span');
    title.textContent = branch.branch;
    summary.appendChild(title);
    const badge = document.createElement('span');
    badge.className = 'badge';
    badge.textContent = branch.current
      ? 'current branch · ' + branch.environments.length + ' env(s)'
      : branch.environments.length + ' env(s)';
    summary.appendChild(badge);
    details.appendChild(summary);

    const body = document.createElement('div');
    body.className = 'body';
    if (branch.environments.length === 0) {
      const empty = document.createElement('p');
      empty.className = 'empty';
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
        const envRow = row(env, index, branch.environments.length);
        tbody.appendChild(envRow);
        // Expandable variable section under each env row
        const varRow = document.createElement('tr');
        const varCell = document.createElement('td');
        varCell.colSpan = 6;
        varCell.appendChild(variableList(env));
        varRow.appendChild(varCell);
        tbody.appendChild(varRow);
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
      return
    }
    const total = folder.branches.reduce(function (n, b) { return n + b.environments.length; }, 0);
    message.textContent = total === 0
      ? 'No environments yet. Create one, or import a Postman environment.'
      : 'Click a name to rename it. Click a color dot to change it.';
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
