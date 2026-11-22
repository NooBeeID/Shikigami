<script lang="ts">
  import { tick, untrack } from 'svelte';
  import { layout } from '../../lib/stores/layout';
  import { sessions } from '../../lib/stores/sessions';
  import { activeTab, findLeaf } from '../layout/model';
  import { openFilePane } from '../shell/helpers';
  import { reviewFiles, refreshReview } from './store';
  import type { ReviewFile, ReviewStatus } from './types';
  import { fetchDiff, type DiffResult, type Hunk } from './diff';
  import DiffView from './DiffView.svelte';
  import HunkActions, { type ReviewActionResult } from './HunkActions.svelte';
  import { attachReviewKeys } from './keys';

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

  let selectedPath = $state<string | null>(null);
  let diffData = $state<DiffResult | null>(null);
  let diffLoading = $state(false);
  let diffError = $state<string | null>(null);
  let diffSeq = 0;
  let listEl: HTMLUListElement | undefined = $state();
  let savedScroll = 0;

  const selectedFile = $derived(selectedPath === null ? null : (result.files.find((f) => f.path === selectedPath) ?? null));

  let selectedHunkIndex = $state(0);
  let diffPaneEl: HTMLElement | undefined = $state();
  let hunkRefs = $state<Record<string, HunkActions | undefined>>({});
  let fileActions: HunkActions | undefined = $state();
  const selectedHunk = $derived(diffData?.hunks[selectedHunkIndex] ?? null);

  // Clamp indeks hunk terpilih dan bersihkan registri basi saat diff berubah.
  $effect(() => {
    const hunks = diffData?.hunks ?? [];
    untrack(() => {
      const max = Math.max(0, hunks.length - 1);
      if (selectedHunkIndex > max) selectedHunkIndex = max;
      const ids = new Set(hunks.map((h) => h.id));
      for (const k of Object.keys(hunkRefs)) if (!ids.has(k)) delete hunkRefs[k];
    });
  });

  async function moveHunk(delta: number) {
    const n = diffData?.hunks.length ?? 0;
    if (n === 0 || !selectedHunk) return;
    hunkRefs[selectedHunk.id]?.cancel();
    selectedHunkIndex = Math.min(n - 1, Math.max(0, selectedHunkIndex + delta));
    await tick();
    if (!selectedHunk) return;
    diffPaneEl?.querySelector(`[data-hunk-id="${CSS.escape(selectedHunk.id)}"]`)?.scrollIntoView({ block: 'nearest' });
  }

  function curRef(): HunkActions | undefined {
    return selectedHunk ? hunkRefs[selectedHunk.id] : undefined;
  }

  $effect(() => {
    const el = diffPaneEl;
    if (!el) return;
    return attachReviewKeys({
      target: el,
      next: () => void moveHunk(1),
      prev: () => void moveHunk(-1),
      accept: () => void curRef()?.accept(),
      revert: () => {
        const r = curRef();
        if (!r) return;
        if (r.isPending()) void r.confirmRevert();
        else r.startRevert();
      },
      confirm: () => void curRef()?.confirmRevert(),
      cancel: () => curRef()?.cancel(),
      isConfirming: () => curRef()?.isPending() === true,
    });
  });

  async function onActionDone(r: ReviewActionResult) {
    const sid = sessionId;
    if (!sid) return;
    const idx = visible.findIndex((f) => f.path === selectedPath);
    const nextPath = r.removed ? ((visible[idx + 1] ?? visible[idx - 1])?.path ?? null) : null;
    await Promise.all([loadDiff({ silent: true }), refreshReview(sid)]);
    if (r.removed) {
      const next = nextPath ? $reviewFiles.files.find((f) => f.path === nextPath) : undefined;
      if (next) openDiff(next);
      else await backToList();
    }
  }

  function openDiff(f: ReviewFile) {
    selectedHunkIndex = 0;
    savedScroll = listEl?.scrollTop ?? 0;
    selectedPath = f.path;
    diffData = null;
    pick(f);
    void loadDiff();
  }

  async function loadDiff(opts?: { silent?: boolean }) {
    if (!sessionId || selectedPath === null) return;
    const seq = ++diffSeq;
    const sid = sessionId;
    const path = selectedPath;
    if (!opts?.silent && diffData === null) diffLoading = true;
    diffError = null;
    try {
      const d = await fetchDiff(sid, path);
      if (seq !== diffSeq) return;
      diffData = d;
    } catch (e) {
      if (seq !== diffSeq) return;
      diffError = e instanceof Error ? e.message : String(e);
    } finally {
      if (seq === diffSeq) diffLoading = false;
    }
  }

  async function backToList() {
    diffSeq++;
    selectedPath = null;
    diffData = null;
    diffError = null;
    diffLoading = false;
    await tick();
    if (listEl) listEl.scrollTop = savedScroll;
  }

  // Segarkan diff saat daftar file berubah; kembali ke daftar bila file hilang.
  $effect(() => {
    const files = result.files;
    const supported = result.supported;
    untrack(() => {
      if (selectedPath === null) return;
      if (!supported || !files.some((f) => f.path === selectedPath)) void backToList();
      else void loadDiff({ silent: true });
    });
  });

  // Sesi berganti: tutup diff.
  let prevSessionId: string | null = null;
  $effect(() => {
    const sid = sessionId;
    if (sid === prevSessionId) return;
    prevSessionId = sid;
    if (untrack(() => selectedPath) !== null) void backToList();
  });

  function openInPane(f: ReviewFile | string, e?: Event) {
    e?.stopPropagation();
    if (!session) return;
    const rel = typeof f === 'string' ? f : f.path;
    const abs = session.cwd.replace(/\/+$/, '') + '/' + rel;
    openFilePane({ projectId: session.projectId, filePath: abs, workspaceId: session.workspaceId });
  }
