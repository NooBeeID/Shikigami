<script lang="ts">
  import type { Leaf } from '../layout/model';
  import ThreadTab from './ThreadTab.svelte';

  let { pane, visible }: { pane: Leaf; visible: boolean } = $props();
</script>

{#if pane.threadId}
  <ThreadTab threadId={pane.threadId} {visible} />
{:else}
  <div class="empty">Thread tidak tersedia</div>
{/if}

<style>
  .empty { padding: 16px; font-size: 12px; color: var(--fg-muted, #888); }
</style>
