<script lang="ts">
  import { workspaces } from '../../../lib/stores/workspaces';
  import type { Participant, Thread, ThreadMessage } from '../types';
  import ParticipantRow from './ParticipantRow.svelte';

  let { thread }: { thread: Thread; message?: ThreadMessage } = $props();

  function sortParticipants(list: Participant[]): Participant[] {
    const rank = (p: Participant) => (p.role === 'master' ? 0 : 1);
    return (list ?? []).slice().sort((a, b) => rank(a) - rank(b) || (a.name || '').localeCompare(b.name || ''));
  }

  const rows = $derived(sortParticipants(thread.participants));

  let projectNames = $state<Record<string, string>>({});
  $effect(() => {
    const wid = thread.workspaceId;
    return workspaces.subscribe((list) => {
      const map: Record<string, string> = {};
      for (const w of list) {
        if (w.id !== wid) continue;
        for (const p of w.projects ?? []) map[p.id] = p.name;
      }
      projectNames = map;
    });
  });
</script>

<section class="panel">
  <h3 class="title">Peserta <span class="count">{rows.length}</span></h3>
  {#if rows.length === 0}
    <div class="empty">Belum ada peserta</div>
  {:else}
    <div class="list">
      {#each rows as p (p.sessionId)}
        <ParticipantRow participant={p} {thread} projectName={p.projectId ? (projectNames[p.projectId] ?? '') : ''} />
      {/each}
    </div>
  {/if}
</section>

<style>
  .panel { padding: 8px; }
  .title { margin: 0 0 6px; padding: 0 8px; font-size: 12px; font-weight: 600; color: var(--fg, #222); }
  .count { color: var(--fg-muted, #888); font-weight: 400; }
  .list { display: flex; flex-direction: column; gap: 2px; }
  .empty { padding: 6px 8px; font-size: 12px; color: var(--fg-muted, #888); }
</style>
