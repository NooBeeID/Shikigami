<script lang="ts">
  import { NO_VALUE_LABEL, type BoardField, type BoardRecord } from './boardClient';

  let {
    fields,
    records,
    groupFieldId,
    extraFieldIds,
    onOpen,
    selectedRecordId = null,
  }: {
    fields: BoardField[];
    records: BoardRecord[];
    groupFieldId: string;
    extraFieldIds: string[];
    onOpen: (recordId: string) => void;
    selectedRecordId?: string | null;
  } = $props();

  const columns = $derived.by(() => {
    const map = new Map<string, BoardRecord[]>();
    const empty: BoardRecord[] = [];
    for (const r of records) {
      const k = (r.values[groupFieldId] ?? '').trim();
      if (!k) {
        empty.push(r);
        continue;
      }
      const arr = map.get(k);
      if (arr) arr.push(r);
      else map.set(k, [r]);
    }
    // Urutan kolom mengikuti urutan opsi di Lark (kolom kosong tetap tampil);
    // nilai di luar opsi menyusul sesuai kemunculan.
    const options = fields.find((f) => f.id === groupFieldId)?.options ?? [];
    const cols: { name: string; items: BoardRecord[] }[] = options.map((name) => ({ name, items: map.get(name) ?? [] }));
    for (const [name, items] of map) {
      if (!options.includes(name)) cols.push({ name, items });
    }
    if (empty.length > 0 || cols.length === 0) cols.push({ name: NO_VALUE_LABEL, items: empty });
    return cols;
  });

  const extras = $derived(
    extraFieldIds.map((id) => fields.find((f) => f.id === id)).filter((f): f is BoardField => !!f),
  );

  function meta(r: BoardRecord): { name: string; value: string }[] {
    const out: { name: string; value: string }[] = [];
    for (const f of extras) {
      const v = (r.values[f.id] ?? '').trim();
      if (v) out.push({ name: f.name, value: v });
      if (out.length >= 2) break;
    }
    return out;
  }
</script>

<div class="board">
  {#each columns as col (col.name)}
    <section class="col">
      <header><span class="name" title={col.name}>{col.name}</span><span class="count">{col.items.length}</span></header>
      <div class="cards">
        {#each col.items as r (r.id)}
          <button type="button" class="card" class:selected={r.id === selectedRecordId} onclick={() => onOpen(r.id)}>
            <span class="ttl">{r.title}</span>
            {#each meta(r) as m (m.name)}
              <span class="meta">{m.name}: {m.value}</span>
            {/each}
          </button>
        {/each}
      </div>
    </section>
  {/each}
</div>

<style>
  .board { flex: 1; min-width: 0; min-height: 0; display: flex; gap: 8px; padding: 8px; overflow-x: auto; align-items: stretch; }
  .col { flex: 0 0 260px; width: 260px; display: flex; flex-direction: column; min-height: 0; border: 1px solid var(--border, #8884); border-radius: 6px; }
  header { display: flex; justify-content: space-between; gap: 6px; padding: 6px 8px; font-size: 14px; font-weight: 600; color: var(--fg, inherit); border-bottom: 1px solid var(--border, #8884); }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .count { color: var(--fg-muted, #888); font-weight: 400; }
  .cards { flex: 1; min-height: 0; overflow-y: auto; padding: 6px; display: flex; flex-direction: column; gap: 6px; }
  .card {
    font: inherit; font-size: 14px; text-align: left; display: flex; flex-direction: column; gap: 3px;
    padding: 6px 8px; border: 1px solid var(--border, #8884); border-radius: 4px; background: transparent;
    color: var(--fg, inherit); cursor: pointer;
  }
  .card:hover { background: var(--bg-hover, #0001); }
  .card.selected { background: var(--bg-hover, #0002); }
  .ttl { display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; word-break: break-word; }
  .meta { font-size: 13px; color: var(--fg-muted, #888); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
