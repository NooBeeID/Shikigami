<script lang="ts">
  import { tick } from 'svelte';
  import Modal from './Modal.svelte';
  import { settingsOpen, activePage, closeSettings } from './store';
  import { settingsPages } from './pages';

  const pages = $derived($settingsPages);
  const current = $derived(pages.find((p) => p.id === $activePage) ?? pages[0]);

  function select(id: string): void {
    activePage.set(id);
  }

  async function onTabKey(e: KeyboardEvent): Promise<void> {
    const n = pages.length;
    if (n === 0) return;
    const idx = Math.max(0, pages.findIndex((p) => p.id === current?.id));
    let next: number;
    switch (e.key) {
      case 'ArrowDown':
        next = (idx + 1) % n;
        break;
      case 'ArrowUp':
        next = (idx - 1 + n) % n;
        break;
      case 'Home':
        next = 0;
        break;
      case 'End':
        next = n - 1;
        break;
      default:
        return;
    }
    e.preventDefault();
    e.stopPropagation();
    const id = pages[next].id;
    select(id);
    await tick();
    document.getElementById('settings-tab-' + id)?.focus();
  }
</script>

<Modal open={$settingsOpen} onclose={closeSettings} title="Pengaturan">
  <div class="settings-body">
    <div class="settings-tabs" role="tablist" aria-orientation="vertical" aria-label="Halaman pengaturan">
      {#each pages as p (p.id)}
        <button
          type="button"
          role="tab"
          id={'settings-tab-' + p.id}
          aria-selected={p.id === current?.id}
          aria-controls="settings-panel"
          tabindex={p.id === current?.id ? 0 : -1}
          class:active={p.id === current?.id}
          onclick={() => select(p.id)}
          onkeydown={onTabKey}>{p.title}</button
        >
      {/each}
    </div>
    <div
      class="settings-panel"
      id="settings-panel"
      role="tabpanel"
      aria-labelledby={current ? 'settings-tab-' + current.id : undefined}
    >
      {#if current}
        {@const Page = current.component}
        <Page />
      {/if}
    </div>
  </div>
</Modal>

<style>
  .settings-body {
    display: flex;
    min-height: 360px;
    height: 100%;
    width: 100%;
  }
  .settings-tabs {
    flex: 0 0 180px;
    width: 180px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 8px;
    border-right: 1px solid var(--border, #333);
    overflow-y: auto;
  }
  button[role='tab'] {
    text-align: left;
    background: transparent;
    border: 0;
    border-radius: 6px;
    padding: 8px 10px;
    color: var(--fg, #ddd);
    font: inherit;
    cursor: pointer;
  }
  button[role='tab']:hover,
  button[role='tab'].active {
    background: var(--bg-hover, #333);
  }
  button[role='tab']:focus-visible {
    outline: 1px solid var(--fg-muted, #888);
    outline-offset: -1px;
  }
  .settings-panel {
    flex: 1;
    min-width: 0;
    padding: 16px;
    overflow: auto;
  }
</style>
