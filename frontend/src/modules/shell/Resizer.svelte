<script lang="ts">
  import {
    LEFT_MIN, LEFT_MAX, RIGHT_MIN, RIGHT_MAX, LEFT_DEFAULT, RIGHT_DEFAULT,
    setLeftWidth, setRightWidth, leftSidebar, rightSidebar,
  } from '../../lib/stores/shell';

  let { side }: { side: 'left' | 'right' } = $props();

  const visible = $derived(side === 'left' ? $leftSidebar.visible : $rightSidebar.visible);
  const min = $derived(side === 'left' ? LEFT_MIN : RIGHT_MIN);
  const max = $derived(side === 'left' ? LEFT_MAX : RIGHT_MAX);

  let dragging = $state(false);
  let frame: HTMLElement | null = null;
  let pending = 0;
  let raf = 0;

  const cssVar = () => (side === 'left' ? '--left-w' : '--right-w');
  const compute = (x: number) => {
    const raw = side === 'left' ? x : window.innerWidth - x;
    return Math.round(Math.min(max, Math.max(min, raw)));
  };

  function down(e: PointerEvent) {
    frame = (e.currentTarget as HTMLElement).closest('.shell-frame');
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    dragging = true;
    pending = compute(e.clientX);
  }
  function move(e: PointerEvent) {
    if (!dragging) return;
    pending = compute(e.clientX);
    if (raf) return;
    raf = requestAnimationFrame(() => {
      raf = 0;
      frame?.style.setProperty(cssVar(), pending + 'px');
    });
  }
  function up(e: PointerEvent) {
    if (!dragging) return;
    dragging = false;
    if (raf) { cancelAnimationFrame(raf); raf = 0; }
    (e.currentTarget as HTMLElement).releasePointerCapture?.(e.pointerId);
    pending = compute(e.clientX);
    if (side === 'left') setLeftWidth(pending);
    else setRightWidth(pending);
  }
  function reset() {
    if (side === 'left') setLeftWidth(LEFT_DEFAULT);
    else setRightWidth(RIGHT_DEFAULT);
  }
</script>

{#if visible}
  <div
    class="resizer {side}"
    class:dragging
    role="separator"
    aria-orientation="vertical"
    onpointerdown={down}
    onpointermove={move}
    onpointerup={up}
    onpointercancel={up}
    ondblclick={reset}
  ></div>
{/if}

<style>
  .resizer {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 4px;
    cursor: col-resize;
    z-index: 5;
    touch-action: none;
  }
  .resizer.left { right: 0; }
  .resizer.right { left: 0; }
  .resizer:hover, .resizer.dragging { background: var(--accent, #6aa7ff); opacity: 0.5; }
</style>
