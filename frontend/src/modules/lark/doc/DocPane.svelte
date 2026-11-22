<script lang="ts">
  // Placeholder fondasi; isi ditimpa tiket lark-docs. Props jangan diubah.
  import type { Leaf } from '../../layout/model';
  import { watchSource, type SourceStatus } from '../source';

  let { pane, visible }: { pane: Leaf; visible: boolean } = $props();
  let status = $state<SourceStatus | 'checking'>('checking');

  $effect(() => watchSource(() => pane.sourceId, (s) => (status = s)));
</script>

<div class="lark-pane" hidden={!visible}>
  {#if status === 'missing'}
    <p>Sumber tidak ada di workspace ini</p>
  {:else if status === 'error'}
    <p>Tidak dapat memeriksa sumber</p>
  {:else}
    <h3>Dokumen Lark</h3>
    <p>{pane.title}</p>
    <p>Segera hadir</p>
  {/if}
</div>

<style>
  .lark-pane { flex: 1; min-height: 0; padding: 0.75rem; background: var(--bg); color: var(--fg-muted); }
  .lark-pane[hidden] { display: none; }
  h3 { margin: 0 0 0.5rem; }
</style>
