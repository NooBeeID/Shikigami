<script lang="ts">
  import { untrack } from 'svelte';
  import { layout } from '../../lib/stores/layout';
  import { sessions } from '../../lib/stores/sessions';
  import { activeTab, findLeaf } from '../layout/model';
  import { openFilePane } from '../shell/helpers';
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

  // Sesi aktif lokal (meniru penurunan di store.ts; store.ts tidak mengekspor id sesi aktif).
  let sessionId = $state<string | null>(null);
  $effect(() => {
    const doc = $layout;
    let next: string | null | undefined;
    if (doc.root === null) next = null;
    else {
      const leaf = doc.focusId ? findLeaf(doc, doc.focusId) : null;
      const tab = leaf ? activeTab(leaf) : null;
      next = tab && tab.kind === 'terminal' && tab.sessionId ? tab.sessionId : undefined;
    }
    if (next !== undefined && next !== untrack(() => sessionId)) sessionId = next;
  });
  const session = $derived($sessions.find((s) => s.id === sessionId) ?? null);

  // Memilih file membuka diff-nya sebagai tab di area utama (bukan di sidebar yang sempit).
  function openDiff(f: ReviewFile) {
    selected = f;
    onselect?.(f);
    if (!sessionId) return;
    layout.openPane({
      kind: 'review',
      reviewSessionId: sessionId,
      filePath: f.path,
      projectId: session?.projectId,
      title: splitPath(f.path).name,
    });
  }

  function openInPane(f: ReviewFile, e?: Event) {
    e?.stopPropagation();
    if (!session) return;
    const abs = session.cwd.replace(/\/+$/, '') + '/' + f.path;
    openFilePane({ projectId: session.projectId, filePath: abs, workspaceId: session.workspaceId });
  }
</script>

<div class="review">
  {#if !result.supported}
    <div class="empty">Provider sesi ini tidak mendukung review perubahan</div>
  {:else if result.files.length === 0}
    <div class="empty">Belum ada file yang diubah agent di sesi ini</div>
  {:else}
    <div class="listpane">
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
            <li class="item">
              <button
                class="row"
                class:selected={selected?.path === f.path}
                class:reviewed={f.reviewed}
                role="option"
                aria-selected={selected?.path === f.path}
                title={f.path}
                onclick={() => openDiff(f)}
              >
                <span class="st st-{f.status}" title={statusLabel(f.status)} aria-label={statusLabel(f.status)}>{statusLetter(f.status)}</span>
                <span class="path"><span class="name">{sp.name}</span>{#if sp.dir}<span class="dir">{sp.dir}</span>{/if}</span>
                {#if !f.binary && !f.tooLarge}<span class="stat"><span class="plus">+{f.additions}</span> <span class="minus">-{f.deletions}</span></span>{/if}
                {#if f.reviewed}<span class="done" title="Sudah ditinjau" aria-label="Sudah ditinjau">&#10003;</span>{/if}
                {#if f.binary}<span class="tag" title="File biner">biner</span>{/if}
                {#if f.tooLarge}<span class="tag" title="File terlalu besar">terlalu besar</span>{/if}
                {#if f.external}<span class="tag ext" title="File diubah di luar agent">perubahan lain</span>{/if}
              </button>
              <button class="open" type="button" aria-label="Buka file {f.path}" title="Buka file" disabled={!session} onclick={(e) => openInPane(f, e)}>
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path d="M9 2h5v5M14 2L7 9M12 10v3a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1h3"/></svg>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
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
  .tag.ext { color: var(--warn, #d97706); }
  .diffpane:focus-visible { outline: 1px solid color-mix(in srgb, var(--accent, currentColor) 50%, transparent); outline-offset: -1px; }
  .tag { color: var(--fg-muted, #888); font-size: 11px; flex: none; }
  .empty { flex: 1; display: flex; align-items: center; justify-content: center; padding: 12px; text-align: center; color: var(--fg-muted, #888); font-size: 14px; }
  .listpane { display: flex; flex-direction: column; flex: 1; min-height: 0; }
  .listpane.hidden { display: none; }
  .item { display: flex; align-items: center; }
  .item:hover { background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
  .item .row { flex: 1; min-width: 0; width: auto; }
  .item .row:hover { background: none; }
  .item .row.selected { background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
  .open { display: inline-flex; align-items: center; justify-content: center; flex: none; width: 24px; height: 24px; margin-right: 4px; padding: 0; background: none; border: none; color: var(--fg-muted, #888); cursor: pointer; border-radius: 4px; }
  .open:hover:not(:disabled) { color: var(--fg, inherit); background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
  .open:disabled { opacity: 0.4; cursor: default; }
  .open:focus-visible { outline: 1px solid var(--accent, currentColor); }
  .stat { flex: none; font-size: 12px; font-variant-numeric: tabular-nums; }
  .plus { color: var(--ok, #16a34a); }
  .minus { color: var(--danger, #dc2626); }
  .diffpane { display: flex; flex-direction: column; flex: 1; min-height: 0; }
  .dhead { display: flex; align-items: center; gap: 8px; height: 32px; padding: 0 8px; flex: none; border-bottom: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent)); }
  .back { flex: none; background: none; border: none; color: inherit; font: inherit; cursor: pointer; padding: 2px 6px; border-radius: 4px; }
  .back:hover { background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
  .back:focus-visible { outline: 1px solid var(--accent, currentColor); }
  .dpath { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 600; }
  .err { display: flex; align-items: center; gap: 8px; flex: none; padding: 8px; color: var(--danger, #dc2626); font-size: 13px; }
  .retry { background: none; border: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent)); color: inherit; font: inherit; font-size: 12px; padding: 2px 8px; border-radius: 4px; cursor: pointer; }
  .retry:hover { background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
</style>
