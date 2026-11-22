<script lang="ts">
  import { dragState } from './dnd';

  const marker = $derived.by(() => {
    const s = $dragState;
    if (!s || s.kind !== 'tab' || s.index === null || !s.rect || !s.targetId) return null;
    let x = s.rect.x;
    try {
      const strip = document.querySelector(`[data-pane-id="${CSS.escape(s.targetId)}"] [data-tab-strip]`);
      const tabs = strip ? strip.querySelectorAll<HTMLElement>('[data-tab-id]') : [];
      if (tabs.length > 0) {
        x = s.index < tabs.length
          ? tabs[s.index].getBoundingClientRect().left
          : tabs[tabs.length - 1].getBoundingClientRect().right;
      }
    } catch { /* jatuh ke rect */ }
    return { x, y: s.rect.y, h: s.rect.h };
  });

  const box = $derived.by(() => {
    const s = $dragState;
    if (marker) return null;
    if (!s || !s.rect || !s.zone) return null;
    const { x, y, w, h } = s.rect;
    switch (s.zone) {
      case 'top': return { x, y, w, h: h / 2 };
      case 'bottom': return { x, y: y + h / 2, w, h: h / 2 };
      case 'left': return { x, y, w: w / 2, h };
      case 'right': return { x: x + w / 2, y, w: w / 2, h };
      default: return { x, y, w, h };
    }
  });
</script>

{#if marker}
  <div
    class="drop-marker"
    style="left:{marker.x - 1}px;top:{marker.y}px;height:{marker.h}px"
  ></div>
{:else if box}
  <div
    class="drop-zone"
    style="left:{box.x}px;top:{box.y}px;width:{box.w}px;height:{box.h}px"
  ></div>
{/if}

<style>
  .drop-zone {
    position: fixed;
    pointer-events: none;
    z-index: 9999;
    box-sizing: border-box;
    border: 2px solid var(--accent, #4f8cff);
    background: color-mix(in srgb, var(--accent, #4f8cff) 22%, transparent);
    border-radius: 4px;
    transition: left 80ms, top 80ms, width 80ms, height 80ms;
  }
  .drop-marker {
    position: fixed;
    pointer-events: none;
    z-index: 9999;
    width: 2px;
    background: var(--accent, #4f8cff);
    transition: left 80ms;
  }
  @media (prefers-reduced-motion: reduce) {
    .drop-zone, .drop-marker { transition: none; }
  }
</style>
