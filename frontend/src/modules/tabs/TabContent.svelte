<script lang="ts">
  import { tabs, activeTabId } from '../../lib/stores';
  import { tabTypes } from '../../lib/registry';

  const types = tabTypes();
  const activeTab = $derived($tabs.find((t) => t.id === $activeTabId) ?? null);
  const spec = $derived(activeTab ? $types[activeTab.type] : undefined);
</script>

<div class="tabcontent">
  {#if activeTab}
    {#key activeTab.id}
      {#if spec}
        {@const Comp = spec.component}
        <Comp sessionId={activeTab.refId} refId={activeTab.refId} tabId={activeTab.id} />
      {:else}
        <div class="placeholder">tipe tab tidak tersedia</div>
      {/if}
    {/key}
  {:else}
    <div class="placeholder">Belum ada tab</div>
  {/if}
</div>

<style>
  .tabcontent {
    flex: 1;
    min-height: 0;
    position: relative;
    display: flex;
    flex-direction: column;
    background: var(--term-bg, var(--bg, #fff));
  }
  .placeholder {
    margin: auto;
    color: var(--fg-muted, #888);
  }
</style>
