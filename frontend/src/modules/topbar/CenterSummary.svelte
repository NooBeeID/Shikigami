<script lang="ts">
  import { activeWorkspace, activeWorkspaceId, sessions } from '../../lib/stores';
  import { statusStore } from '../status/store';
  import { providerBadge, sessionsOfActive } from './util';

  const ws = $derived($activeWorkspace);
  const mine = $derived(sessionsOfActive($sessions, $activeWorkspaceId));
  const live = $derived(mine.filter((s) => s.status !== 'exited'));
  const running = $derived(mine.some((s) => s.status === 'running' && $statusStore[s.id]?.status === 'running'));
  const projects = $derived(ws?.projects ?? []);
  const projText = $derived(
    projects.length === 0
      ? 'tanpa project'
      : projects.slice(0, 3).map((p) => p.name).join(' · ') + (projects.length > 3 ? ` +${projects.length - 3}` : ''),
  );
  const badges = $derived([...new Set(live.map((s) => s.providerId))]);
</script>

{#if ws}
  <div class="summary">
    <strong class="name">{ws.name}</strong>
    <span class="state"><span class="dot" class:on={running}></span>{running ? 'Running' : 'Idle'}</span>
    <span class="muted">{live.length} sesi</span>
    <span class="muted proj">{projText}</span>
    {#each badges as b (b)}
      <span class="badge" title={b}>{providerBadge(b)}</span>
    {/each}
  </div>
{/if}

<style>
  .summary {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
    color: var(--fg, inherit);
  }
  .name { overflow: hidden; text-overflow: ellipsis; }
  .muted { color: var(--fg-muted, #888); }
  .proj { overflow: hidden; text-overflow: ellipsis; min-width: 0; }
  .state { display: inline-flex; align-items: center; gap: 5px; }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--fg-muted, #888);
    opacity: 0.4;
  }
  .dot.on { background: var(--ok, #3fb950); opacity: 1; }
  .badge {
    padding: 0 6px;
    font-size: 10px;
    border: 1px solid var(--border, #444);
    border-radius: 8px;
    color: var(--fg-muted, #888);
  }
</style>
