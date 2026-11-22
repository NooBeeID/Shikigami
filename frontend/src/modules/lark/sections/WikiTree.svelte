<script lang="ts">
  import { onMount, tick, untrack } from 'svelte';
  import { get } from 'svelte/store';
  import { onEvent } from '../../../lib/ipc';
  import { activeWorkspaceId } from '../../../lib/stores/workspaces';
  import { openSettings } from '../../settings';
  import { listWikiNodes, parseLarkError, type LarkError, type WikiNode } from '../wikiTreeClient';

  type NodeState = { status: 'idle' | 'loading' | 'ready' | 'error'; nodes: WikiNode[]; hasMore: boolean; error?: LarkError };
  type RootState = NodeState & { hidden?: boolean; hiddenReason?: string };

  let {
    source,
    onOpen
  }: {
    source: { id: string; name: string; spaceId?: string; rootToken?: string; host?: string };
    onOpen: (nodeToken: string) => void;
  } = $props();

  const idle = (): NodeState => ({ status: 'idle', nodes: [], hasMore: false });

  let root = $state<RootState>(idle());
  let children = $state<Record<string, NodeState>>({});
  let expanded = $state<Record<string, boolean>>({});
  let focusedKey = $state<string | null>(null);
  let seq = 0;
  let parentOf: Record<string, string | null> = {};
  let container: HTMLDivElement | undefined;

  async function load(parent: string | null, force = false) {
    const ws = get(activeWorkspaceId);
    const target = (): NodeState => (parent === null ? root : children[parent]);
    if (parent !== null && !children[parent]) children[parent] = idle();
    if (!force && target().status === 'ready') return;
    if (!ws) {
      const t = target();
      t.status = 'error';
      t.error = { code: 'no_workspace', message: 'Workspace belum dipilih' };
      return;
    }
    const gen = seq;
    const t0 = target();
    t0.status = 'loading';
    t0.error = undefined;
    try {
      const r = await listWikiNodes(ws, source.id, parent ?? undefined);
      if (gen !== seq) return;
      const t = target();
      if (!t) return;
      t.status = 'ready';
      t.nodes = r.nodes;
      t.hasMore = !!r.hasMore;
      if (parent === null) {
        root.hidden = !!r.hidden;
        root.hiddenReason = r.hiddenReason;
      }
      for (const n of r.nodes) parentOf[n.token] = parent;
    } catch (e) {
      if (gen !== seq) return;
      const t = target();
      if (!t) return;
      t.status = 'error';
      t.error = parseLarkError(e);
    }
  }

  function resetAll() {
    seq++;
    children = {};
    expanded = {};
    focusedKey = null;
    parentOf = {};
    root = idle();
    void load(null, true);
  }

  $effect(() => {
    source.id;
    $activeWorkspaceId;
    untrack(() => resetAll());
  });

  onMount(() => {
    const off = onEvent('workspace.changed', (_t, p) => {
      const pl = p as any;
      if (pl?.kind !== 'configChanged') return;
      const ws = get(activeWorkspaceId);
      if (pl.workspaceId && ws && pl.workspaceId !== ws) return;
      resetAll();
    });
    return off;
  });

  function dropDescendants(token: string) {
    const kids = children[token]?.nodes ?? [];
    for (const k of kids) {
      dropDescendants(k.token);
      delete children[k.token];
      delete expanded[k.token];
    }
  }

  function refreshNode(node: WikiNode) {
    dropDescendants(node.token);
    void load(node.token, true);
  }

  function open(node: WikiNode) {
    expanded[node.token] = true;
    if (!children[node.token] || children[node.token].status === 'idle') void load(node.token);
  }

  function close(node: WikiNode) {
    expanded[node.token] = false;
  }

  function toggle(node: WikiNode) {
    if (expanded[node.token]) close(node);
    else open(node);
  }

  function visibleItems(): WikiNode[] {
    const out: WikiNode[] = [];
    const walk = (nodes: WikiNode[]) => {
      for (const n of nodes) {
        out.push(n);
        if (n.hasChild && expanded[n.token]) walk(children[n.token]?.nodes ?? []);
      }
    };
    walk(root.nodes);
    return out;
  }

  async function focusItem(token: string | undefined | null) {
    if (!token) return;
    focusedKey = token;
    await tick();
    const el = Array.from(container?.querySelectorAll<HTMLElement>('[data-token]') ?? []).find((x) => x.dataset.token === token);
    el?.focus();
  }

  function findNode(token: string): WikiNode | undefined {
    return visibleItems().find((n) => n.token === token);
  }

  function onKey(e: KeyboardEvent, node: WikiNode) {
    if (e.target !== e.currentTarget) return;
    const items = visibleItems();
    const idx = items.findIndex((n) => n.token === node.token);
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        void focusItem(items[idx + 1]?.token);
        break;
      case 'ArrowUp':
        e.preventDefault();
        void focusItem(items[idx - 1]?.token);
        break;
      case 'Home':
        e.preventDefault();
        void focusItem(items[0]?.token);
        break;
      case 'End':
        e.preventDefault();
        void focusItem(items[items.length - 1]?.token);
        break;
      case 'ArrowRight':
        e.preventDefault();
        if (node.hasChild) {
          if (!expanded[node.token]) open(node);
          else void focusItem(children[node.token]?.nodes[0]?.token);
        }
        break;
      case 'ArrowLeft': {
        e.preventDefault();
        if (node.hasChild && expanded[node.token]) {
          close(node);
        } else {
          const p = parentOf[node.token];
          if (p && findNode(p)) void focusItem(p);
        }
        break;
      }
      case 'Enter':
        e.preventDefault();
        onOpen(node.token);
        break;
      case ' ':
        e.preventDefault();
        onOpen(node.token);
        break;
    }
  }

  const tabbable = $derived(focusedKey ?? root.nodes[0]?.token ?? null);

  function needsConnector(err?: LarkError) {
    return err?.code === 'cli_missing' || err?.code === 'not_logged_in';
  }
