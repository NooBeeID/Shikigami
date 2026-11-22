<script lang="ts">
  import { tick } from 'svelte';
  import type { Leaf } from './model';
  import { layout, type LayoutStore } from '../../lib/stores/layout';
  import { getPaneKind } from './registry';
  import { startPaneDrag } from './dnd';
  import PaneMenu from './PaneMenu.svelte';
  import StatusBadge from '../status/StatusBadge.svelte';
  import { activeWorkspace } from '../../lib/stores/workspaces';
  import { sessions } from '../../lib/stores/sessions';

  interface Props {
    pane: Leaf;
    index: number;
    focused: boolean;
    maximized: boolean;
    store?: LayoutStore;
    onSplit: (dir: 'h' | 'v') => void;
    onMaximizeToggle: () => void;
    onClose: () => void;
    onMinimize: () => void;
    onRename: (title: string) => void;
    onEndSession: () => void;
    onTidy: () => void;
  }
  let {
    pane, index, focused, maximized, store = layout,
    onSplit, onMaximizeToggle, onClose, onMinimize, onRename, onEndSession, onTidy,
  }: Props = $props();

  let editing = $state(false);
  let draft = $state('');
  let menuOpen = $state(false);
  let menuBtn: HTMLElement | undefined = $state();
  let inputEl: HTMLInputElement | undefined = $state();
  let committed = false;

  const spec = $derived(getPaneKind(pane.kind));
  const badge = $derived(spec?.badgeFor?.(pane) ?? null);
  const session = $derived(pane.sessionId ? $sessions.find((s) => s.id === pane.sessionId) : undefined);
  const isTerminal = $derived(pane.kind === 'terminal' && !!pane.sessionId);
  const canEnd = $derived(isTerminal && !!spec?.onEndSession && session?.status !== 'exited');

  function relativeTo(root: string, abs: string): string {
    if (abs === root) return '';
    if (abs.startsWith(root + '/')) return abs.slice(root.length + 1);
    return '';
  }

  const chip = $derived.by(() => {
    const project = $activeWorkspace?.projects?.find((p) => p.id === pane.projectId);
    if (!project) return null;
    let abs = project.path;
    if (pane.kind === 'terminal') {
      if (session?.cwd) abs = session.cwd;
    } else if (pane.filePath) {
      const i = pane.filePath.lastIndexOf('/');
      abs = i > 0 ? pane.filePath.slice(0, i) : pane.filePath;
    }
    const rel = relativeTo(project.path, abs);
    return { text: rel ? `${project.name}/${rel}` : project.name, tip: abs };
  });

  async function startRename() {
    draft = pane.title;
    committed = false;
    editing = true;
    await tick();
    inputEl?.focus();
    inputEl?.select();
  }

  function commitRename() {
    if (committed) return;
    committed = true;
    const t = draft.trim();
    editing = false;
    if (t && t !== pane.title) onRename(t);
  }

  function onInputKey(e: KeyboardEvent) {
    e.stopPropagation();
    if (e.key === 'Enter') {
      e.preventDefault();
      commitRename();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      committed = true;
      editing = false;
    }
  }

  function onHeaderDown(e: PointerEvent) {
    if (e.button !== 0 || editing) return;
    if ((e.target as HTMLElement).closest('button, input, .pane-menu')) return;
    startPaneDrag(e, pane.id, pane.title, store);
  }

  function onHeaderDbl(e: MouseEvent) {
    if ((e.target as HTMLElement).closest('button, input')) return;
    onMaximizeToggle();
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  class="pane-header"
  class:focused
  data-pane-header
  onpointerdown={onHeaderDown}
  ondblclick={onHeaderDbl}
  role="toolbar"
  tabindex="-1"
  aria-label="Header pane {index}"
>
  <span class="num">{index}</span>
  {#if editing}
    <input
      class="rename"
      bind:this={inputEl}
      bind:value={draft}
      onkeydown={onInputKey}
      onblur={commitRename}
      onpointerdown={(e) => e.stopPropagation()}
      ondblclick={(e) => e.stopPropagation()}
    />
  {:else}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <span
      class="title"
      title={pane.title}
      ondblclick={(e) => { e.stopPropagation(); startRename(); }}>{pane.title}</span>
  {/if}
  {#if badge}<span class="badge">{badge}</span>{/if}
  {#if isTerminal}<StatusBadge sessionId={pane.sessionId!} compact />{/if}
  {#if chip}<span class="chip" title={chip.tip}>{chip.text}</span>{/if}
  <span class="spacer"></span>
  <button type="button" class="hbtn" aria-label="Bagi kanan" title="Bagi kanan" onclick={() => onSplit('h')}>
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="1.5" y="2" width="11" height="10" rx="1"/><path d="M7 2v10"/></svg>
  </button>
  <button type="button" class="hbtn" aria-label="Bagi bawah" title="Bagi bawah" onclick={() => onSplit('v')}>
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="1.5" y="2" width="11" height="10" rx="1"/><path d="M1.5 7h11"/></svg>
  </button>
  {#if isTerminal}
    <button type="button" class="hbtn" aria-label="Minimize" title="Minimize (sesi tetap jalan)" onclick={onMinimize}>
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><path d="M3 11h8"/></svg>
    </button>
  {/if}
  <button
    type="button"
    class="hbtn"
    aria-label={maximized ? 'Pulihkan' : 'Maksimalkan'}
    title={maximized ? 'Pulihkan' : 'Maksimalkan'}
    onclick={onMaximizeToggle}
  >
    {#if maximized}
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="4" y="1.5" width="8.5" height="8.5" rx="1"/><path d="M1.5 4.5v8h8"/></svg>
    {:else}
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="2" y="2" width="10" height="10" rx="1"/></svg>
    {/if}
  </button>
  <button type="button" class="hbtn close" aria-label="Tutup pane" title="Tutup pane" onclick={onClose}>
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><path d="M3 3l8 8M11 3l-8 8"/></svg>
  </button>
  <button
    type="button"
    class="hbtn"
    bind:this={menuBtn}
    aria-label="Menu"
    title="Menu"
    aria-haspopup="menu"
    aria-expanded={menuOpen}
    onclick={() => (menuOpen = !menuOpen)}
  >
    <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor"><circle cx="3" cy="7" r="1.1"/><circle cx="7" cy="7" r="1.1"/><circle cx="11" cy="7" r="1.1"/></svg>
  </button>
  {#if menuOpen && menuBtn}
    <PaneMenu
      anchor={menuBtn}
      canEndSession={canEnd}
      {onEndSession}
      onRename={startRename}
      {onTidy}
      {onClose}
      onDismiss={() => (menuOpen = false)}
    />
  {/if}
</div>

<style>
  .pane-header {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 34px;
    box-sizing: border-box;
    padding: 0 10px;
    background: var(--bg-elev, #222);
    border-bottom: 1px solid var(--border, #444);
    color: var(--fg, #ddd);
    font-size: 12px;
    cursor: grab;
    user-select: none;
    min-width: 0;
    flex: none;
    touch-action: none;
  }
  .pane-header.focused { border-bottom-color: var(--accent, #4f8cff); }
  .pane-header.focused .title { font-weight: 600; }
  .num {
    min-width: 16px;
    height: 16px;
    padding: 0 4px;
    box-sizing: border-box;
    border-radius: 8px;
    background: var(--bg-hover, #333);
    font-size: 10px;
    line-height: 16px;
    text-align: center;
  }
  .pane-header.focused .num { font-weight: 700; }
  .title {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .rename {
    min-width: 0;
    width: 160px;
    font: inherit;
    color: var(--fg, #ddd);
    background: var(--bg, #111);
    border: 1px solid var(--accent, #4f8cff);
    border-radius: 3px;
    padding: 1px 4px;
    outline: none;
  }
  .badge {
    font-size: 10px;
    padding: 0 4px;
    border: 1px solid var(--border, #444);
    border-radius: 3px;
    color: var(--fg-muted, #999);
    flex: none;
  }
  .chip {
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 11px;
    color: var(--fg-muted, #999);
    background: var(--bg-hover, #333);
    border-radius: 3px;
    padding: 0 6px;
    min-width: 0;
  }
  .spacer { flex: 1; }
  .hbtn {
    width: 22px;
    height: 22px;
    flex: none;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 0;
    border-radius: 4px;
    color: var(--fg-muted, #999);
    cursor: pointer;
    padding: 0;
  }
  .hbtn:hover { background: var(--bg-hover, #333); color: var(--fg, #ddd); }
  .hbtn.close:hover { color: var(--danger, #e5534b); }
</style>
