<script lang="ts">
  import { dragState } from './dnd';

  const box = $derived.by(() => {
    const s = $dragState;
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

{#if box}
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
  @media (prefers-reduced-motion: reduce) {
    .drop-zone { transition: none; }
  }
</style>