</script>

{#snippet refreshIcon()}
  <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M13.5 8a5.5 5.5 0 1 1-1.6-3.9" />
    <path d="M13.5 2.5v3h-3" />
  </svg>
{/snippet}

{#snippet typeIcon(objType: string)}
  {#if objType === 'docx' || objType === 'doc'}
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M4 1.5h5l3 3v10H4z" />
      <path d="M9 1.5v3h3M6 8h4M6 10.5h4" />
    </svg>
  {:else if objType === 'sheet'}
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <rect x="2" y="2.5" width="12" height="11" rx="1" />
      <path d="M2 6.5h12M2 10h12M6.5 2.5v11" />
    </svg>
  {:else}
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M9.5 6.5l-3 3" />
      <path d="M7 4.5l1-1a2.5 2.5 0 0 1 3.5 3.5l-1 1M9 11.5l-1 1A2.5 2.5 0 0 1 4.5 9l1-1" />
    </svg>
  {/if}
{/snippet}

{#snippet levelStatus(st: NodeState, level: number, retry: () => void)}
  {#if st.status === 'loading' || st.status === 'idle'}
    <li role="none" class="note" style="--lvl:{level}"><span role="status">Memuat...</span></li>
  {:else if st.status === 'error'}
    <li role="none" class="note err" style="--lvl:{level}">
      <span>{st.error?.message ?? 'Gagal memuat'}</span>
      <button type="button" class="link" onclick={retry}>Coba lagi</button>
    </li>
  {:else if st.nodes.length === 0}
    <li role="none" class="note" style="--lvl:{level}">Belum ada isi</li>
  {/if}
{/snippet}

{#snippet nodeList(nodes: WikiNode[], level: number)}
  {#each nodes as node (node.token)}
    {@const isOpen = node.hasChild && !!expanded[node.token]}
    <li role="none">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div
        class="item"
        role="treeitem"
        aria-level={level}
        aria-expanded={node.hasChild ? isOpen : undefined}
        aria-selected="false"
        tabindex={tabbable === node.token ? 0 : -1}
        data-token={node.token}
        style="--lvl:{level}"
        onfocus={() => (focusedKey = node.token)}
        onclick={() => {
          focusedKey = node.token;
          onOpen(node.token);
        }}
        onkeydown={(e) => onKey(e, node)}
      >
        {#if node.hasChild}
          <button
            type="button"
            class="arrow"
            class:open={isOpen}
            tabindex="-1"
            aria-label={isOpen ? `Ciutkan ${node.title || 'Tanpa judul'}` : `Bentangkan ${node.title || 'Tanpa judul'}`}
            onclick={(e) => {
              e.stopPropagation();
              toggle(node);
            }}
          >
            <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 1.5L7 5L3 8.5" /></svg>
          </button>
        {:else}
          <span class="arrow spacer"></span>
        {/if}
        <span class="ico">{@render typeIcon(node.objType)}</span>
        <span class="label" title={node.title || 'Tanpa judul'}>{node.title || 'Tanpa judul'}</span>
        {#if node.hasChild}
          <button
            type="button"
            class="rbtn"
            aria-label={`Segarkan ${node.title || 'Tanpa judul'}`}
            onclick={(e) => {
              e.stopPropagation();
              refreshNode(node);
            }}
          >
            {@render refreshIcon()}
          </button>
        {/if}
      </div>
      {#if isOpen}
        {@const st = children[node.token] ?? idle()}
        <ul role="group">
          {@render levelStatus(st, level + 1, () => load(node.token, true))}
          {#if st.status === 'ready'}
            {@render nodeList(st.nodes, level + 1)}
            {#if st.hasMore}
              <li role="none" class="note" style="--lvl:{level + 1}">Menampilkan 50 pertama</li>
            {/if}
          {/if}
        </ul>
      {/if}
    </li>
  {/each}
{/snippet}

<div class="wikitree" bind:this={container}>
  <div class="bar">
    <button type="button" class="rbtn always" aria-label="Segarkan pohon" title="Segarkan pohon" onclick={resetAll}>
      {@render refreshIcon()}
    </button>
  </div>

  {#if root.status === 'loading' || root.status === 'idle'}
    <div class="note flat" role="status">Memuat...</div>
  {:else if root.status === 'error'}
    <div class="note flat err">
      <span>{root.error?.message ?? 'Gagal memuat'}</span>
      {#if needsConnector(root.error)}
        <span class="hint">Hubungkan Lark di Pengaturan &gt; Connector</span>
        <button type="button" class="link" onclick={() => openSettings('connectors')}>Buka Pengaturan &gt; Connector</button>
      {/if}
      <button type="button" class="link" onclick={() => load(null, true)}>Coba lagi</button>
    </div>
  {:else if root.hidden}
    <div class="note flat dim">{root.hiddenReason || 'Wiki ini tidak dapat ditampilkan'}</div>
  {:else if root.nodes.length === 0}
    <div class="note flat">Belum ada isi</div>
  {/if}

  {#if root.status === 'ready' && !root.hidden && root.nodes.length > 0}
    <ul role="tree" aria-label={`Pohon wiki ${source.name}`}>
      {@render nodeList(root.nodes, 1)}
    </ul>
    {#if root.hasMore}
      <div class="note flat dim">Menampilkan 50 pertama</div>
    {/if}
  {/if}
</div>

<style>
  .wikitree {
    --indent: 14px;
    font-size: 12px;
    color: var(--fg, inherit);
    padding-left: 18px;
    min-width: 0;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .bar {
    display: flex;
    justify-content: flex-end;
    min-height: 0;
  }
  .item {
    display: flex;
    align-items: center;
    gap: 4px;
    height: 24px;
    padding-right: 4px;
    padding-left: calc(var(--indent) * (var(--lvl) - 1));
    border-radius: 4px;
    cursor: pointer;
    min-width: 0;
    outline: none;
  }
  .item:hover,
  .item:focus-visible {
    background: var(--bg-elev, rgba(127, 127, 127, 0.12));
  }
  .item:focus-visible {
    outline: 2px solid var(--accent, #4c8dff);
    outline-offset: -2px;
  }
  .arrow {
    flex: 0 0 14px;
    width: 14px;
    height: 14px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    padding: 0;
    color: var(--fg-muted, #888);
    cursor: pointer;
  }
  .arrow.spacer {
    cursor: default;
  }
  .arrow svg {
    transition: transform 0.12s ease;
  }
  .arrow.open svg {
    transform: rotate(90deg);
  }
  .ico {
    flex: 0 0 14px;
    display: inline-flex;
    color: var(--fg-muted, #888);
  }
  .label {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .rbtn {
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    background: none;
    border: none;
    padding: 0;
    border-radius: 3px;
    color: var(--fg-muted, #888);
    cursor: pointer;
    opacity: 0;
  }
  .item:hover .rbtn,
  .item:focus-within .rbtn,
  .rbtn:focus-visible,
  .rbtn.always {
    opacity: 1;
  }
  .rbtn:hover {
    background: var(--bg-elev, rgba(127, 127, 127, 0.12));
    color: var(--fg, inherit);
  }
  .rbtn:focus-visible,
  .arrow:focus-visible,
  .link:focus-visible {
    outline: 2px solid var(--accent, #4c8dff);
  }
  .note {
    color: var(--fg-muted, #888);
    padding: 2px 4px 2px calc(var(--indent) * var(--lvl, 1) + 4px);
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
    list-style: none;
  }
  .note.flat {
    padding-left: 4px;
  }
  .note.dim {
    opacity: 0.8;
    font-style: italic;
  }
  .note.err {
    color: var(--fg, inherit);
  }
  .hint {
    color: var(--fg-muted, #888);
  }
  .link {
    background: none;
    border: none;
    padding: 0;
    color: var(--accent, #4c8dff);
    cursor: pointer;
    font-size: inherit;
    text-decoration: underline;
  }
</style>
