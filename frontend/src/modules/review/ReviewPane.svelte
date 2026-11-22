<script lang="ts">
  import { onMount, tick, untrack } from 'svelte';
  import { onEvent } from '../../lib/ipc';
  import { layout } from '../../lib/stores/layout';
  import { sessions } from '../../lib/stores/sessions';
  import type { Tab } from '../layout/model';
  import { openFilePane } from '../shell/helpers';
  import { fetchReviewFiles, refreshReview } from './store';
  import type { ReviewFile } from './types';
  import { fetchDiff, type DiffResult, type Hunk } from './diff';
  import DiffView from './DiffView.svelte';
  import HunkActions, { type ReviewActionResult } from './HunkActions.svelte';
  import { attachReviewKeys } from './keys';

  let { pane, paneId }: { pane: Tab; paneId: string; visible?: boolean } = $props();

  const sessionId = $derived(pane.reviewSessionId ?? '');
  const path = $derived(pane.filePath ?? '');
  const session = $derived($sessions.find((s) => s.id === sessionId) ?? null);

  let file = $state<ReviewFile | null>(null);
  let supported = $state(true);
  let filesLoaded = $state(false);
  let diffData = $state<DiffResult | null>(null);
  let diffLoading = $state(false);
  let diffError = $state<string | null>(null);
  let diffSeq = 0;
  let filesSeq = 0;

  let selectedHunkIndex = $state(0);
  let diffPaneEl: HTMLElement | undefined = $state();
  let hunkRefs = $state<Record<string, HunkActions | undefined>>({});
  const selectedHunk = $derived(diffData?.hunks[selectedHunkIndex] ?? null);

  async function loadFiles() {
    const sid = sessionId;
    if (!sid) return;
    const seq = ++filesSeq;
    try {
      const res = await fetchReviewFiles(sid);
      if (seq !== filesSeq) return;
      supported = res.supported;
      file = res.files.find((f) => f.path === path) ?? null;
    } catch (e) {
      if (seq !== filesSeq) return;
      console.warn('review.files gagal:', e);
    } finally {
      if (seq === filesSeq) filesLoaded = true;
    }
  }

  async function loadDiff(opts?: { silent?: boolean }) {
    if (!sessionId || !path) return;
    const seq = ++diffSeq;
    if (!opts?.silent && diffData === null) diffLoading = true;
    diffError = null;
    try {
      const d = await fetchDiff(sessionId, path);
      if (seq !== diffSeq) return;
      diffData = d;
    } catch (e) {
      if (seq !== diffSeq) return;
      diffError = e instanceof Error ? e.message : String(e);
    } finally {
      if (seq === diffSeq) diffLoading = false;
    }
  }

  // Muat awal + tiap kali sesi/path tab berubah.
  $effect(() => {
    void sessionId;
    void path;
    untrack(() => {
      selectedHunkIndex = 0;
      diffData = null;
      void loadFiles();
      void loadDiff();
    });
  });

  // Segarkan saat agent mengubah file sesi ini.
  onMount(() =>
    onEvent('review.changed', (_t, payload) => {
      const p = payload as { sessionId?: unknown } | null;
      if (!p || p.sessionId !== sessionId) return;
      void loadFiles();
      void loadDiff({ silent: true });
    }),
  );

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
    await Promise.all([loadDiff({ silent: true }), loadFiles(), refreshReview(sessionId)]);
    // File sudah tuntas (mis. revert seluruh file): tab tidak berguna lagi.
    if (r.removed) layout.closeTab(paneId, pane.id);
  }

  function openInPane() {
    if (!session) return;
    const abs = session.cwd.replace(/\/+$/, '') + '/' + path;
    openFilePane({ projectId: session.projectId, filePath: abs, workspaceId: session.workspaceId });
  }
</script>

{#snippet hunkActions(h: Hunk)}
  <HunkActions bind:this={hunkRefs[h.id]} scope="hunk" hunk={h} {sessionId} {path}
    currentHash={diffData!.currentHash} external={file?.external ?? false} onDone={onActionDone} />
{/snippet}

<div class="review">
  {#if !supported}
    <div class="empty">Provider sesi ini tidak mendukung review perubahan</div>
  {:else if filesLoaded && !file}
    <div class="empty">File ini sudah tidak ada dalam daftar review</div>
  {:else}
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <div class="diffpane" bind:this={diffPaneEl} tabindex="0">
      <div class="dhead">
        <span class="dpath" title={path}>{path}</span>
        {#if file && !file.binary && !file.tooLarge}
          <span class="stat"><span class="plus">+{file.additions}</span> <span class="minus">-{file.deletions}</span></span>
        {/if}
        {#if file?.external}<span class="tag ext" title="File diubah di luar agent">perubahan lain</span>{/if}
        {#if diffData && file}
          <HunkActions scope="file" {sessionId} {path} currentHash={diffData.currentHash} external={file.external} onDone={onActionDone} />
        {/if}
        <button class="open" type="button" aria-label="Buka file" title="Buka file" disabled={!session} onclick={openInPane}>
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path d="M9 2h5v5M14 2L7 9M12 10v3a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1h3"/></svg>
        </button>
      </div>
      {#if diffError}
        <div class="err">
          <span>Gagal memuat diff: {diffError}</span>
          <button class="retry" type="button" onclick={() => loadDiff()}>Coba lagi</button>
        </div>
      {/if}
      <div class="body">
        {#if diffData}
          <DiffView diff={diffData} {path} selectedHunkId={selectedHunk?.id} {hunkActions} {sessionId} />
        {:else if diffLoading}
          <div class="empty">Memuat diff...</div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .review { display: flex; flex-direction: column; height: 100%; min-height: 0; font-size: 14px; color: var(--fg, inherit); }
  .diffpane { display: flex; flex-direction: column; flex: 1; min-height: 0; }
  .diffpane:focus-visible { outline: 1px solid color-mix(in srgb, var(--accent, currentColor) 50%, transparent); outline-offset: -1px; }
  .body { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; }
  .dhead { display: flex; align-items: center; gap: 8px; height: 32px; padding: 0 8px; flex: none; border-bottom: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent)); }
  .dpath { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 600; }
  .stat { flex: none; font-size: 12px; font-variant-numeric: tabular-nums; }
  .plus { color: var(--ok, #16a34a); }
  .minus { color: var(--danger, #dc2626); }
  .tag { color: var(--fg-muted, #888); font-size: 11px; flex: none; }
  .tag.ext { color: var(--warn, #d97706); }
  .empty { flex: 1; display: flex; align-items: center; justify-content: center; padding: 12px; text-align: center; color: var(--fg-muted, #888); font-size: 14px; }
  .open { display: inline-flex; align-items: center; justify-content: center; flex: none; width: 24px; height: 24px; padding: 0; background: none; border: none; color: var(--fg-muted, #888); cursor: pointer; border-radius: 4px; }
  .open:hover:not(:disabled) { color: var(--fg, inherit); background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
  .open:disabled { opacity: 0.4; cursor: default; }
  .open:focus-visible { outline: 1px solid var(--accent, currentColor); }
  .err { display: flex; align-items: center; gap: 8px; flex: none; padding: 8px; color: var(--danger, #dc2626); font-size: 13px; }
  .retry { background: none; border: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent)); color: inherit; font: inherit; font-size: 12px; padding: 2px 8px; border-radius: 4px; cursor: pointer; }
  .retry:hover { background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
</style>
