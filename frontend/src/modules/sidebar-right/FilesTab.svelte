<script lang="ts">
  import { activeWorkspace, activeProjectId, selectProject, addProject, type Project } from '../../lib/stores';
  import { openFilePane, openFolder, basename } from '../shell';
  import { TreeModel, type TreeRow } from '../files/tree/treeModel.svelte';
  import { errMessage } from '../workspace/errors';

  const model = new TreeModel();
  let projects = $state<Project[]>([]);
  let hasWorkspace = $state(false);
  let workspaceId = '';
  let activeId = $state<string | null>(null);
  let focusedKey = $state<string | null>(null);
  let treeEl: HTMLDivElement | undefined = $state();
  let lastSig: string | null = null;

  let pathInput = $state('');
  let busy = $state(false);
  let error = $state('');

  let pendingPick = false;
  let pendingTimer: ReturnType<typeof setTimeout> | undefined;
  let knownIds = new Set<string>();

  $effect(() => {
    const unsub = activeWorkspace.subscribe((ws) => {
      const newWsId = ws?.id ?? '';
      if (newWsId !== workspaceId) {
        pendingPick = false;
        clearTimeout(pendingTimer);
        knownIds = new Set((ws?.projects ?? []).map((p) => p.id));
      }
      workspaceId = newWsId;
      hasWorkspace = ws !== null;
      const list = ws?.projects ?? [];
      if (pendingPick) {
        const fresh = list.filter((p) => !knownIds.has(p.id));
        if (fresh.length > 0) {
          pendingPick = false;
          clearTimeout(pendingTimer);
          selectProject(fresh[fresh.length - 1].id);
        }
      }
      knownIds = new Set(list.map((p) => p.id));
      projects = list;
    });
    return () => {
      unsub();
      clearTimeout(pendingTimer);
    };
  });

  $effect(() => {
    const unsub = activeProjectId.subscribe((id) => (activeId = id));
    return unsub;
  });

  const selected = $derived(projects.find((p) => p.id === activeId) ?? projects[0] ?? null);

  $effect(() => {
    const sel = selected;
    const sig = sel ? sel.id + '|' + sel.path : '';
    if (sig === lastSig) return;
    lastSig = sig;
    focusedKey = null;
    model.setProjects(sel ? [sel] : []);
  });

  const nodeRows = $derived(model.rows.filter((r) => !r.placeholder));

  function openFile(row: TreeRow) {
    if (!selected) return;
    openFilePane({ projectId: selected.id, filePath: row.path, workspaceId, title: basename(row.path) });
  }

  function activate(row: TreeRow) {
    if (row.kind === 'project' || row.kind === 'dir') model.toggle(row);
    else openFile(row);
  }

  function onRowClick(row: TreeRow) {
    if (row.placeholder) return;
    focusedKey = row.key;
    activate(row);
  }

  function parentOf(row: TreeRow): TreeRow | undefined {
    const i = nodeRows.findIndex((r) => r.key === row.key);
    for (let j = i - 1; j >= 0; j--) if (nodeRows[j].depth < row.depth) return nodeRows[j];
    return undefined;
  }

  function onKeydown(e: KeyboardEvent) {
    if (nodeRows.length === 0) return;
    const idx = nodeRows.findIndex((r) => r.key === focusedKey);
    const cur = idx >= 0 ? nodeRows[idx] : undefined;
    const isDir = cur && (cur.kind === 'project' || cur.kind === 'dir');
    let handled = true;
    switch (e.key) {
      case 'ArrowDown':
        focusedKey = nodeRows[Math.min(idx + 1, nodeRows.length - 1)].key;
        break;
      case 'ArrowUp':
        focusedKey = nodeRows[Math.max(idx - 1, 0)].key;
        break;
      case 'Home':
        focusedKey = nodeRows[0].key;
        break;
      case 'End':
        focusedKey = nodeRows[nodeRows.length - 1].key;
        break;
      case 'ArrowRight':
        if (!cur) break;
        if (isDir && !cur.expanded) model.toggle(cur);
        else if (isDir && cur.expanded && nodeRows[idx + 1]?.depth > cur.depth) focusedKey = nodeRows[idx + 1].key;
        break;
      case 'ArrowLeft':
        if (!cur) break;
        if (isDir && cur.expanded) model.toggle(cur);
        else {
          const p = parentOf(cur);
          if (p) focusedKey = p.key;
        }
        break;
      case 'Enter':
      case ' ':
        if (cur) activate(cur);
        break;
      default:
        handled = false;
    }
    if (handled) {
      e.preventDefault();
      if (!cur && focusedKey === null) focusedKey = nodeRows[0].key;
    }
  }

  $effect(() => {
    const key = focusedKey;
    if (key === null || !treeEl) return;
    const el = [...treeEl.querySelectorAll<HTMLElement>('[data-key]')].find((n) => n.dataset.key === key && !n.dataset.placeholder);
    el?.scrollIntoView({ block: 'nearest' });
  });

  function onOpenFolder() {
    pendingPick = true;
    clearTimeout(pendingTimer);
    pendingTimer = setTimeout(() => (pendingPick = false), 60000);
    openFolder();
  }

  async function onAddPath(e: SubmitEvent) {
    e.preventDefault();
    const path = pathInput.trim();
    if (!path) return;
    busy = true;
    error = '';
    try {
      const p = await addProject(path);
      selectProject(p.id);
      pathInput = '';
    } catch (err) {
      error = errMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<div class="files-tab">
  {#if hasWorkspace && projects.length > 0}
    <div class="header">
      <select
        aria-label="Pilih project"
        value={selected?.id ?? ''}
        onchange={(e) => selectProject(e.currentTarget.value)}
      >
        {#each projects as p (p.id)}
          <option value={p.id} title={p.path}>{p.name}</option>
        {/each}
      </select>
      <button class="icon" aria-label="Segarkan" title="Segarkan" onclick={() => model.refreshAll()}>&#8635;</button>
    </div>
  {/if}

  {#if !hasWorkspace}
    <div class="msg">Tidak ada workspace aktif.</div>
  {:else if projects.length === 0}
    <div class="empty">
      <div class="empty-title">Belum ada project</div>
      <div class="msg">Buka folder project untuk mulai menjelajah file.</div>
      <button class="primary" onclick={onOpenFolder}>Open Folder…</button>
      <form class="path-form" onsubmit={onAddPath}>
        <input
          type="text"
          placeholder="/path/ke/project"
          aria-label="Path project"
          bind:value={pathInput}
          disabled={busy}
        />
        <button type="submit" disabled={busy}>Tambah</button>
      </form>
      {#if error}<div class="err-inline">{error}</div>{/if}
    </div>
  {:else}
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <div class="tree" role="tree" tabindex="0" bind:this={treeEl} onkeydown={onKeydown}>
      {#each model.rows as row (row.key + ':' + (row.placeholder ?? ''))}
        {#if row.placeholder}
          <div
            class="row placeholder"
            role="none"
            data-key={row.key}
            data-placeholder="1"
            style:padding-left="{row.depth * 12 + 8}px"
          >
            {#if row.placeholder === 'loading'}
              Memuat...
            {:else if row.placeholder === 'empty'}
              Folder kosong
            {:else}
              <span class="err">{row.error ?? 'Gagal memuat folder.'}</span>
              <button
                class="retry"
                onclick={(e) => {
                  e.stopPropagation();
                  const parent = model.rows.find((r) => r.key === row.key && !r.placeholder);
                  if (parent) model.retry(parent);
                }}>Coba lagi</button
              >
            {/if}
          </div>
        {:else}
          {@const isDir = row.kind === 'project' || row.kind === 'dir'}
          <div
            class="row"
            class:focused={focusedKey === row.key}
            class:project={row.kind === 'project'}
            role="treeitem"
            tabindex="-1"
            aria-level={row.depth + 1}
            aria-selected={focusedKey === row.key}
            aria-expanded={isDir ? row.expanded : undefined}
            data-key={row.key}
            title={row.path}
            style:padding-left="{row.depth * 12 + 8}px"
            onclick={() => onRowClick(row)}
            onkeydown={() => {}}
          >
            <span class="arrow">{isDir ? (row.expanded ? '▾' : '▸') : row.kind === 'symlink' ? '↗' : '·'}</span>
            <span class="name">{row.name}</span>
            {#if isDir && row.expanded}
              <button
                class="icon refresh"
                aria-label="Segarkan folder"
                title="Segarkan folder"
                onclick={(e) => {
                  e.stopPropagation();
                  model.refreshDir(row);
                }}>&#8635;</button
              >
            {/if}
          </div>
        {/if}
      {/each}
    </div>
  {/if}

  {#if hasWorkspace && projects.length > 0}
    <div class="footer">
      <button class="primary" onclick={onOpenFolder}>Open Folder…</button>
    </div>
  {/if}
</div>

<style>
  .files-tab {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    font-size: 12px;
    color: var(--fg, inherit);
  }
  .header {
    flex: none;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent));
  }
  .header select {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .footer {
    flex: none;
    padding: 10px 12px;
    border-top: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent));
  }
  .primary {
    width: 100%;
  }
  .icon {
    flex: none;
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    padding: 0 4px;
    font-size: 13px;
    line-height: 1;
  }
  .icon:hover {
    background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent));
  }
  .msg {
    padding: 12px;
    color: var(--fg-muted, inherit);
  }
  .empty {
    flex: 1;
    min-height: 0;
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px 16px;
  }
  .empty .msg {
    padding: 0;
  }
  .empty-title {
    font-weight: 600;
  }
  .path-form {
    display: flex;
    gap: 4px;
    min-width: 0;
  }
  .path-form input {
    flex: 1;
    min-width: 0;
  }
  .path-form button {
    flex: none;
  }
  .err-inline {
    color: var(--danger, inherit);
    overflow-wrap: anywhere;
  }
  .tree {
    flex: 1;
    min-height: 0;
    overflow: auto;
    outline: none;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 4px;
    height: 28px;
    box-sizing: border-box;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    cursor: pointer;
    content-visibility: auto;
    contain-intrinsic-size: auto 22px;
  }
  .row:hover,
  .row.focused {
    background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent));
  }
  .tree:focus-visible .row.focused {
    outline: 1px solid var(--accent, currentColor);
    outline-offset: -1px;
  }
  .row.project .name {
    font-weight: 600;
  }
  .arrow {
    width: 10px;
    flex: none;
    text-align: center;
    color: var(--fg-muted, inherit);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
    min-width: 0;
  }
  .refresh {
    display: none;
  }
  .row:hover .refresh,
  .row.focused .refresh {
    display: inline;
  }
  .placeholder {
    cursor: default;
    color: var(--fg-muted, inherit);
  }
  .placeholder:hover {
    background: none;
  }
  .err {
    color: var(--danger, inherit);
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .retry {
    flex: none;
    font-size: 11px;
    cursor: pointer;
  }
</style>
