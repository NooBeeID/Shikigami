<script lang="ts">
  import {
    MAX_EXTRA_FIELDS,
    type BoardField,
    type TableInfo,
    type ViewInfo,
    type ViewMode,
  } from './boardClient';

  let {
    tables, tableId, views, viewId, mode, canKanban, search,
    kanbanFields, groupFieldId, extraCandidates, extraFieldIds,
    loading, loadingMore, hasMore, loadedCount, shownCount, linkAvailable,
    onTable, onView, onMode, onSearch, onGroup, onExtra, onLoadMore, onRefresh, onCopy, onSend,
  }: {
    tables: TableInfo[]; tableId: string;
    views: ViewInfo[]; viewId: string;
    mode: ViewMode; canKanban: boolean;
    search: string;
    kanbanFields: BoardField[]; groupFieldId: string;
    extraCandidates: BoardField[]; extraFieldIds: string[];
    loading: boolean; loadingMore: boolean; hasMore: boolean;
    loadedCount: number; shownCount: number;
    linkAvailable: boolean;
    onTable: (id: string) => void; onView: (id: string) => void; onMode: (m: ViewMode) => void;
    onSearch: (q: string) => void; onGroup: (id: string) => void; onExtra: (ids: string[]) => void;
    onLoadMore: () => void; onRefresh: () => void; onCopy: () => void; onSend: () => void;
  } = $props();

  let extraOpen = $state(false);
  let extraEl: HTMLDivElement | undefined = $state();

  const NO_KANBAN = 'Tabel ini tidak punya field single-select';
  const copyTitle = $derived(linkAvailable ? 'Salin link Base/view ini' : 'Link Base tidak tersedia');
  const sendTitle = $derived(linkAvailable ? 'Tempel link ke sesi agen aktif' : 'Link Base tidak tersedia');

  function toggleExtra(id: string, on: boolean) {
    if (on) {
      if (extraFieldIds.includes(id) || extraFieldIds.length >= MAX_EXTRA_FIELDS) return;
      onExtra([...extraFieldIds, id]);
    } else {
      onExtra(extraFieldIds.filter((x) => x !== id));
    }
  }

  function onWindowClick(e: MouseEvent) {
    if (extraOpen && extraEl && !extraEl.contains(e.target as Node)) extraOpen = false;
  }
  function onWindowKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && extraOpen) extraOpen = false;
  }
</script>

<svelte:window onclick={onWindowClick} onkeydown={onWindowKey} />

<div class="bar">
  <select aria-label="Tabel" value={tableId} disabled={loading || tables.length === 0} onchange={(e) => onTable(e.currentTarget.value)}>
    {#each tables as t (t.id)}
      <option value={t.id}>{t.name}</option>
    {/each}
  </select>
  <select aria-label="View" value={viewId} disabled={loading || tables.length === 0} onchange={(e) => onView(e.currentTarget.value)}>
    <option value="">View bawaan</option>
    {#each views as v (v.id)}
      <option value={v.id}>{v.name}</option>
    {/each}
  </select>

  <div class="seg">
    <button type="button" aria-pressed={mode === 'grid'} class:on={mode === 'grid'} onclick={() => onMode('grid')}>Grid</button>
    <button type="button" aria-pressed={mode === 'kanban' && canKanban} class:on={mode === 'kanban' && canKanban} disabled={!canKanban} title={canKanban ? '' : NO_KANBAN} onclick={() => onMode('kanban')}>Kanban</button>
  </div>
  {#if !canKanban}
    <span class="hint">{NO_KANBAN}</span>
  {/if}

  {#if mode === 'kanban' && canKanban}
    <select aria-label="Kelompokkan menurut" value={groupFieldId} onchange={(e) => onGroup(e.currentTarget.value)}>
      {#each kanbanFields as f (f.id)}
        <option value={f.id}>{f.name}</option>
      {/each}
    </select>
  {/if}

  <div class="extra" bind:this={extraEl}>
    <button type="button" onclick={() => (extraOpen = !extraOpen)} aria-expanded={extraOpen} disabled={extraCandidates.length === 0}>
      Field tambahan ({extraFieldIds.length}/{MAX_EXTRA_FIELDS})
    </button>
    {#if extraOpen}
      <div class="menu">
        {#each extraCandidates as f (f.id)}
          {@const checked = extraFieldIds.includes(f.id)}
          <label>
            <input type="checkbox" {checked} disabled={!checked && extraFieldIds.length >= MAX_EXTRA_FIELDS} onchange={(e) => toggleExtra(f.id, e.currentTarget.checked)} />
            <span>{f.name}</span>
          </label>
        {/each}
      </div>
    {/if}
  </div>

  <input type="search" placeholder="Cari tiket…" aria-label="Cari tiket" value={search} oninput={(e) => onSearch(e.currentTarget.value)} />
  {#if search.trim() !== ''}
    <span class="hint">{shownCount} dari {loadedCount} tiket termuat</span>
  {/if}

  <button type="button" disabled={!hasMore || loadingMore || loading} onclick={onLoadMore}>{loadingMore ? 'Memuat…' : 'Muat lagi'}</button>
  <button type="button" aria-label="Segarkan" title="Segarkan" disabled={loading} onclick={onRefresh}>↻</button>
  <button type="button" disabled={!linkAvailable} title={copyTitle} onclick={onCopy}>Salin link</button>
  <button type="button" disabled={!linkAvailable} title={sendTitle} onclick={onSend}>Kirim ke agen</button>
</div>

<style>
  .bar { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; padding: 6px 8px; border-bottom: 1px solid var(--border, #8884); }
  button, select, input[type='search'] {
    font: inherit; font-size: 13px; border: 1px solid var(--border, #8884); border-radius: 4px;
    padding: 2px 8px; background: transparent; color: var(--fg, inherit); cursor: pointer;
  }
  input[type='search'] { cursor: text; min-width: 140px; }
  button:disabled, select:disabled { opacity: 0.5; cursor: default; }
  button:not(:disabled):hover { background: var(--bg-hover, #0001); }
  .seg { display: inline-flex; }
  .seg button:first-child { border-top-right-radius: 0; border-bottom-right-radius: 0; }
  .seg button:last-child { border-top-left-radius: 0; border-bottom-left-radius: 0; border-left: 0; }
  .seg button.on { background: var(--bg-hover, #0002); }
  .hint { font-size: 13px; color: var(--fg-muted, #888); }
  .extra { position: relative; }
  .menu {
    position: absolute; top: 100%; left: 0; z-index: 10; margin-top: 2px; min-width: 180px; max-height: 240px; overflow: auto;
    background: var(--bg, #fff); border: 1px solid var(--border, #8884); border-radius: 4px; padding: 4px;
    display: flex; flex-direction: column; gap: 2px;
  }
  .menu label { display: flex; gap: 6px; align-items: center; font-size: 13px; color: var(--fg, inherit); padding: 2px 4px; white-space: nowrap; }
</style>
