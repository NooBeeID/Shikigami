<script lang="ts">
  import { primaryField, type BoardField, type BoardRecord } from './boardClient';

  let {
    fields,
    records,
    extraFieldIds = [],
    onOpen,
    selectedRecordId = null,
  }: {
    fields: BoardField[];
    records: BoardRecord[];
    extraFieldIds?: string[];
    onOpen: (recordId: string) => void;
    selectedRecordId?: string | null;
  } = $props();

  const primary = $derived(primaryField(fields));
  const columns = $derived.by(() => {
    const rest = fields.filter((f) => f !== primary);
    const extras = extraFieldIds.map((id) => rest.find((f) => f.id === id)).filter((f): f is BoardField => !!f);
    const others = rest.filter((f) => !extras.includes(f));
    return [...extras, ...others];
  });

  function key(e: KeyboardEvent, id: string) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onOpen(id);
    }
  }
</script>

<div class="wrap">
  <table>
    <thead>
      <tr>
        <th class="title">{primary?.name ?? 'Judul'}</th>
        {#each columns as f (f.id)}
          <th>{f.name}</th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each records as r (r.id)}
        <tr
          tabindex="0"
          role="button"
          class:selected={r.id === selectedRecordId}
          onclick={() => onOpen(r.id)}
          onkeydown={(e) => key(e, r.id)}
        >
          <td class="title" title={r.title}>{r.title}</td>
          {#each columns as f (f.id)}
            {@const v = r.values[f.id] ?? ''}
            <td title={v}>{v}</td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .wrap { flex: 1; min-width: 0; min-height: 0; overflow: auto; }
  table { border-collapse: separate; border-spacing: 0; font-size: 12px; color: var(--fg, inherit); }
  th, td {
    text-align: left; padding: 4px 10px; border-bottom: 1px solid var(--border, #8884);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 280px;
  }
  th { position: sticky; top: 0; z-index: 2; background: var(--bg, #fff); color: var(--fg-muted, #888); font-weight: 600; }
  th.title, td.title { position: sticky; left: 0; background: var(--bg, #fff); border-right: 1px solid var(--border, #8884); }
  th.title { z-index: 3; }
  tbody tr { cursor: pointer; }
  tbody tr:hover td { background: var(--bg-hover, #0001); }
  tbody tr.selected td { background: var(--bg-hover, #0002); }
  tbody tr:focus-visible { outline: 1px solid var(--fg-muted, #888); outline-offset: -1px; }
</style>
