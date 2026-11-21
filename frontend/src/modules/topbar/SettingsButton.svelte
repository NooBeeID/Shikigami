<script lang="ts">
  import { zoom, tidyGrid } from '../shell';
  import { toggleLeft, toggleRight, leftSidebar, rightSidebar } from '../../lib/stores/shell';
  import { errText } from './util';

  let open = $state(false);
  let error = $state('');
  let root: HTMLDivElement | undefined = $state();

  type Item = { label: string; hint?: string; run: () => void };
  const groups = $derived<Item[][]>([
    [
      { label: 'Perbesar', hint: '⌘=', run: () => zoom('in') },
      { label: 'Perkecil', hint: '⌘-', run: () => zoom('out') },
      { label: 'Reset zoom', hint: '⌘0', run: () => zoom('reset') },
    ],
    [
      { label: `${$leftSidebar.visible ? 'Sembunyikan' : 'Tampilkan'} sidebar kiri`, hint: '⌘B', run: toggleLeft },
      { label: `${$rightSidebar.visible ? 'Sembunyikan' : 'Tampilkan'} sidebar kanan`, hint: '⌥⌘B', run: toggleRight },
    ],
    [{ label: 'Susun grid', run: tidyGrid }],
  ]);

  function toggle() {
    open = !open;
    error = '';
  }

  function act(it: Item) {
    try {
      it.run();
      open = false;
      error = '';
    } catch (e) {
      error = errText(e);
    }
  }

  function items(): HTMLButtonElement[] {
    return root ? Array.from(root.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')) : [];
  }

  function onMenuKey(e: KeyboardEvent) {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    e.preventDefault();
    const list = items();
    if (!list.length) return;
    const i = list.indexOf(document.activeElement as HTMLButtonElement);
    const next = e.key === 'ArrowDown' ? (i + 1) % list.length : (i - 1 + list.length) % list.length;
    list[next].focus();
  }

  $effect(() => {
    if (!open) return;
    const down = (e: PointerEvent) => {
      if (root && !root.contains(e.target as Node)) open = false;
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') open = false;
    };
    window.addEventListener('pointerdown', down);
    window.addEventListener('keydown', key);
    return () => {
      window.removeEventListener('pointerdown', down);
      window.removeEventListener('keydown', key);
    };
  });
</script>

<div class="wrap" bind:this={root}>
  <button type="button" class="gear" aria-label="Pengaturan" title="Pengaturan" aria-haspopup="menu" aria-expanded={open} onclick={toggle}>
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h0a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h0a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v0a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  </button>
  {#if open}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div class="menu" role="menu" tabindex="-1" onkeydown={onMenuKey}>
      {#each groups as g, gi (gi)}
        {#if gi > 0}<div class="sep" role="separator"></div>{/if}
        {#each g as it (it.label)}
          <button type="button" role="menuitem" class="item" onclick={() => act(it)}>
            <span>{it.label}</span>
            {#if it.hint}<span class="hint">{it.hint}</span>{/if}
          </button>
        {/each}
      {/each}
      {#if error}<div class="err">{error}</div>{/if}
    </div>
  {/if}
</div>

<style>
  .wrap { --wails-draggable: no-drag; position: relative; display: inline-flex; align-items: center; padding: 0 6px; }
  .gear {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    padding: 0;
    background: transparent;
    border: none;
    color: var(--fg-muted, #888);
    border-radius: 4px;
    cursor: pointer;
  }
  .gear:hover, .gear[aria-expanded='true'] { background: var(--bg-hover, #333); color: var(--fg, inherit); }
  .menu {
    position: absolute;
    top: calc(100% + 4px);
    right: 6px;
    z-index: 1000;
    min-width: 230px;
    padding: 4px;
    background: var(--bg-elev, #222);
    border: 1px solid var(--border, #444);
    border-radius: 6px;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  }
  .item {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    width: 100%;
    padding: 5px 8px;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--fg, inherit);
    font-size: 12px;
    text-align: left;
    cursor: pointer;
  }
  .item:hover, .item:focus-visible { background: var(--bg-hover, #333); outline: none; }
  .hint { color: var(--fg-muted, #888); }
  .sep { height: 1px; margin: 4px 0; background: var(--border, #444); }
  .err { padding: 4px 8px; font-size: 11px; color: var(--danger, #f85149); }
</style>
