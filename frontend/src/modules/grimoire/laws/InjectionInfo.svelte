<script lang="ts">
  import { injectionInfo } from './lawsStore';
  let { projectId }: { projectId: string } = $props();
</script>

{#if $injectionInfo && $injectionInfo.exists}
  <div class="inject-info" data-project={projectId} title="Estimasi kasar (~4 karakter per token)">
    Disuntik ke sesi baru: {$injectionInfo.indexBytes} byte, ~{$injectionInfo.estTokens} token
  </div>
{/if}

<style>
  .inject-info { font-size: 13px; color: var(--fg-muted); padding: 2px 12px; }
</style>
