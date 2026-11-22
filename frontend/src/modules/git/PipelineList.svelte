<script lang="ts">
  import type { Provider, Repo } from './client';
  let { provider, repo }: { provider: Provider; repo: Repo } = $props();
</script>

<p class="muted" data-repo={repo.path}>Segera hadir ({provider === 'gitlab' ? 'Pipeline' : 'Actions'})</p>

<style>
  .muted { margin: 0; padding: 4px 10px; color: var(--fg-muted, #888); font-size: 14px; }
</style>