</script>

{#snippet hunkActions(h: Hunk)}
  <HunkActions bind:this={hunkRefs[h.id]} scope="hunk" hunk={h} sessionId={sessionId!} path={selectedPath!}
    currentHash={diffData!.currentHash} external={selectedFile?.external ?? false} onDone={onActionDone} />
{/snippet}

<div class="review">
  {#if !result.supported}
    <div class="empty">Provider sesi ini tidak mendukung review perubahan</div>
  {:else if result.files.length === 0}
    <div class="empty">Belum ada file yang diubah agent di sesi ini</div>
  {:else}
    <div class="listpane" class:hidden={selectedPath !== null}>
      <div class="head">
        <label class="filter"><input type="checkbox" bind:checked={onlyUnreviewed} /> Belum ditinjau</label>
        <span class="sum">{unreviewed} dari {result.files.length} belum ditinjau</span>
      </div>
      {#if visible.length === 0}
        <div class="empty">Semua file sudah ditinjau</div>
      {:else}
        <ul class="list" role="listbox" aria-label="File yang diubah" bind:this={listEl}>
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
    {#if selectedPath !== null}
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <div class="diffpane" bind:this={diffPaneEl} tabindex="0">
        <div class="dhead">
          <button class="back" type="button" aria-label="Kembali ke daftar" onclick={backToList}>&lt; Kembali</button>
          <span class="dpath" title={selectedPath}>{selectedPath}</span>
          {#if selectedFile && !selectedFile.binary && !selectedFile.tooLarge}
            <span class="stat"><span class="plus">+{selectedFile.additions}</span> <span class="minus">-{selectedFile.deletions}</span></span>
          {/if}
          {#if selectedFile?.external}<span class="tag ext" title="File diubah di luar agent">perubahan lain</span>{/if}
          {#if diffData && selectedFile}
            <HunkActions bind:this={fileActions} scope="file" sessionId={sessionId!} path={selectedPath}
              currentHash={diffData.currentHash} external={selectedFile.external} onDone={onActionDone} />
          {/if}
          <button class="open" type="button" aria-label="Buka file" title="Buka file" disabled={!session} onclick={(e) => openInPane(selectedPath!, e)}>
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path d="M9 2h5v5M14 2L7 9M12 10v3a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1h3"/></svg>
          </button>
        </div>
        {#if diffError}
          <div class="err">
            <span>Gagal memuat diff: {diffError}</span>
            <button class="retry" type="button" onclick={() => loadDiff()}>Coba lagi</button>
          </div>
        {/if}
        {#if diffData}
          <DiffView diff={diffData} path={selectedPath} selectedHunkId={selectedHunk?.id} {hunkActions} sessionId={sessionId ?? undefined} />
        {:else if diffLoading}
          <div class="empty">Memuat diff...</div>
        {/if}
      </div>
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
