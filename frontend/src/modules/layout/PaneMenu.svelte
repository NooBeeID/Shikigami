<script lang="ts">
  import { onMount } from 'svelte';

  interface Props {
    anchor: HTMLElement;
    canEndSession: boolean;
    onEndSession: () => void;
    onRename: () => void;
    onTidy: () => void;
    onClosePane: () => void;
    onDismiss: () => void;
  }
  let { anchor, canEndSession, onEndSession, onRename, onTidy, onClosePane, onDismiss }: Props = $props();

  let root: HTMLDivElement | undefined = $state();
  let confirming = $state(false);
  let pos = $state({ left: 0, top: 0 });

  function place() {
    if (!root) return;
    const r = anchor.getBoundingClientRect();
    const w = root.offsetWidth;
    const h = root.offsetHeight;
    let left = r.right - w;
    let top = r.bottom + 4;
    left = Math.max(4, Math.min(left, window.innerWidth - w - 4));
    top = Math.max(4, Math.min(top, window.innerHeight - h - 4));
    pos = { left, top };
  }

  function items(): HTMLElement[] {
    return root ? Array.from(root.querySelectorAll<HTMLElement>('button')) : [];
  }

  function run(fn: () => void) {
    fn();
    onDismiss();
  }

  onMount(() => {
    place();
    items()[0]?.focus();
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node | null;
      if (t && (root?.contains(t) || anchor.contains(t))) return;
      onDismiss();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        onDismiss();
      } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        const list = items();
        if (!list.length) return;
        e.preventDefault();
        const i = list.indexOf(document.activeElement as HTMLElement);
        const next = e.key === 'ArrowDown' ? (i + 1) % list.length : (i - 1 + list.length) % list.length;
        list[next].focus();
      }
    };
    window.addEventListener('pointerdown', onDown, true);
    window.addEventListener('keydown', onKey, true);
    return () => {
      window.removeEventListener('pointerdown', onDown, true);
      window.removeEventListener('keydown', onKey, true);
    };
  });

  $effect(() => {
    void confirming;
    queueMicrotask(() => {
      place();
      if (confirming) items()[0]?.focus();
    });
  });
</script>

<div class="pane-menu" role="menu" bind:this={root} style="left:{pos.left}px;top:{pos.top}px">
  {#if confirming}
    <div class="confirm">Akhiri sesi tab ini?</div>
    <div class="row">
      <button type="button" class="danger" onclick={() => run(onEndSession)}>Akhiri</button>
      <button type="button" onclick={() => (confirming = false)}>Batal</button>
    </div>
  {:else}
    {#if canEndSession}
      <button type="button" role="menuitem" onclick={() => (confirming = true)}>Akhiri sesi</button>
    {/if}
    <button type="button" role="menuitem" onclick={() => run(onRename)}>Ganti nama tab</button>
    <button type="button" role="menuitem" onclick={() => run(onTidy)}>Susun grid</button>
    <div class="sep"></div>
    <button type="button" role="menuitem" title="Menutup semua tab; sesi terminal diakhiri" onclick={() => run(onClosePane)}>Tutup pane</button>
  {/if}
</div>

<style>
  .pane-menu {
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
  button:hover, button:focus-visible { background: var(--bg-hover, #333); outline: none; }
  .sep { height: 1px; margin: 4px 0; background: var(--border, #444); }
  .confirm { padding: 6px 10px; }
  .row { display: flex; gap: 4px; padding: 0 4px 4px; }
  .row button { flex: 1; text-align: center; border: 1px solid var(--border, #444); }
  button.danger { color: var(--danger, #e5534b); }
</style>
