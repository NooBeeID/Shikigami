<script lang="ts">
  let { path }: { path: string } = $props();
</script>

<div class="notice" role="status">
  <div class="title">File biner tidak dapat ditampilkan.</div>
  <div class="path">{path}</div>
</div>

<style>
  .notice { margin: auto; padding: 16px; text-align: center; color: var(--fg-muted, inherit); }
  .title { font-weight: 600; }
  .path { margin-top: 6px; font-size: 14px; font-family: ui-monospace, monospace; word-break: break-all; }
</style>
