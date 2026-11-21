<script lang="ts">
  import { onDestroy } from 'svelte';
  import { clampRatio, type SplitPath } from './model';
  import type { LayoutStore } from '../../lib/stores/layout';

  let { dir, path, store }: { dir: 'h' | 'v'; path: SplitPath; store: LayoutStore } = $props();

  let dragging = false;
  let rect: DOMRect | null = null;
  let lastX = 0;
  let lastY = 0;
  let raf = 0;

  function apply() {
    if (!rect) return;
    const r = dir === 'h' ? (lastX - rect.left) / rect.width : (lastY - rect.top) / rect.height;
    if (Number.isFinite(r)) store.resize(path, clampRatio(r));
  }

  function down(e: PointerEvent) {
    if (e.button !== 0) return;
    e.preventDefault();
    const el = e.currentTarget as HTMLElement;
    el.setPointerCapture(e.pointerId);
    dragging = true;
    rect = el.parentElement!.getBoundingClientRect();
    document.body.classList.add('pane-resizing');
  }

  function move(e: PointerEvent) {
    if (!dragging) return;
    lastX = e.clientX;
    lastY = e.clientY;
    if (raf) return;
    raf = requestAnimationFrame(() => {
      raf = 0;
      apply();
    });
  }

  function up(e: PointerEvent) {
    try {
      (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
    } catch {
      /* pointer sudah dilepas */
    }
    if (dragging) {
      lastX = e.clientX || lastX;
      lastY = e.clientY || lastY;
      if (raf) {
        cancelAnimationFrame(raf);
        raf = 0;
      }
      apply();
    }
    dragging = false;
    document.body.classList.remove('pane-resizing');
  }

  function reset() {
    store.resize(path, 0.5);
  }

  onDestroy(() => {
    if (raf) cancelAnimationFrame(raf);
    document.body.classList.remove('pane-resizing');
  });
</script>

<div
  class="splitter {dir}"
  role="separator"
  aria-orientation={dir === 'h' ? 'vertical' : 'horizontal'}
  tabindex="-1"
  onpointerdown={down}
  onpointermove={move}
  onpointerup={up}
  onpointercancel={up}
  ondblclick={reset}
></div>

<style>
  .splitter {
    position: relative;
    flex: 0 0 4px;
    background: var(--border, #333);
    z-index: 3;
    touch-action: none;
  }
  .splitter.h { cursor: col-resize; }
  .splitter.v { cursor: row-resize; }
  .splitter::before { content: ''; position: absolute; }
  .splitter.h::before { inset: 0 -3px; }
  .splitter.v::before { inset: -3px 0; }
  .splitter:hover,
  .splitter:active { background: var(--accent, #4c8dff); }
</style>
