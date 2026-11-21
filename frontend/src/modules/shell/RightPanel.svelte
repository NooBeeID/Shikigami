<script lang="ts">
  import { rightTabs } from './registry';
  import { rightSidebar, setRightTab } from '../../lib/stores/shell';

  const tabs = rightTabs();
  const active = $derived($tabs.find((t) => t.id === $rightSidebar.activeTab) ?? $tabs[0] ?? null);
  const single = $derived($tabs.length === 1);
  const headerLabel = $derived(active ? (active.id === 'files' ? 'FILE' : active.title.toUpperCase()) : '');
</script>

<div class="right-panel">
  {#if !active}
    <div class="empty"></div>
  {:else}
    {#if single}
      <div class="header">{headerLabel}</div>
    {:else}
      <div class="tabbar" role="tablist">
        {#each $tabs as t (t.id)}
          <button
            role="tab"
            aria-selected={t.id === active.id}
            class:active={t.id === active.id}
            onclick={() => setRightTab(t.id)}
          >{t.title}</button>
        {/each}
      </div>
    {/if}
    <div class="body">
      {#key active.id}
        {@const C = active.component}
        <C />
      {/key}
    </div>
  {/if}
</div>

<style>
  .right-panel { display: flex; flex-direction: column; height: 100%; min-width: 0; min-height: 0; overflow: hidden; }
  .header { padding: 8px 12px; font-size: 11px; font-weight: 600; letter-spacing: 0.08em; color: var(--fg-muted); border-bottom: 1px solid var(--border); }
  .tabbar { display: flex; border-bottom: 1px solid var(--border); }
  .tabbar button { flex: 1; border: 0; border-radius: 0; background: transparent; color: var(--fg-muted); padding: 7px 8px; }
  .tabbar button.active { color: var(--fg); box-shadow: inset 0 -2px var(--accent); }
  .body { flex: 1; min-height: 0; min-width: 0; overflow: auto; }
  .empty { flex: 1; }
</style>
