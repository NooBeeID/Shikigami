<script lang="ts">
  // Pembungkus atas renderer files yang sudah aman (tanpa HTML mentah, href dibatasi). Jangan tambah {@html}.
  import FilesMarkdownView from '../../files/viewer/MarkdownView.svelte';
  let { source }: { source: string } = $props();
</script>

<div class="doc-markdown">
  <FilesMarkdownView {source} />
</div>

<style>
  .doc-markdown { flex: 1; min-height: 0; min-width: 0; overflow: auto; }
</style>
