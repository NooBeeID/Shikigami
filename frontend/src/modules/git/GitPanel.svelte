<script lang="ts">
  import type { Provider, Repo } from './client';
  import { gitState, refreshRepos, reposFor, repoKey, type GitState } from './store';
  import { openSettings } from '../settings';
  import ProjectSection from './ProjectSection.svelte';

  let { provider }: { provider: Provider } = $props();

  let st = $state<GitState>({ status: 'loading' });
  let repos = $state<Repo[]>([]);

  $effect(() => gitState.subscribe((v) => { st = v; }));
  $effect(() => reposFor(provider).subscribe((v) => { repos = v; }));

  function reload() {
    if (repos.length === 0) gitState.set({ status: 'loading' });
    void refreshRepos();
  }

  let label = $derived(provider === 'gitlab' ? 'GitLab' : 'GitHub');
  let errTitle = $derived.by(() => {
    if (st.status !== 'error') return '';
    switch (st.code) {
      case 'cli_missing': return st.message || `${provider === 'gitlab' ? 'glab' : 'gh'} belum terpasang`;
      case 'not_logged_in': return 'Belum login';
      case 'permission': return 'Izin kurang';
      case 'timeout': return 'Waktu habis';
      default: return st.message || 'Terjadi galat';
    }
  });
  let needSettings = $derived(st.status === 'error' && (st.code === 'cli_missing' || st.code === 'not_logged_in'));
</script>

<section class="git" aria-label={label}>
  <div class="header">
    <span class="title">{label.toUpperCase()}</span>
    <button class="icon" type="button" title="Segarkan" aria-label="Segarkan" onclick={reload}>&#8635;</button>
  </div>
  <div class="body">
    {#if st.status === 'loading' && repos.length === 0}
      <p class="muted">Memuat repo...</p>
    {/if}
    {#if st.status === 'error'}
      <div class="errbox" role="alert">
        <div>{errTitle}</div>
        <div class="acts">
          {#if needSettings}
            <button type="button" onclick={() => openSettings('connectors')}>Buka Pengaturan &gt; Connector</button>
          {/if}
          <button type="button" onclick={reload}>Coba lagi</button>
        </div>
      </div>
    {/if}
    {#if st.status === 'ready' && repos.length === 0}
      <p class="muted">Tidak ada project dengan remote {label} di workspace ini</p>
    {/if}
    {#each repos as repo, i (repoKey(repo))}
      <ProjectSection {provider} {repo} index={i} />
    {/each}
    <p class="muted foot">Bila daftar belum muncul, pastikan CLI sudah login di Pengaturan &gt; Connector.</p>
  </div>
</section>

<style>
  .git { display: flex; flex-direction: column; height: 100%; min-height: 0; color: var(--fg, #ddd); }
  .header { display: flex; align-items: center; justify-content: space-between; padding: 8px 10px; }
  .title { font-size: 13px; letter-spacing: 0.08em; color: var(--fg-muted, #888); }
  .icon { background: none; border: none; color: var(--fg-muted, #888); cursor: pointer; font-size: 16px; }
  .icon:hover { color: var(--fg, #ddd); }
  .body { flex: 1; min-height: 0; overflow: auto; padding: 0 4px 8px; }
  .muted { margin: 0; padding: 6px 10px; color: var(--fg-muted, #888); font-size: 14px; }
  .foot { font-size: 12px; margin-top: 8px; }
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
