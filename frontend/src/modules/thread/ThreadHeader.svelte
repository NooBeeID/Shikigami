<script lang="ts">
  import ThreadSlot from './ThreadSlot.svelte';
  import type { Thread } from './types';

  let { thread }: { thread: Thread } = $props();
  const closed = $derived(thread.state === 'closed');
</script>

<header>
  <div class="top">
    <span class="title" title={thread.title}>{thread.title}</span>
    <span class="badge" class:ok={!closed}>{closed ? 'ditutup' : 'aktif'}</span>
    {#if thread.readyToClose && !closed}<span class="badge ready">siap ditutup</span>{/if}
    <span class="actions"><ThreadSlot name="header-actions" {thread} /></span>
  </div>
  {#if thread.goal}
    <details>
      <summary>Goal</summary>
      <p>{thread.goal}</p>
    </details>
  {/if}
</header>

<style>
  header { padding: 8px 12px; border-bottom: 1px solid var(--border, #8884); flex: none; }
  .top { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .title { font-weight: 600; font-size: 15px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
  .badge { font-size: 13px; color: var(--fg-muted, #888); border: 1px solid var(--border, #8884); border-radius: 10px; padding: 0 7px; flex: none; }
  .badge.ok { color: var(--ok, #30a46c); }
  .badge.ready { color: var(--accent, #3b82f6); }
  .actions { margin-left: auto; display: flex; gap: 6px; align-items: center; flex: none; }
  details { margin-top: 4px; font-size: 14px; color: var(--fg-muted, #888); }
  summary { cursor: pointer; }
  p { margin: 4px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
