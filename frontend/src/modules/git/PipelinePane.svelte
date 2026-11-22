<script lang="ts">
  import { get } from 'svelte/store';
  import { untrack } from 'svelte';
  import { activeWorkspaceId } from '../../lib/stores/workspaces';
  import { openSettings } from '../settings';
  import type { Leaf } from '../layout/model';
  import { parseGitError, type GitError } from './client';
  import { getPipeline, type PipelineDetail } from './pipelineClient';
  import { openInBrowser, copyLink, sendToAgent } from './actions';
  import StatusBadge from './StatusBadge.svelte';
  import StageList from './StageList.svelte';

  let { pane, visible }: { pane: Leaf; visible: boolean } = $props();

  type Phase = 'loading' | 'ready' | 'error' | 'missing';
  let phase = $state<Phase>('loading');
  let detail = $state<PipelineDetail | null>(null);
  let err = $state<GitError | null>(null);
  let seq = 0;

  function formatDuration(sec?: number | null): string {
    if (sec === undefined || sec === null || !(sec > 0)) return '-';
    const s = Math.round(sec);
    if (s < 60) return `${s}s`;
    if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
    return `${Math.floor(s / 3600)}j ${Math.floor((s % 3600) / 60)}m`;
  }

  async function load(provider?: string, projectId?: string, pipelineId?: string) {
    const my = ++seq;
    const ws = get(activeWorkspaceId);
    if (!provider || !projectId || !pipelineId || ws === null) {
      detail = null;
      err = null;
      phase = 'missing';
      return;
    }
    phase = 'loading';
    err = null;
    try {
      const r = await getPipeline({ workspaceId: ws, projectId, id: pipelineId });
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
    const p = pane.provider, pr = pane.projectId, id = pane.pipelineId;
    untrack(() => { void load(p, pr, id); });
    return () => { seq++; };
  });

  function refresh() { void load(pane.provider, pane.projectId, pane.pipelineId); }

  let title = $derived(pane.title || 'Pipeline ' + (pane.pipelineId ?? ''));
  let url = $derived(detail?.url ?? '');
  let canAct = $derived(phase === 'ready' && url !== '');
  let shortSha = $derived((detail?.sha ?? '').slice(0, 7));
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
    {#if detail}
      <div class="meta">
        <StatusBadge status={detail.status} kind="pipeline" />
        <span class="ref" title={detail.ref}>{detail.ref}</span>
        <span class="sha">{shortSha}</span>
        <span>{formatDuration(detail.durationSec)}</span>
        {#if detail.author}<span>{detail.author}</span>{/if}
      </div>
    {/if}
  </div>

  {#if phase === 'missing'}
    <p class="muted">Project atau pipeline tidak ditemukan</p>
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
    <div class="body"><StageList stages={detail.stages ?? []} /></div>
  {:else}
    <p class="muted">Memuat...</p>
  {/if}
</div>

<style>
  .git-pane { display: flex; flex-direction: column; height: 100%; color: var(--fg, #ddd); }
  .git-pane[hidden] { display: none; }
  .head { padding: 8px 12px; border-bottom: 1px solid var(--border, #333); background: var(--bg-elev, #222); display: flex; flex-direction: column; gap: 4px; }
  .line1 { display: flex; align-items: center; gap: 8px; font-size: 14px; }
  .title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 600; }
  .acts { flex: none; display: flex; align-items: center; gap: 2px; }
  .act { background: none; border: none; color: var(--fg-muted, #888); cursor: pointer; font-size: 14px; padding: 0 4px; border-radius: 4px; }
  .act:hover:not(:disabled) { color: var(--fg, #ddd); background: var(--bg-hover, #2c2c2c); }
  .act:focus-visible, .refresh:focus-visible, .errs button:focus-visible { outline: 2px solid var(--accent, #6aa8e8); }
  .act:disabled, .refresh:disabled { opacity: .4; cursor: not-allowed; }
  .refresh { margin-left: 6px; background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 2px 8px; font-size: 13px; cursor: pointer; }
  .refresh:hover:not(:disabled) { background: var(--bg-hover, #2c2c2c); }
  .meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 10px; font-size: 12px; color: var(--fg-muted, #888); }
  .ref { min-width: 0; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sha { font-family: ui-monospace, Menlo, monospace; }
  .body { flex: 1; min-height: 0; overflow: auto; }
  .muted { margin: 0; padding: 8px 12px; color: var(--fg-muted, #888); font-size: 14px; }
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
