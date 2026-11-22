<script lang="ts">
  import { reviewFiles } from './store';
  import type { ReviewFile, ReviewStatus } from './types';

  let { onselect }: { onselect?: (file: ReviewFile) => void } = $props();

  let onlyUnreviewed = $state(false);
  let selected = $state<ReviewFile | null>(null);

  const result = $derived($reviewFiles);
  const visible = $derived(onlyUnreviewed ? result.files.filter((f) => !f.reviewed) : result.files);
  const unreviewed = $derived(result.files.filter((f) => !f.reviewed).length);

  // Titik kait: lepas pilihan bila file sudah tidak ada; segarkan objek bila ada.
  $effect(() => {
    if (!selected) return;
    const cur = result.files.find((f) => f.path === selected!.path);
    if (!cur) selected = null;
    else if (cur !== selected) selected = cur;
  });

  function splitPath(p: string): { dir: string; name: string } {
    const i = p.lastIndexOf('/');
    return i < 0 ? { dir: '', name: p } : { dir: p.slice(0, i + 1), name: p.slice(i + 1) };
  }
  function statusLetter(s: ReviewStatus): 'M' | 'A' | 'D' {
    return s === 'added' ? 'A' : s === 'deleted' ? 'D' : 'M';
  }
  function statusLabel(s: ReviewStatus): string {
    return s === 'added' ? 'Baru' : s === 'deleted' ? 'Dihapus' : 'Diubah';
  }
  function pick(f: ReviewFile) {
    selected = f;
    onselect?.(f);
  }
</script>

<div class="review">
  {#if !result.supported}
    <div class="empty">Provider sesi ini tidak mendukung review perubahan</div>
  {:else if result.files.length === 0}
    <div class="empty">Belum ada file yang diubah agent di sesi ini</div>
  {:else}
    <div class="head">
      <label class="filter"><input type="checkbox" bind:checked={onlyUnreviewed} /> Belum ditinjau</label>
      <span class="sum">{unreviewed} dari {result.files.length} belum ditinjau</span>
    </div>
    {#if visible.length === 0}
      <div class="empty">Semua file sudah ditinjau</div>
    {:else}
      <ul class="list" role="listbox" aria-label="File yang diubah">
        {#each visible as f (f.path)}
          {@const sp = splitPath(f.path)}
          <li>
            <button
              class="row"
              class:selected={selected?.path === f.path}
              class:reviewed={f.reviewed}
              role="option"
              aria-selected={selected?.path === f.path}
              title={f.path}
              onclick={() => pick(f)}
            >
              <span class="st st-{f.status}" title={statusLabel(f.status)} aria-label={statusLabel(f.status)}>{statusLetter(f.status)}</span>
              <span class="path"><span class="name">{sp.name}</span>{#if sp.dir}<span class="dir">{sp.dir}</span>{/if}</span>
              {#if f.reviewed}<span class="done" title="Sudah ditinjau" aria-label="Sudah ditinjau">&#10003;</span>{/if}
              {#if f.binary}<span class="tag" title="File biner">biner</span>{/if}
              {#if f.tooLarge}<span class="tag" title="File terlalu besar">besar</span>{/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
</div>

<style>
  .review { display: flex; flex-direction: column; height: 100%; min-height: 0; font-size: 14px; color: var(--fg, inherit); }
  .head { display: flex; align-items: center; justify-content: space-between; gap: 8px; height: 32px; padding: 0 8px; flex: none; border-bottom: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent)); }
  .filter { display: inline-flex; align-items: center; gap: 6px; cursor: pointer; }
  .sum { color: var(--fg-muted, #888); font-size: 12px; }
  .list { flex: 1; min-height: 0; overflow-y: auto; margin: 0; padding: 0; list-style: none; }
  .row { display: flex; align-items: center; gap: 8px; width: 100%; height: 28px; padding: 0 8px; background: none; border: none; color: inherit; font: inherit; text-align: left; cursor: pointer; }
  .row:hover, .row.selected { background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
  .row:focus-visible { outline: 1px solid var(--accent, currentColor); outline-offset: -1px; }
  .reviewed .path { opacity: 0.6; }
  .st { width: 14px; text-align: center; font-weight: 700; font-size: 12px; flex: none; }
  .st-modified { color: var(--warn, #d97706); }
  .st-added { color: var(--ok, #16a34a); }
  .st-deleted { color: var(--danger, #dc2626); }
  .path { flex: 1; min-width: 0; display: flex; gap: 6px; overflow: hidden; white-space: nowrap; }
  .name { font-weight: 600; flex: none; }
  .dir { color: var(--fg-muted, #888); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .done { color: var(--ok, #16a34a); flex: none; }
  .tag { color: var(--fg-muted, #888); font-size: 11px; flex: none; }
  .empty { flex: 1; display: flex; align-items: center; justify-content: center; padding: 12px; text-align: center; color: var(--fg-muted, #888); font-size: 14px; }
</style>
