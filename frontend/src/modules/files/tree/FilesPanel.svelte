<script lang="ts">
  import { activeWorkspace, openFileTab } from '../shared/shellBridge';
  import { TreeModel, type TreeRow } from './treeModel.svelte';

  const model = new TreeModel();
  let hasWorkspace = $state(false);
  let workspaceId = '';
  let lastSig: string | null = null;
  let focusedKey = $state<string | null>(null);
  let treeEl: HTMLDivElement | undefined = $state();

  $effect(() => {
    const unsub = activeWorkspace.subscribe((ws) => {
      const sig = ws ? ws.id + '|' + ws.projects.map((p) => p.id + ':' + p.path).join(',') : '';
      hasWorkspace = ws !== null;
      workspaceId = ws?.id ?? '';
      if (sig === lastSig) return;
      lastSig = sig;
      focusedKey = null;
      model.setProjects(ws?.projects ?? []);
    });
    return unsub;
  });

  const nodeRows = $derived(model.rows.filter((r) => !r.placeholder));

  function openFile(row: TreeRow) {
    const project = model.projects.find((p) => p.path === row.root);
    openFileTab({ path: row.path, projectId: project?.id ?? '', workspaceId });
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
</script>

<div class="files">
  <div class="header">
    <span class="title">Files</span>
    <button class="icon" aria-label="Segarkan" title="Segarkan" onclick={() => model.refreshAll()}>&#8635;</button>
  </div>
  {#if !hasWorkspace}
    <div class="msg">Tidak ada workspace aktif.</div>
  {:else if model.projects.length === 0}
    <div class="msg">Workspace ini belum punya project.</div>
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
            <span class="arrow">{isDir ? (row.expanded ? '▾' : '▸') : ''}</span>
            <svg class="ico" class:dir={isDir} width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              {#if isDir && row.expanded}
                <path d="M3 8a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v1H3z" />
                <path d="M3 11h19l-2 7a2 2 0 0 1-2 1.5H5a2 2 0 0 1-2-2z" />
              {:else if isDir}
                <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
              {:else if row.kind === 'symlink'}
                <path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1" />
                <path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" />
              {:else}
                <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
                <path d="M14 3v5h5" />
              {/if}
            </svg>
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
</div>

<style>
  .files {
    height: 100%;
    display: flex;
    flex-direction: column;
    font-size: 12px;
    color: var(--fg, inherit);
  }
  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    flex: none;
  }
  .title {
    font-weight: 600;
    text-transform: uppercase;
    font-size: 11px;
    color: var(--fg-muted, inherit);
  }
  .icon {
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
  .tree {
    flex: 1;
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
  .ico {
    flex: none;
    color: var(--fg-muted, inherit);
  }
  .ico.dir {
    color: var(--accent, #3b82f6);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
    min-width: 0;
  }
  .refresh {
    display: none;
    flex: none;
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
