<script lang="ts">
  import { tabs, activeTabId, activateTab, closeTab, sessions, providers, activeWorkspace } from '../../lib/stores';
  import { tabLabel } from './tab-title';

  const items = $derived(
    $tabs.map((t) => ({ tab: t, ...tabLabel(t, $sessions, $providers, $activeWorkspace) })),
  );
</script>

<div class="tabbar" role="tablist">
  {#each items as it (it.tab.id)}
    <div
      class="tab"
      class:active={it.tab.id === $activeTabId}
      class:exited={it.exited}
      role="tab"
      aria-selected={it.tab.id === $activeTabId}
      tabindex="-1"
      onauxclick={(e) => {
        if (e.button === 1) closeTab(it.tab.id);
      }}
    >
      <button
        class="label"
        onclick={() => activateTab(it.tab.id)}
        title={it.title + (it.subtitle ? ' · ' + it.subtitle : '')}
      >
        {#if it.exited}<span class="dot" aria-label="sesi berakhir"></span>{/if}
        <span class="title">{it.title}</span>
        {#if it.subtitle}<span class="sub">{it.subtitle}</span>{/if}
        {#if it.exited}<span class="sub">(exit)</span>{/if}
      </button>
      <button
        class="close"
        aria-label="Tutup tab"
        onclick={(e) => {
          e.stopPropagation();
          closeTab(it.tab.id);
        }}>×</button
      >
    </div>
  {/each}
</div>

<style>
  .tabbar {
    display: flex;
    height: var(--tabbar-h, 34px);
    overflow-x: auto;
    overflow-y: hidden;
    background: var(--bg-elev, #f3f3f3);
    border-bottom: 1px solid var(--border, #ddd);
    flex-shrink: 0;
  }
  .tab {
    display: flex;
    align-items: center;
    max-width: 200px;
    flex-shrink: 0;
    color: var(--fg, #222);
    border-bottom: 2px solid transparent;
    border-right: 1px solid var(--border, #ddd);
  }
  .tab:hover {
    background: var(--bg-hover, #e8e8e8);
  }
  .tab.active {
    border-bottom-color: var(--accent, #3b82f6);
    background: var(--bg, #fff);
  }
  .tab.exited {
    color: var(--fg-muted, #888);
  }
  button {
    background: none;
    border: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }
  .label {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    padding: 0 6px 0 10px;
    height: 100%;
  }
  .title,
  .sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sub {
    font-size: 11px;
    color: var(--fg-muted, #888);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
    background: var(--danger, #d33);
  }
  .close {
    padding: 0 8px;
    height: 100%;
    color: var(--fg-muted, #888);
  }
  .close:hover {
    color: var(--fg, #222);
  }
</style>
