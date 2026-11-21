<script lang="ts">
  import type { Leaf } from '../model';
  import FileTab from '../../files/editor/FileTab.svelte';

  let { pane }: { pane: Leaf; visible: boolean } = $props();

  const refId = $derived(
    JSON.stringify({ path: pane.filePath ?? '', projectId: pane.projectId ?? '', workspaceId: pane.workspaceId ?? '' }),
  );
</script>

{#if pane.filePath}
  <FileTab {refId} />
{:else}
  <div class="empty">File tidak tersedia</div>
{/if}

<style>
  .empty { padding: 0.75rem; color: var(--fg-muted, #888); }
</style>
