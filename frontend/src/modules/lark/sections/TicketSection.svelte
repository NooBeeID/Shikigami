<script lang="ts">
  import { get } from 'svelte/store';
  import { activeWorkspaceId } from '../../../lib/stores/workspaces';
  import { layout } from '../../../lib/stores/layout';
  import { larkSources } from '../store';
  import type { TicketSource } from '../client';
  import AddSourceDialog from '../AddSourceDialog.svelte';
  import SourceRow from './SourceRow.svelte';

  let tickets = $state<TicketSource[]>([]);
  let ws = $state<string | null>(get(activeWorkspaceId));
  let addOpen = $state(false);

  $effect(() => larkSources.subscribe((v) => { tickets = v.tickets; }));
  $effect(() => activeWorkspaceId.subscribe((v) => { ws = v; }));

  function open(s: TicketSource) {
    layout.openPane({ kind: 'lark-board', sourceId: s.id, title: s.name });
  }
</script>

<div class="sec">
  <div class="bar">
    <button type="button" class="add" disabled={!ws} onclick={() => (addOpen = true)}>Tambah Base</button>
  </div>
  {#if tickets.length === 0}
    <p class="empty">Belum ada Base. Tambah Base.</p>
  {:else}
    {#each tickets as s (s.id)}
      <SourceRow id={s.id} name={s.name} host={s.host} label="Base" onopen={() => open(s)} />
    {/each}
  {/if}
  <AddSourceDialog kind="ticket" open={addOpen} onclose={() => (addOpen = false)} />
</div>

<style>
  .sec {
    display: flex;
    flex-direction: column;
    padding: 0 6px 6px 18px;
  }
  .bar {
    display: flex;
    justify-content: flex-end;
    padding-bottom: 2px;
  }
  .add {
    background: none;
    border: none;
    color: var(--fg-muted, #888);
    font-size: 14px;
    cursor: pointer;
  }
  .add:hover:not(:disabled) {
    color: var(--fg, #ddd);
  }
  .add:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .empty {
    margin: 0;
    padding: 4px 6px;
    color: var(--fg-muted, #888);
    font-size: 14px;
  }
</style>
