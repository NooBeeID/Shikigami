<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { activityEntries, ensureActivityStore } from './store';
  import { sessions } from '../../lib/stores';
  import { focusOrOpenSession, sessionTitle } from '../shell';
  import ActivityRow from './ActivityRow.svelte';

  let filter = $state('');
  let listEl: HTMLDivElement | undefined = $state();

  onMount(() => { ensureActivityStore(); });

  function alive(id: string): boolean {
    return $sessions.some((s) => s.id === id);
  }
  function nameOf(id: string): string {
    return alive(id) ? (sessionTitle(id) || id.slice(0, 8)) : id.slice(0, 8);
  }

  const entries = $derived($activityEntries);
  const ids = $derived([...new Set(entries.map((e) => e.sessionId))]);
  const visible = $derived(filter ? entries.filter((e) => e.sessionId === filter) : entries);

  $effect(() => {
    if (filter && !ids.includes(filter)) filter = '';
  });

  function open(id: string) {
    if (alive(id)) focusOrOpenSession(id);
  }

  let prevHeight = 0;
  let prevTop = 0;
  $effect.pre(() => {
    visible.length;
    if (listEl) { prevHeight = listEl.scrollHeight; prevTop = listEl.scrollTop; }
  });
  $effect(() => {
    visible.length;
    const el = listEl;
    if (!el) return;
    tick().then(() => {
      if (prevTop > 4) el.scrollTop = prevTop + (el.scrollHeight - prevHeight);
    });
  });
</script>

<div class="activity">
  <div class="head">
    <select bind:value={filter} aria-label="Filter sesi">
      <option value="">Semua sesi</option>
      {#each ids as id (id)}
        <option value={id}>{nameOf(id)}</option>
      {/each}
    </select>
  </div>
  {#if visible.length === 0}
    <div class="empty">Belum ada aktivitas</div>
  {:else}
    <div class="list" bind:this={listEl}>
      {#each visible as e, i (`${e.ts}|${e.sessionId}|${e.tool}|${e.summary}|${i}`)}
        <ActivityRow entry={e} name={nameOf(e.sessionId)} alive={alive(e.sessionId)} onclick={() => open(e.sessionId)} />
      {/each}
    </div>
  {/if}
</div>

<style>
  .activity { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .head { padding: 4px 8px; border-bottom: 1px solid var(--border, #444); }
  select {
    width: 100%; font: inherit; font-size: 12px; background: var(--bg-elev, transparent);
    color: var(--fg, inherit); border: 1px solid var(--border, #444); border-radius: 3px;
  }
  .list { flex: 1; min-height: 0; overflow-y: auto; }
  .empty { flex: 1; display: flex; align-items: center; justify-content: center; color: var(--fg-muted, #888); font-size: 12px; }
</style>
