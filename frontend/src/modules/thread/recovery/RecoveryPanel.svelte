<script lang="ts">
  import type { Thread, ThreadMessage } from '../types';
  import DeadBadge from './DeadBadge.svelte';
  import { deadParticipants } from './util';

  let { thread }: { thread: Thread; message?: ThreadMessage } = $props();
  const dead = $derived(thread.state === 'closed' ? [] : deadParticipants(thread));
</script>

{#if dead.length > 0}
  <section class="panel">
    <h3 class="title">Agent mati <span class="count">{dead.length}</span></h3>
    <div class="list">
      {#each dead as p (p.sessionId)}
        <div class="row">
          <span class="name">{p.name || p.role}</span>
          <DeadBadge participant={p} />
        </div>
      {/each}
    </div>
  </section>
{/if}

<style>
  .panel { padding: 8px; }
  .title { margin: 0 0 6px; padding: 0 8px; font-size: 14px; font-weight: 600; color: var(--fg, #222); }
  .count { color: var(--fg-muted, #888); font-weight: 400; }
  .list { display: flex; flex-direction: column; gap: 4px; }
  .row { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; padding: 2px 8px; }
  .name { font-size: 14px; color: var(--fg, #222); }
</style>
