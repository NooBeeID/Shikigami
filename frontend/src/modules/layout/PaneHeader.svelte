<script lang="ts">
  import { tick } from 'svelte';
  import { activeTab, type Leaf, type Tab } from './model';
  import { layout, type LayoutStore } from '../../lib/stores/layout';
  import { getPaneKind } from './registry';
  import { startPaneDrag, startTabDrag } from './dnd';
  import TabContextMenu from './TabContextMenu.svelte';
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
    onTidy: () => void;
    // Legacy, diabaikan (agar LayoutArea lama tetap kompilasi selama fase paralel).
    onClose?: () => void;
    onMinimize?: () => void;
    onRename?: (title: string) => void;
    onEndSession?: () => void;
  }
  let {
    pane, index, focused, maximized, store = layout,
    onSplit, onMaximizeToggle, onTidy,
  }: Props = $props();

  let editing = $state(false);
  let draft = $state('');
  let menuOpen = $state(false);
  let menuBtn: HTMLElement | undefined = $state();
  let inputEl: HTMLInputElement | undefined = $state();
  let tabsEl: HTMLElement | undefined = $state();
  let committed = false;
  let ctxMenu = $state<{ tabId: string; x: number; y: number } | null>(null);
  const ctxTab = $derived(ctxMenu ? pane.tabs.find((x) => x.id === ctxMenu!.tabId) : undefined);
  $effect(() => {
    if (ctxMenu && !ctxTab) ctxMenu = null;
  });

  const active = $derived(activeTab(pane));
  const spec = $derived(getPaneKind(active.kind));
  const badge = $derived(spec?.badgeFor?.(active) ?? null);
  const session = $derived(active.sessionId ? $sessions.find((s) => s.id === active.sessionId) : undefined);
  const isTerminal = $derived(active.kind === 'terminal' && !!active.sessionId);
  const canEnd = $derived(isTerminal && !!spec?.onEndSession && session?.status !== 'exited');

  function isTermTab(t: Tab): boolean {
    return t.kind === 'terminal' && !!t.sessionId;
  }

  function relativeTo(root: string, abs: string): string {
    if (abs === root) return '';
    if (abs.startsWith(root + '/')) return abs.slice(root.length + 1);
    return '';
  }

  const chip = $derived.by(() => {
    const project = $activeWorkspace?.projects?.find((p) => p.id === active.projectId);
    if (!project) return null;
    let abs = project.path;
    if (active.kind === 'terminal') {
      if (session?.cwd) abs = session.cwd;
    } else if (active.filePath) {
      const i = active.filePath.lastIndexOf('/');
      abs = i > 0 ? active.filePath.slice(0, i) : active.filePath;
    }
    const rel = relativeTo(project.path, abs);
    return { text: rel ? `${project.name}/${rel}` : project.name, tip: abs };
  });

  async function endTabSession(t: Tab) {
    try {
      await getPaneKind(t.kind)?.onEndSession?.(t);
    } catch (err) {
      console.error('[layout] gagal mengakhiri sesi tab', err);
    }
  }

  function selectTab(t: Tab) {
    store.activateTab(pane.id, t.id);
  }

  function closeTabAction(t: Tab) {
    store.closeTab(pane.id, t.id);
    if (isTermTab(t)) void endTabSession(t);
  }

  function minimizeTab(t: Tab) {
    if (!isTermTab(t)) return;
    store.closeTab(pane.id, t.id);
  }

  function endActiveSession() {
    const t = active;
    store.closeTab(pane.id, t.id);
    void endTabSession(t);
  }

  function closeWholePane() {
    const tabs = [...pane.tabs];
    store.closePane(pane.id);
    for (const t of tabs) if (isTermTab(t)) void endTabSession(t);
  }

  async function startRename() {
    draft = active.title;
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
    if (t && t !== active.title) store.renameTab(pane.id, active.id, t);
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

  async function onTabsKey(e: KeyboardEvent) {
    if (editing) return;
    const tabs = pane.tabs;
    const cur = Math.max(0, tabs.findIndex((t) => t.id === active.id));
    let target = -1;
    if (e.key === 'ArrowRight') target = (cur + 1) % tabs.length;
    else if (e.key === 'ArrowLeft') target = (cur - 1 + tabs.length) % tabs.length;
    else if (e.key === 'Home') target = 0;
    else if (e.key === 'End') target = tabs.length - 1;
    else if (e.key === 'Delete') {
      e.preventDefault();
      const id = (e.target as HTMLElement).closest<HTMLElement>('[role=tab]')?.dataset.tabId;
      closeTabAction(tabs.find((t) => t.id === id) ?? active);
      return;
    } else return;
    e.preventDefault();
    selectTab(tabs[target]);
    await tick();
    tabsEl?.querySelector<HTMLElement>('[role=tab][aria-selected=true]')?.focus();
  }

  function onTabsWheel(e: WheelEvent) {
    if (!tabsEl || e.deltaX !== 0) return;
    if (tabsEl.scrollWidth <= tabsEl.clientWidth) return;
    e.preventDefault();
    tabsEl.scrollLeft += e.deltaY;
  }

  $effect(() => {
    void active.id;
    void pane.tabs.length;
    tick().then(() => {
      tabsEl?.querySelector<HTMLElement>('[role=tab][aria-selected=true]')
        ?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    });
  });

  function onTabDown(e: PointerEvent, t: Tab) {
    if (e.button !== 0 || editing) return;
    if ((e.target as HTMLElement).closest('button, input')) return;
    e.stopPropagation();
    startTabDrag(e, pane.id, t.id, t.title, store);
  }

  function onTabContext(e: MouseEvent, t: Tab) {
    e.preventDefault();
    e.stopPropagation();
    ctxMenu = { tabId: t.id, x: e.clientX, y: e.clientY };
  }

  function onHeaderDown(e: PointerEvent) {
    if (e.button !== 0 || editing) return;
    if ((e.target as HTMLElement).closest('button, input, .pane-menu, [role=tab]')) return;
    startPaneDrag(e, pane.id, active.title, store);
  }

  function onHeaderDbl(e: MouseEvent) {
    if ((e.target as HTMLElement).closest('button, input, [role=tab]')) return;
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
  <!-- svelte-ignore a11y_interactive_supports_focus -->
  <div
    class="tabs"
    class:single={pane.tabs.length === 1}
    role="tablist"
    data-tab-strip
    tabindex="-1"
    aria-label="Tab pane {index}"
    bind:this={tabsEl}
    onkeydown={onTabsKey}
    onwheel={onTabsWheel}
  >
    {#each pane.tabs as t (t.id)}
      <div
        class="tab"
        class:active={t.id === active.id}
        role="tab"
        aria-selected={t.id === active.id}
        tabindex={t.id === active.id ? 0 : -1}
        data-tab-id={t.id}
        title={t.title}
        onpointerdown={(e) => onTabDown(e, t)}
        oncontextmenu={(e) => onTabContext(e, t)}
        onclick={() => selectTab(t)}
        onkeydown={(e) => { if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); selectTab(t); } }}
      >
        {#if isTermTab(t)}<StatusBadge sessionId={t.sessionId!} compact />{/if}
        {#if editing && t.id === active.id}
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
            ondblclick={(e) => { if (t.id === active.id) { e.stopPropagation(); startRename(); } }}>{t.title}</span>
        {/if}
        {#if isTermTab(t)}
          <button type="button" class="tbtn" aria-label="Minimize tab" title="Minimize (sesi tetap jalan)" tabindex="-1"
                  onclick={(e) => { e.stopPropagation(); minimizeTab(t); }}>
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><path d="M3 11h8"/></svg>
          </button>
        {/if}
        <button type="button" class="tbtn close" aria-label="Tutup tab {t.title}" title="Tutup tab" tabindex="-1"
                onclick={(e) => { e.stopPropagation(); closeTabAction(t); }}>
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><path d="M3 3l8 8M11 3l-8 8"/></svg>
        </button>
      </div>
    {/each}
  </div>
  {#if badge}<span class="badge">{badge}</span>{/if}
  {#if chip}<span class="chip" title={chip.tip}>{chip.text}</span>{/if}
  <span class="spacer"></span>
  {#if isTerminal}
    <button type="button" class="hbtn" aria-label="Bagi kanan" title="Bagi kanan" onclick={() => onSplit('h')}>
      <svg width="18" height="18" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="1.5" y="2" width="11" height="10" rx="1"/><path d="M7 2v10"/></svg>
    </button>
    <button type="button" class="hbtn" aria-label="Bagi bawah" title="Bagi bawah" onclick={() => onSplit('v')}>
      <svg width="18" height="18" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="1.5" y="2" width="11" height="10" rx="1"/><path d="M1.5 7h11"/></svg>
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
      <svg width="18" height="18" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="4" y="1.5" width="8.5" height="8.5" rx="1"/><path d="M1.5 4.5v8h8"/></svg>
    {:else}
      <svg width="18" height="18" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="2" y="2" width="10" height="10" rx="1"/></svg>
    {/if}
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
    <svg width="18" height="18" viewBox="0 0 14 14" fill="currentColor"><circle cx="3" cy="7" r="1.1"/><circle cx="7" cy="7" r="1.1"/><circle cx="11" cy="7" r="1.1"/></svg>
  </button>
  {#if menuOpen && menuBtn}
    <PaneMenu
      anchor={menuBtn}
      canEndSession={canEnd}
      onEndSession={endActiveSession}
      onRename={startRename}
      {onTidy}
      onClosePane={closeWholePane}
      onDismiss={() => (menuOpen = false)}
    />
  {/if}
  {#if ctxMenu && ctxTab}
    <TabContextMenu
      paneId={pane.id}
      tabId={ctxTab.id}
      x={ctxMenu.x}
      y={ctxMenu.y}
      onClose={() => (ctxMenu = null)}
      onCloseTab={() => closeTabAction(ctxTab)}
      onMinimizeTab={isTermTab(ctxTab) ? () => minimizeTab(ctxTab) : undefined}
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
    font-size: 14px;
    cursor: grab;
    user-select: none;
    min-width: 0;
    flex: none;
    touch-action: none;
  }
  .pane-header.focused { border-bottom-color: var(--accent, #4f8cff); }
  .pane-header.focused .tab.active .title { font-weight: 600; }
  .num {
    min-width: 16px;
    height: 16px;
    padding: 0 4px;
    box-sizing: border-box;
    border-radius: 8px;
    background: var(--bg-hover, #333);
    font-size: 12px;
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
    font-size: 12px;
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
    font-size: 13px;
    color: var(--fg-muted, #999);
    background: var(--bg-hover, #333);
    border-radius: 3px;
    padding: 0 6px;
    min-width: 0;
  }
  .spacer { flex: 1; }
  .hbtn {
    width: 28px;
    height: 28px;
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
  .tabs {
    display: flex;
    align-items: center;
    gap: 2px;
    flex: 0 1 auto;
    min-width: 0;
    overflow-x: auto;
    overflow-y: hidden;
    scrollbar-width: none;
  }
  .tabs::-webkit-scrollbar { display: none; }
  .tab {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 26px;
    padding: 0 4px 0 8px;
    box-sizing: border-box;
    max-width: 200px;
    min-width: 60px;
    flex: none;
    border-radius: 4px;
    cursor: pointer;
    color: var(--fg-muted, #999);
  }
  .tab:hover { background: var(--bg-hover, #333); }
  .tab.active { background: var(--bg-hover, #333); color: var(--fg, #ddd); }
  .tab:focus-visible { outline: 1px solid var(--accent, #4f8cff); }
  .tabs.single .tab { background: transparent; padding-left: 0; }
  .tbtn {
    width: 18px;
    height: 18px;
    flex: none;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 0;
    border-radius: 3px;
    color: inherit;
    cursor: pointer;
    padding: 0;
    opacity: 0;
  }
  .tab:hover .tbtn, .tab.active .tbtn, .tab:focus-within .tbtn { opacity: 1; }
  .tbtn:hover { background: var(--bg, #111); }
  .tbtn.close:hover { color: var(--danger, #e5534b); }
</style>
