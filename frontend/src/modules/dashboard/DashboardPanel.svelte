<script lang="ts">
  import { onMount } from 'svelte';
  import { agentRows, groupRows } from './store';
  import { STATUS_ORDER, STATUS_LABEL } from './types';
  import type { AgentStatus } from './types';
  import { ensureActivityStore } from '../activity/store';
  import AgentRow from './AgentRow.svelte';

  const DONE_LIMIT = 10;
  let now = $state(Date.now());
  let doneOpen = $state(true);
  let visible = $state(typeof document === 'undefined' ? true : !document.hidden);

  const groups = $derived(groupRows($agentRows));
  const total = $derived($agentRows.length);
  const visibleGroups = $derived(STATUS_ORDER.filter((s) => groups[s].length > 0));
  const doneRows = $derived(groups.done.slice(0, DONE_LIMIT));
  const doneHidden = $derived(Math.max(0, groups.done.length - DONE_LIMIT));
  const counts = $derived(
    Object.fromEntries(STATUS_ORDER.map((s) => [s, groups[s].length])) as Record<AgentStatus, number>,
  );

  onMount(() => {
    ensureActivityStore();
  });

  $effect(() => {
    const onVis = () => {
      visible = !document.hidden;
      if (visible) now = Date.now();
    };
    document.addEventListener('visibilitychange', onVis);
    return () => document.removeEventListener('visibilitychange', onVis);
  });

  $effect(() => {
    if (!visible) return;
    now = Date.now();
    const id = setInterval(() => {
      now = Date.now();
    }, 1000);
    return () => clearInterval(id);
  });
</script>

<div class="panel rows-root">
  <div class="title">Dashboard</div>
  <div class="summary" role="group" aria-label="Ringkasan sesi agent">
    {#each STATUS_ORDER as s (s)}
      <span class="chip chip-{s}"><span class="dot dot-{s}" aria-hidden="true"></span>{STATUS_LABEL[s]} {counts[s]}</span>
    {/each}
  </div>

  {#if total === 0}
    <div class="empty">Belum ada sesi agent</div>
  {:else}
    {#each visibleGroups as s (s)}
      <section class="group" aria-label={STATUS_LABEL[s]}>
        {#if s === 'done'}
          <button type="button" class="group-head toggle" aria-expanded={doneOpen} onclick={() => (doneOpen = !doneOpen)}>
            <span class="caret" aria-hidden="true">{doneOpen ? '▾' : '▸'}</span>{STATUS_LABEL[s]} <span class="count">{groups.done.length}</span>
          </button>
          {#if doneOpen}
            <div role="list" aria-label={STATUS_LABEL[s]}>
              {#each doneRows as row (row.sessionId)}<AgentRow {row} {now} />{/each}
            </div>
            {#if doneHidden > 0}<div class="more">+{doneHidden} lainnya</div>{/if}
          {/if}
        {:else}
          <div class="group-head">{STATUS_LABEL[s]} <span class="count">{groups[s].length}</span></div>
          <div role="list" aria-label={STATUS_LABEL[s]}>
            {#each groups[s] as row (row.sessionId)}<AgentRow {row} {now} />{/each}
          </div>
        {/if}
      </section>
    {/each}
  {/if}
</div>

<style>
  .panel {
    padding: 8px 0;
    overflow-y: auto;
    height: 100%;
    color: var(--fg);
    font-size: 13px;
  }
  .title {
    font-weight: 600;
    text-transform: uppercase;
    font-size: 13px;
    padding: 0 10px 6px;
    color: var(--fg-muted);
  }
  .summary {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    padding: 0 10px 8px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 2px 8px;
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--bg-elev);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex: none;
  }
  .dot-waiting { background: var(--warn, #f5a524); }
  .dot-running { background: var(--ok, #30a46c); }
  .dot-idle { background: var(--fg-muted, #8b8d98); }
  .dot-done { background: var(--accent, #3e63dd); }
  .group-head {
    padding: 6px 10px 2px;
    font-weight: 600;
    color: var(--fg-muted);
  }
  .toggle {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: 0;
    font: inherit;
    font-weight: 600;
    cursor: pointer;
  }
  .toggle:hover,
  .toggle:focus-visible {
    background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent));
  }
  .toggle:focus-visible {
    outline: 1px solid var(--accent, currentColor);
  }
  .caret {
    display: inline-block;
    width: 1em;
  }
  .more,
  .empty {
    padding: 6px 10px;
    color: var(--fg-muted);
  }
</style>
