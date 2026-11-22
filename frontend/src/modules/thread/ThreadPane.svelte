<script lang="ts">
  import type { Tab } from '../layout/model';
  import ThreadTab from './ThreadTab.svelte';

  let { pane, visible }: { pane: Tab; paneId?: string; visible: boolean } = $props();
</script>

{#if pane.threadId}
  <ThreadTab threadId={pane.threadId} {visible} />
{:else}
  <div class="empty">Thread tidak tersedia</div>
{/if}

<style>
  .empty { padding: 16px; font-size: 14px; color: var(--fg-muted, #888); }
</style>
