<script lang="ts">
  import { get } from 'svelte/store';
  import { untrack } from 'svelte';
  import { activeWorkspaceId } from '../../lib/stores/workspaces';
  import { layout } from '../../lib/stores/layout';
  import { openSettings } from '../settings';
  import { parseGitError, type Provider, type Repo, type GitError } from './client';
  import { listPipelines, type Pipeline } from './pipelineClient';
  import PipelineRow from './PipelineRow.svelte';

  let { provider, repo }: { provider: Provider; repo: Repo } = $props();

  let status = $state<'loading' | 'ready' | 'error'>('loading');
  let pipelines = $state<Pipeline[]>([]);
  let err = $state<GitError | null>(null);
  let seq = 0;

  async function load(repoArg: Repo) {
    const my = ++seq;
    const ws = get(activeWorkspaceId);
    if (ws === null) {
      pipelines = [];
      err = null;
      status = 'ready';
      return;
    }
    status = 'loading';
    err = null;
    try {
      const r = await listPipelines({ workspaceId: ws, projectId: repoArg.projectId });
      if (my !== seq) return;
      pipelines = r;
      status = 'ready';
    } catch (x) {
      if (my !== seq) return;
      err = parseGitError(x);
      pipelines = [];
      status = 'error';
    }
  }

  $effect(() => {
    const r = repo;
    untrack(() => { void load(r); });
    return () => { seq++; };
  });

  function retry() { void load(repo); }

  function openPipeline(p: Pipeline) {
    layout.openPane({
      kind: 'git-pipeline',
      provider,
      projectId: repo.projectId,
      pipelineId: p.id,
      title: 'Pipeline ' + p.id + ' (' + p.ref + ')',
    });
  }

  let errTitle = $derived.by(() => {
    if (!err) return '';
    switch (err.code) {
      case 'cli_missing': return err.message || `${provider === 'gitlab' ? 'glab' : 'gh'} belum terpasang`;
      case 'not_logged_in': return 'Belum login';
      case 'permission': return 'Izin kurang';
      case 'timeout': return 'Waktu habis';
      case 'not_found': return err.message;
      default: return err.message || 'Terjadi galat';
    }
  });
  let needSettings = $derived(err !== null && (err.code === 'cli_missing' || err.code === 'not_logged_in'));
</script>

<div class="pipelines">
  {#if status === 'loading'}
    <p class="muted">Memuat...</p>
  {:else if status === 'error' && err}
    <div class="errbox" role="alert">
      <div>{errTitle}</div>
      <div class="acts">
        {#if needSettings}
          <button type="button" onclick={() => openSettings('connectors')}>Buka Pengaturan &gt; Connector</button>
        {/if}
        <button type="button" onclick={retry}>Coba lagi</button>
      </div>
    </div>
  {:else if pipelines.length === 0}
    <p class="muted">Tidak ada pipeline</p>
  {:else}
    {#each pipelines as p (p.id)}
      <PipelineRow pipeline={p} {provider} onOpen={openPipeline} />
    {/each}
  {/if}
</div>

<style>
  .muted { margin: 0; padding: 4px 10px; color: var(--fg-muted, #888); font-size: 14px; }
  .errbox {
    margin: 4px 6px 8px; padding: 8px 10px; background: var(--bg-elev, #222);
    border: 1px solid var(--border, #333); border-radius: 4px; color: var(--danger, #e49a9a);
    font-size: 14px; display: flex; flex-direction: column; gap: 6px;
  }
  .acts { display: flex; gap: 6px; flex-wrap: wrap; }
  .acts button {
    background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333);
    border-radius: 4px; padding: 2px 8px; font-size: 14px; cursor: pointer;
  }
  .acts button:hover { background: var(--bg-hover, #2c2c2c); }
</style>
