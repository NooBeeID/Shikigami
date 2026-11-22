<script lang="ts">
  import { onMount } from 'svelte';
  import { layout, type LayoutStore } from '../../lib/stores/layout';
  import { emptyDoc, leaves, findLeaf, pathOf, allTabs, activeTab, type Leaf, type Tab, type Node, type LayoutDoc } from './model';
  import { getPaneKind } from './registry';
  import { ensureHost, getHost, attachHost, detachHost, syncPanes } from './panes/hosts';
  import PaneHeader from './PaneHeader.svelte';
  import DropOverlay from './DropOverlay.svelte';
  import Splitter from './Splitter.svelte';
  import { installLayoutShortcuts, splitPaneWithNewSession } from './shortcuts';

  let { store = layout }: { store?: LayoutStore } = $props();

  let doc = $state<LayoutDoc>(emptyDoc());
  $effect.pre(() =>
    store.subscribe((v) => {
      doc = v;
    }),
  );

  const all = $derived(leaves(doc));
  const indexOf = $derived(new Map(all.map((l, i) => [l.id, i + 1])));
  const maxId = $derived(doc.maximizedId && findLeaf(doc, doc.maximizedId) ? doc.maximizedId : null);
  const maxPath = $derived(maxId ? pathOf(doc, maxId) : null);
  const tabs = $derived(allTabs(doc));
  const visibleTabIds = $derived(
    new Set(all.filter((l) => !maxId || l.id === maxId).map((l) => activeTab(l).id)),
  );

  $effect.pre(() => {
    syncPanes(tabs, visibleTabIds);
  });

  onMount(() => installLayoutShortcuts(store));

  function inMax(path: string): boolean {
    return maxPath === null || maxPath === path || path === '' || maxPath.startsWith(path + '.');
  }

  function hostSlot(node: HTMLElement, p: { tab: Tab; paneId: string; visible: boolean }) {
    let raf = 0;
    function refit() {
      if (raf) cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => {
        raf = requestAnimationFrame(() => {
          raf = 0;
          window.dispatchEvent(new Event('resize'));
        });
      });
    }
    ensureHost(p.tab, p.paneId, p.visible);
    attachHost(p.tab.id, node);
    refit();
    let id = p.tab.id;
    return {
      update(np: { tab: Tab; paneId: string; visible: boolean }) {
        id = np.tab.id;
        ensureHost(np.tab, np.paneId, np.visible);
        attachHost(np.tab.id, node);
        refit();
      },
      destroy() {
        if (raf) cancelAnimationFrame(raf);
        const h = getHost(id);
        if (h && h.parentElement === node) detachHost(id);
      },
    };
  }

  // Tutup pane = akhiri sesinya juga, supaya hilang dari sidebar.
  async function closeAndEnd(pane: Leaf) {
    store.closePane(pane.id);
    for (const tab of pane.tabs) {
      if (tab.kind === 'terminal' && tab.sessionId) await endSession(tab);
    }
  }

  async function endSession(tab: Tab) {
    try {
      await getPaneKind(tab.kind)?.onEndSession?.(tab);
    } catch (err) {
      console.error('[layout] gagal mengakhiri sesi:', err);
    }
  }
</script>

{#snippet renderNode(n: Node, path: string, grow: number)}
  {#if n.type === 'pane'}
    {#key n.id}
      {@const at = activeTab(n)}
      <div
        class="pane"
        class:focused={doc.focusId === n.id}
        data-pane-id={n.id}
        style:flex="{maxPath !== null ? 1 : grow} 1 0"
        style:display={inMax(path) ? null : 'none'}
        onpointerdowncapture={() => {
          if (doc.focusId !== n.id) store.setFocus(n.id);
        }}
      >
        <PaneHeader
          pane={n}
          index={indexOf.get(n.id) ?? 0}
          focused={doc.focusId === n.id}
          maximized={maxId === n.id}
          {store}
          onSplit={(dir) => void splitPaneWithNewSession(n.id, dir, store)}
          onMaximizeToggle={() => (doc.maximizedId === n.id ? store.restore() : store.maximize(n.id))}
          onClose={() => void closeAndEnd(n)}
          onMinimize={() => store.closeTab(n.id, at.id)}
          onRename={(t) => store.renameTab(n.id, at.id, t)}
          onEndSession={() => void endSession(at)}
          onTidy={() => store.tidyGrid()}
        />
        {#key at.id}
          <div class="pane-slot" use:hostSlot={{ tab: at, paneId: n.id, visible: visibleTabIds.has(at.id) }}></div>
        {/key}
      </div>
    {/key}
  {:else}
    <div
      class="split {n.dir}"
      style:flex="{maxPath !== null ? 1 : grow} 1 0"
      style:display={inMax(path) ? 'flex' : 'none'}
    >
      {@render renderNode(n.a, path ? path + '.a' : 'a', n.ratio)}
      {#if maxPath === null}<Splitter dir={n.dir} {path} {store} />{/if}
      {@render renderNode(n.b, path ? path + '.b' : 'b', 1 - n.ratio)}
    </div>
  {/if}
{/snippet}

<div class="layout-area">
  {#if doc.root}
    {@render renderNode(doc.root, '', 1)}
  {:else}
    <div class="empty">Belum ada pane. Buka sesi dari sidebar atau file dari panel file.</div>
  {/if}
  <DropOverlay />
</div>

<style>
  .layout-area {
    position: relative;
    display: flex;
    flex: 1;
    width: 100%;
    height: 100%;
    min-width: 0;
    min-height: 0;
    background: var(--bg);
    overflow: hidden;
  }
  .split { display: flex; min-width: 0; min-height: 0; }
  .split.h { flex-direction: row; }
  .split.v { flex-direction: column; }
  .pane {
    position: relative;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
    background: var(--bg-elev, var(--bg));
  }
  .pane-slot { flex: 1; min-height: 0; min-width: 0; position: relative; }
  .pane.focused::after {
    content: '';
    position: absolute;
    inset: 0;
    border: 1px solid var(--accent, #4c8dff);
    pointer-events: none;
    z-index: 2;
  }
  .empty {
    margin: auto;
    color: var(--fg-muted);
    font-size: 15px;
    text-align: center;
    padding: 16px;
  }
  :global(body.pane-dragging .pane-host) { pointer-events: none; }
  :global(body.pane-dragging), :global(body.pane-resizing) { user-select: none; }
</style>
