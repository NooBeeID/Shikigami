<script lang="ts">
  import type { Thread } from '../types';

  let { thread }: { thread: Thread } = $props();

  function formatClosedAt(v?: string): string {
    if (!v) return '';
    const d = new Date(v);
    if (isNaN(d.getTime())) return '';
    return d.toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' });
  }

  const when = $derived(formatClosedAt(thread.closedAt));
</script>

{#if thread.state === 'closed'}
  <div class="notice" role="status">Thread ditutup{when ? ` pada ${when}` : ''}</div>
{/if}

<style>
  .notice { margin: 6px 12px; padding: 6px 10px; font-size: 12px; color: var(--fg-muted, #888); border: 1px solid var(--border, #8884); border-radius: 6px; }
</style>
