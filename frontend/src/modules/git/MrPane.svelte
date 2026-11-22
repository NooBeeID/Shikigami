<script lang="ts">
  import { get } from 'svelte/store';
  import { untrack } from 'svelte';
  import { activeWorkspaceId } from '../../lib/stores/workspaces';
  import { layout } from '../../lib/stores/layout';
  import { openSettings } from '../settings';
  import type { Tab } from '../layout/model';
  import { parseGitError, type GitError } from './client';
  import { getMrDetail, type MergeRequestDetail } from './mrDetailClient';
  import { openInBrowser, copyLink, sendToAgent } from './actions';
  import StatusBadge from './StatusBadge.svelte';
  import ReviewerList from './ReviewerList.svelte';

  let { pane, visible }: { pane: Tab; paneId?: string; visible: boolean } = $props();

  type Phase = 'loading' | 'ready' | 'error' | 'missing';
  let phase = $state<Phase>('loading');
  let detail = $state<MergeRequestDetail | null>(null);
  let err = $state<GitError | null>(null);
  let seq = 0;

  function relativeTime(iso: string): string {
    const t = Date.parse(iso);
    if (!iso || Number.isNaN(t)) return '';
    const d = Math.max(0, (Date.now() - t) / 1000);
    if (d < 60) return 'baru saja';
    const m = Math.floor(d / 60);
    if (m < 60) return `${m} menit lalu`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h} jam lalu`;
    const days = Math.floor(h / 24);
    if (days < 30) return `${days} hari lalu`;
    const months = Math.floor(days / 30);
    if (months < 12) return `${months} bulan lalu`;
    return `${Math.floor(days / 365) || 1} tahun lalu`;
  }

  async function load(provider?: string, projectId?: string, number?: number) {
    const my = ++seq;
    const ws = get(activeWorkspaceId);
    if (!provider || !projectId || !number || ws === null) {
      detail = null;
      err = null;
      phase = 'missing';
      return;
    }
    phase = 'loading';
    err = null;
    try {
      const r = await getMrDetail(ws, projectId, number);
      if (my !== seq) return;
      detail = r;
      phase = 'ready';
    } catch (x) {
      if (my !== seq) return;
      err = parseGitError(x);
      detail = null;
      phase = 'error';
    }
  }

  $effect(() => {
    const p = pane.provider, pr = pane.projectId, n = pane.number;
    untrack(() => { void load(p, pr, n); });
    return () => { seq++; };
  });

  function refresh() { void load(pane.provider, pane.projectId, pane.number); }

  function openPipeline() {
    const lp = detail?.lastPipeline;
    if (!lp) return;
    layout.openPane({
      kind: 'git-pipeline',
      provider: pane.provider,
      projectId: pane.projectId,
      pipelineId: lp.id,
      title: 'Pipeline ' + lp.id,
    });
  }

  let prefix = $derived(pane.provider === 'gitlab' ? '!' : '#');
  let title = $derived(detail?.title || pane.title || 'Merge Request');
  let numLabel = $derived(pane.number ? prefix + pane.number : '');
  let url = $derived(detail?.url ?? '');
  let canAct = $derived(phase === 'ready' && url !== '');
  let rel = $derived(detail ? relativeTime(detail.updatedAt) : '');
  let errTitle = $derived.by(() => {
    if (!err) return '';
    switch (err.code) {
      case 'cli_missing': return err.message || `${pane.provider === 'gitlab' ? 'glab' : 'gh'} belum terpasang`;
      case 'not_logged_in': return 'Belum login';
      case 'permission': return 'Izin kurang';
      case 'timeout': return 'Waktu habis';
      case 'not_found': return err.message;
      default: return err.message || 'Terjadi galat';
    }
  });
  let needSettings = $derived(err !== null && (err.code === 'cli_missing' || err.code === 'not_logged_in'));
</script>

<div class="git-pane" hidden={!visible}>
  <div class="head">
    <div class="line1">
      <span class="num">{numLabel}</span>
      <span class="title" title={title}>{title}</span>
      <span class="acts">
        <button type="button" class="act" aria-label="Buka di browser" title="Buka di browser"
          disabled={!canAct} onclick={() => openInBrowser(url)}>↗</button>
        <button type="button" class="act" aria-label="Salin link" title="Salin link"
          disabled={!canAct} onclick={() => void copyLink(url)}>⧉</button>
        <button type="button" class="act" aria-label="Kirim ke agen" title="Kirim ke agen"
          disabled={!canAct} onclick={() => void sendToAgent(url)}>➤</button>
        <button type="button" class="refresh" disabled={phase === 'loading' || phase === 'missing'} onclick={refresh}>Segarkan</button>
      </span>
    </div>
  </div>

  {#if phase === 'missing'}
    <p class="muted">Project atau MR tidak ditemukan</p>
  {:else if phase === 'error' && err}
    <div class="errbox" role="alert">
      <div>{errTitle}</div>
      <div class="errs">
        {#if needSettings}
          <button type="button" onclick={() => openSettings('connectors')}>Buka Pengaturan &gt; Connector</button>
        {/if}
        <button type="button" onclick={refresh}>Coba lagi</button>
      </div>
    </div>
  {:else if phase === 'ready' && detail}
    <div class="body">
      <div class="meta">
        <StatusBadge status={detail.state} kind="mr" />
        {#if detail.draft}<span class="draft">Draf</span>{/if}
        {#if detail.author}<span>@{detail.author}</span>{/if}
        <span class="branch" title="{detail.sourceBranch} -> {detail.targetBranch}">{detail.sourceBranch} -&gt; {detail.targetBranch}</span>
        {#if rel}<span title={detail.updatedAt}>{rel}</span>{/if}
      </div>

      {#if detail.labels.length > 0}
        <div class="chips">
          {#each detail.labels as l, i (l + i)}<span class="chip">{l}</span>{/each}
        </div>
      {/if}

      <section>
        <h3>Reviewer</h3>
        <ReviewerList reviewers={detail.reviewers} />
      </section>

      {#if detail.mergeable === 'mergeable'}
        <div class="merge good">Bisa digabung</div>
      {:else if detail.mergeable === 'conflict'}
        <div class="merge bad">Konflik</div>
      {/if}

      {#if detail.lastPipeline}
        <section>
          <h3>Pipeline terakhir</h3>
          <button type="button" class="pipe" onclick={openPipeline}>
            <StatusBadge status={detail.lastPipeline.status} kind="pipeline" />
            <span>#{detail.lastPipeline.id}</span>
          </button>
        </section>
      {/if}

      <section>
        <h3>Deskripsi</h3>
        {#if detail.description.trim()}
          <div class="desc">{detail.description}</div>
        {:else}
          <p class="muted nopad">Tidak ada deskripsi</p>
        {/if}
      </section>
    </div>
  {:else}
    <p class="muted">Memuat...</p>
  {/if}
</div>

<style>
  .git-pane { display: flex; flex-direction: column; height: 100%; color: var(--fg, #ddd); }
  .git-pane[hidden] { display: none; }
  .head { padding: 8px 12px; border-bottom: 1px solid var(--border, #333); background: var(--bg-elev, #222); display: flex; flex-direction: column; gap: 4px; }
  .line1 { display: flex; align-items: center; gap: 8px; font-size: 14px; }
  .num { flex: none; color: var(--fg-muted, #888); }
  .title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 600; }
  .acts { flex: none; display: flex; align-items: center; gap: 2px; }
  .act { background: none; border: none; color: var(--fg-muted, #888); cursor: pointer; font-size: 14px; padding: 0 4px; border-radius: 4px; }
  .act:hover:not(:disabled) { color: var(--fg, #ddd); background: var(--bg-hover, #2c2c2c); }
  .act:focus-visible, .refresh:focus-visible, .errs button:focus-visible, .pipe:focus-visible { outline: 2px solid var(--accent, #6aa8e8); }
  .act:disabled, .refresh:disabled { opacity: .4; cursor: not-allowed; }
  .refresh { margin-left: 6px; background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 2px 8px; font-size: 13px; cursor: pointer; }
  .refresh:hover:not(:disabled) { background: var(--bg-hover, #2c2c2c); }
  .body { flex: 1; min-height: 0; overflow: auto; padding: 8px 12px; display: flex; flex-direction: column; gap: 12px; }
  .meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 10px; font-size: 12px; color: var(--fg-muted, #888); }
  .branch { min-width: 0; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: ui-monospace, Menlo, monospace; }
  .draft { color: var(--fg-muted, #888); border: 1px solid var(--border, #333); border-radius: 4px; padding: 0 6px; font-size: 12px; }
  .chips { display: flex; flex-wrap: wrap; gap: 4px; }
  .chip { border: 1px solid var(--border, #333); border-radius: 10px; padding: 0 8px; font-size: 12px; overflow-wrap: anywhere; }
  h3 { margin: 0 0 4px; font-size: 12px; font-weight: 600; color: var(--fg-muted, #888); text-transform: uppercase; letter-spacing: .04em; }
  .merge { font-size: 14px; font-weight: 600; }
  .good { color: var(--success, #6fbf73); }
  .bad { color: var(--danger, #e49a9a); }
  .pipe { display: inline-flex; align-items: center; gap: 6px; background: none; color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 2px 8px; font-size: 13px; cursor: pointer; }
  .pipe:hover { background: var(--bg-hover, #2c2c2c); }
  .desc { white-space: pre-wrap; overflow-wrap: anywhere; font-size: 14px; }
  .muted { margin: 0; padding: 8px 12px; color: var(--fg-muted, #888); font-size: 14px; }
  .muted.nopad { padding: 0; }
  .errbox {
    margin: 8px 12px; padding: 8px 10px; background: var(--bg-elev, #222);
    border: 1px solid var(--border, #333); border-radius: 4px; color: var(--danger, #e49a9a);
    font-size: 14px; display: flex; flex-direction: column; gap: 6px;
  }
  .errs { display: flex; gap: 6px; flex-wrap: wrap; }
  .errs button {
    background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333);
    border-radius: 4px; padding: 2px 8px; font-size: 14px; cursor: pointer;
  }
  .errs button:hover { background: var(--bg-hover, #2c2c2c); }
</style>
