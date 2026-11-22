<script lang="ts">
  import type { Tab } from '../model';
  import ReadOnlyFileView from './ReadOnlyFileView.svelte';
  import FileTab from '../../files/editor/FileTab.svelte';

  let { pane }: { pane: Tab; paneId?: string; visible?: boolean } = $props();

  const refId = $derived(
    JSON.stringify({ path: pane.filePath ?? '', projectId: pane.projectId ?? '', workspaceId: pane.workspaceId ?? '' }),
  );
</script>

{#if pane.readOnly && pane.root && pane.filePath}
  <ReadOnlyFileView root={pane.root} path={pane.filePath} projectId={pane.projectId} />
{:else if pane.filePath}
  <FileTab {refId} />
{:else}
  <div class="empty">File tidak tersedia</div>
{/if}

<style>
  .empty { padding: 1.25rem; color: var(--fg-muted, #888); }
</style>
