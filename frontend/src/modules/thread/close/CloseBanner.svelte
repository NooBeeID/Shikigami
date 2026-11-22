<script lang="ts">
  import type { Thread } from '../types';
  import { openCloseDialog } from './closeDialogStore';

  let { thread }: { thread: Thread } = $props();
</script>

{#if thread.readyToClose && thread.state === 'active'}
  <div class="banner" role="status">
    <div class="body">
      <div class="title">Goal tercapai</div>
      {#if thread.closeSummary}<div class="summary">{thread.closeSummary}</div>{/if}
    </div>
    <button type="button" class="close-btn" onclick={() => openCloseDialog(thread.id)}>Close</button>
  </div>
{/if}

<style>
  .banner { border: 1px solid var(--ok, #30a46c); border-left-width: 4px; background: color-mix(in srgb, var(--ok, #30a46c) 12%, transparent); border-radius: 6px; margin: 6px 12px; padding: 8px 10px; display: flex; align-items: flex-start; gap: 10px; color: var(--fg, #222); }
  .body { flex: 1; min-width: 0; }
  .title { font-weight: bold; font-size: 14px; color: var(--ok, #30a46c); }
  .summary { white-space: pre-wrap; overflow-wrap: anywhere; font-size: 14px; }
  .close-btn { flex: none; font: inherit; font-size: 14px; color: var(--fg, #222); background: transparent; border: 1px solid var(--border, #8884); border-radius: 6px; padding: 4px 12px; cursor: pointer; }
  .close-btn:hover { background: var(--bg-hover, #0001); }
</style>
