<script lang="ts">
  import { get } from 'svelte/store';
  import { untrack } from 'svelte';
  import { activeWorkspaceId } from '../../lib/stores/workspaces';
  import { layout } from '../../lib/stores/layout';
  import { openSettings } from '../settings';
  import { parseGitError, type Provider, type Repo, type GitError } from './client';
  import { selectedState, repoKey } from './store';
  import { listMergeRequests, type MergeRequest, type MergeRequestState } from './mrClient';
  import MrRow from './MrRow.svelte';

  let { provider, repo }: { provider: Provider; repo: Repo } = $props();

  let sel = $state<Record<string, MergeRequestState>>({});
  $effect(() => selectedState.subscribe((v) => { sel = v; }));

  let key = $derived(repoKey(repo));
  let curState = $derived<MergeRequestState>(sel[key] ?? 'open');

  let status = $state<'loading' | 'ready' | 'error'>('loading');
  let mrs = $state<MergeRequest[]>([]);
  let err = $state<GitError | null>(null);
  let seq = 0;

  const states: MergeRequestState[] = ['open', 'merged', 'closed'];
  const stateLabel: Record<MergeRequestState, string> = { open: 'Terbuka', merged: 'Digabung', closed: 'Tertutup' };

  function setState(s: MergeRequestState) {
    selectedState.update((m) => ({ ...m, [key]: s }));
  }

  async function load(repoArg: Repo, st: MergeRequestState) {
    const my = ++seq;
    const ws = get(activeWorkspaceId);
    if (ws === null) {
      mrs = [];
      err = null;
      status = 'ready';
      return;
    }
    status = 'loading';
    err = null;
    try {
      const r = await listMergeRequests({ workspaceId: ws, projectId: repoArg.projectId, state: st });
      if (my !== seq) return;
      mrs = r;
      status = 'ready';
    } catch (x) {
      if (my !== seq) return;
      err = parseGitError(x);
      mrs = [];
      status = 'error';
    }
  }

  $effect(() => {
    const r = repo;
    const s = curState;
    untrack(() => { void load(r, s); });
    return () => { seq++; };
  });

  function retry() { void load(repo, curState); }

  function openMr(mr: MergeRequest) {
    layout.openPane({
      kind: 'git-mr',
      provider,
      projectId: repo.projectId,
      number: mr.number,
      title: (provider === 'gitlab' ? '!' : '#') + mr.number + ' ' + mr.title,
    });
  }

  let noun = $derived(provider === 'gitlab' ? 'MR' : 'PR');
  let emptyText = $derived(`Tidak ada ${noun} ${stateLabel[curState].toLowerCase()}`);
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

<div class="mrs">
  <div class="seg" role="group" aria-label="Status {noun}">
    {#each states as s (s)}
      <button type="button" aria-pressed={curState === s} class:on={curState === s} onclick={() => setState(s)}>{stateLabel[s]}</button>
    {/each}
  </div>
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
  {:else if mrs.length === 0}
    <p class="muted">{emptyText}</p>
  {:else}
    {#each mrs as mr (mr.number)}
      <MrRow {mr} {provider} onOpen={openMr} />
    {/each}
  {/if}
</div>

<style>
  .seg { display: inline-flex; margin: 2px 10px 4px; border: 1px solid var(--border, #333); border-radius: 4px; overflow: hidden; }
  .seg button { background: none; border: none; color: var(--fg-muted, #888); font-size: 12px; padding: 2px 8px; cursor: pointer; }
  .seg button + button { border-left: 1px solid var(--border, #333); }
  .seg button.on { background: var(--bg-hover, #2c2c2c); color: var(--fg, #ddd); }
  .seg button:focus-visible { outline: 2px solid var(--accent, #6aa8e8); outline-offset: -2px; }
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
