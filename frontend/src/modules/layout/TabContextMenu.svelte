<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { layout } from '../../lib/stores/layout';
  import { findLeaf } from './model';

  interface Props {
    paneId: string;
    tabId: string;
    x: number;
    y: number;
    onClose: () => void;
    onCloseTab: () => void;
    onMinimizeTab?: () => void;
  }
  let { paneId, tabId, x, y, onClose, onCloseTab, onMinimizeTab }: Props = $props();

  let root: HTMLDivElement | undefined = $state();
  // mulai di titik klik (nilai awal sengaja), lalu di-clamp setelah mount
  let pos = $state({ left: 0, top: 0 });
  pos = { left: untrack(() => x), top: untrack(() => y) };

  const canSplit = $derived((findLeaf($layout, paneId)?.tabs.length ?? 0) > 1);

  function place() {
    if (!root) return;
    const w = root.offsetWidth;
    const h = root.offsetHeight;
    const left = Math.max(4, Math.min(x, window.innerWidth - w - 4));
    const top = Math.max(4, Math.min(y, window.innerHeight - h - 4));
    pos = { left, top };
  }

  function items(): HTMLButtonElement[] {
    return root
      ? Array.from(root.querySelectorAll<HTMLButtonElement>('button[role="menuitem"]:not(:disabled)'))
      : [];
  }

  function run(fn: () => void) {
    fn();
    onClose();
  }

  function split() {
    run(() => layout.moveTab(paneId, tabId, paneId, 'right'));
  }
  function closeTab() {
    run(onCloseTab);
  }
  function minimize() {
    run(() => onMinimizeTab?.());
  }

  onMount(() => {
    place();
    items()[0]?.focus();
    const onDown = (e: PointerEvent) => {
      if (root?.contains(e.target as Node)) return;
      onClose();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        onClose();
      } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        const list = items();
        if (!list.length) return;
        e.preventDefault();
        const i = list.indexOf(document.activeElement as HTMLButtonElement);
        let next: number;
        if (i < 0) next = e.key === 'ArrowDown' ? 0 : list.length - 1;
        else next = e.key === 'ArrowDown' ? (i + 1) % list.length : (i - 1 + list.length) % list.length;
        list[next].focus();
      }
    };
    const onScroll = () => onClose();
    const onBlur = () => onClose();
    window.addEventListener('pointerdown', onDown, true);
    window.addEventListener('keydown', onKey, true);
    window.addEventListener('scroll', onScroll, true);
    window.addEventListener('blur', onBlur);
    return () => {
      window.removeEventListener('pointerdown', onDown, true);
      window.removeEventListener('keydown', onKey, true);
      window.removeEventListener('scroll', onScroll, true);
      window.removeEventListener('blur', onBlur);
    };
  });
</script>

<div
  class="tab-menu"
  role="menu"
  tabindex="-1"
  aria-label="Menu tab"
  bind:this={root}
  style="left:{pos.left}px;top:{pos.top}px"
  oncontextmenu={(e) => e.preventDefault()}
>
  <button type="button" role="menuitem" disabled={!canSplit} aria-disabled={!canSplit} onclick={split}>Pisahkan ke pane baru</button>
  <button type="button" role="menuitem" onclick={closeTab}>Tutup tab</button>
  {#if onMinimizeTab}
    <div class="sep" role="separator"></div>
    <button type="button" role="menuitem" onclick={minimize}>Minimize</button>
  {/if}
</div>

<style>
  .tab-menu {
    position: fixed;
    z-index: 9000;
    min-width: 168px;
    padding: 4px;
    background: var(--bg-elev, #222);
    border: 1px solid var(--border, #444);
    border-radius: 6px;
    box-shadow: 0 4px 14px rgba(0, 0, 0, 0.35);
    color: var(--fg, #ddd);
    font-size: 14px;
    display: flex;
    flex-direction: column;
  }
  button {
    background: none;
    border: 0;
    color: inherit;
    font: inherit;
    text-align: left;
    padding: 6px 10px;
    border-radius: 4px;
    cursor: pointer;
  }
  button:hover:not(:disabled), button:focus-visible { background: var(--bg-hover, #333); outline: none; }
  button:disabled { opacity: 0.45; cursor: default; }
  .sep { height: 1px; margin: 4px 0; background: var(--border, #444); }
</style>
