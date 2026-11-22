<script lang="ts">
  import { reviewCounts } from './store';
  let { sessionId }: { sessionId: string } = $props();
  const count = $derived($reviewCounts[sessionId] ?? 0);
</script>

{#if count > 0}
  <span class="review-badge" role="status" aria-label="{count} file belum ditinjau" title="{count} file belum ditinjau">{count > 99 ? '99+' : count}</span>
{/if}

<style>
  .review-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 16px;
    height: 16px;
    padding: 0 5px;
    border-radius: 8px;
    font-size: 11px;
    font-weight: 600;
    line-height: 1;
    flex: none;
    background: var(--accent, #3b82f6);
    color: var(--accent-fg, #fff);
  }
</style>
