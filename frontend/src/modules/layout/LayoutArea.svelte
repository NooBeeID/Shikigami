<script lang="ts">
  import { onMount } from 'svelte';
  import { layout, type LayoutStore } from '../../lib/stores/layout';
  import { emptyDoc, leaves, findLeaf, pathOf, type Leaf, type Node, type LayoutDoc } from './model';
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
  const visibleIds = $derived(new Set(maxId ? [maxId] : all.map((l) => l.id)));

  $effect.pre(() => {
    syncPanes(all, visibleIds);
  });

  onMount(() => installLayoutShortcuts(store));

  function inMax(path: string): boolean {
    return maxPath === null || maxPath === path || path === '' || maxPath.startsWith(path + '.');
  }

  function hostSlot(node: HTMLElement, p: { leaf: Leaf; visible: boolean }) {
    ensureHost(p.leaf, p.visible);
    attachHost(p.leaf.id, node);
    const id = p.leaf.id;
    return {
      destroy() {
        const h = getHost(id);
        if (h && h.parentElement === node) detachHost(id);
      },
    };
  }

  // Tutup pane = akhiri sesinya juga, supaya hilang dari sidebar.
  async function closeAndEnd(leaf: Leaf) {
    store.closePane(leaf.id);
    if (leaf.kind === 'terminal' && leaf.sessionId) await endSession(leaf);
  }

  async function endSession(leaf: Leaf) {
    try {
      await getPaneKind(leaf.kind)?.onEndSession?.(leaf);
    } catch (err) {
      console.error('[layout] gagal mengakhiri sesi:', err);
    }
  }
</script>

{#snippet renderNode(n: Node, path: string, grow: number)}
  {#if n.type === 'pane'}
    {#key n.id}
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
          onMinimize={() => store.closePane(n.id)}
          onRename={(t) => store.renamePane(n.id, t)}
          onEndSession={() => void endSession(n)}
          onTidy={() => store.tidyGrid()}
        />
        <div class="pane-slot" use:hostSlot={{ leaf: n, visible: visibleIds.has(n.id) }}></div>
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
