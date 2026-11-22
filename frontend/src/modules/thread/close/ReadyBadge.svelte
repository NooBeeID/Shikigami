<script lang="ts">
  import type { Thread } from '../types';

  let { thread }: { thread: Thread } = $props();
</script>

{#if thread.readyToClose && thread.state === 'active'}
  <span class="chip" title="Master mengusulkan penutupan thread">siap ditutup</span>
{/if}

<style>
  .chip { font-size: 11px; color: var(--ok, #30a46c); border: 1px solid var(--ok, #30a46c); border-radius: 10px; padding: 0 7px; flex: none; white-space: nowrap; }
</style>
