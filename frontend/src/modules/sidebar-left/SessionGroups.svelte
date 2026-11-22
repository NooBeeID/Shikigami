<script lang="ts">
  import { sessions, activeWorkspace, activeWorkspaceId } from '../../lib/stores';
  import type { Session } from '../../lib/stores';
  import { layout } from '../../lib/stores/layout';
  import { groupSessionsByProject } from './groups';
  import { sessionFilter } from './registry';
  import SessionRow from './SessionRow.svelte';

  let { onerror, filter }: { onerror: (msg: string) => void; filter?: (s: Session) => boolean } = $props();

  const leaves = layout.leaves;
  const visible = $derived(((s: Session) => $sessionFilter(s) && (filter ? filter(s) : true)));
  const groups = $derived(groupSessionsByProject($sessions.filter(visible), $activeWorkspace?.projects ?? [], $activeWorkspaceId));
  const shown = $derived(new Set($leaves.filter((l) => l.kind === 'terminal' && l.sessionId).map((l) => l.sessionId!)));
  let collapsed = $state<Record<string, boolean>>({});
</script>

<div class="groups">
  {#if groups.length === 0}
    <div class="dim">Belum ada project</div>
  {/if}
  {#each groups as g (g.projectId)}
    <section>
      <button type="button" class="ghead" aria-expanded={!collapsed[g.projectId]} onclick={() => (collapsed[g.projectId] = !collapsed[g.projectId])}>
        <span class="chev" aria-hidden="true">{collapsed[g.projectId] ? '▸' : '▾'}</span>
        <span class="gname">{g.name}</span>
        <span class="count">{g.sessions.length}</span>
      </button>
      {#if !collapsed[g.projectId]}
        {#each g.sessions as s (s.id)}
          <SessionRow session={s} inLayout={shown.has(s.id)} {onerror} />
        {:else}
          <div class="dim empty">Belum ada sesi</div>
        {/each}
      {/if}
    </section>
  {/each}
</div>

<style>
  .groups { flex: 1; min-height: 0; overflow: auto; padding: 12px 14px; }
  .ghead { display: flex; align-items: center; gap: 6px; width: 100%; text-align: left; font: inherit; font-size: 12px; font-weight: 600; color: var(--fg-muted, #888); background: transparent; border: 0; padding: 14px 8px 8px; cursor: pointer; }
  .gname { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .chev { width: 10px; flex: none; }
  .count { font-weight: 400; }
  .dim { font-size: 12px; color: var(--fg-muted, #888); padding: 10px 14px; }
  .empty { padding-left: 26px; opacity: 0.7; }
</style>
