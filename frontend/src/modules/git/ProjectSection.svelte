<script lang="ts">
  import type { Provider, Repo } from './client';
  import { collapsed, isSectionOpen, repoKey, toggleSection } from './store';
  import MrList from './MrList.svelte';
  import PipelineList from './PipelineList.svelte';

  let { provider, repo, index = 0 }: { provider: Provider; repo: Repo; index?: number } = $props();

  let map = $state<Record<string, boolean>>({});
  $effect(() => collapsed.subscribe((v) => { map = v; }));

  let key = $derived(repoKey(repo));
  let open = $derived(isSectionOpen(map, key, index));
  let name = $derived(repo.projects.map((p) => p.name).join(', ') || repo.projectName);
</script>

<div class="group">
  <button type="button" class="head" aria-expanded={open} title={repo.webUrl} onclick={() => toggleSection(key, open)}>
    <span class="chev">{open ? '▾' : '▸'}</span>
    <span class="name">{name} - {repo.path}</span>
  </button>
  {#if open}
    <div class="sub">{provider === 'gitlab' ? 'Merge Request' : 'Pull Request'}</div>
    <MrList {provider} {repo} />
    <div class="sub">{provider === 'gitlab' ? 'Pipeline' : 'Actions'}</div>
    <PipelineList {provider} {repo} />
  {/if}
</div>

<style>
  .head {
    display: flex; align-items: center; gap: 4px; width: 100%;
    background: none; border: none; color: var(--fg, #ddd);
    font-size: 14px; font-weight: 600; text-align: left; padding: 5px 8px; cursor: pointer;
  }
  .head:hover { background: var(--bg-hover, #2c2c2c); }
  .chev { color: var(--fg-muted, #888); width: 10px; flex: none; }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sub { padding: 4px 12px 0; font-size: 12px; letter-spacing: 0.06em; color: var(--fg-muted, #888); }
</style>
