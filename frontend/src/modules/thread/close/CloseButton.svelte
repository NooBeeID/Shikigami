<script lang="ts">
  import type { Thread } from '../types';
  import CloseDialog from './CloseDialog.svelte';
  import { closeDialogFor, openCloseDialog } from './closeDialogStore';

  let { thread }: { thread: Thread } = $props();
</script>

{#if thread.state === 'active'}
  <button type="button" class="btn" title="Tutup thread ini" onclick={() => openCloseDialog(thread.id)}>Tandai selesai</button>
{/if}
<CloseDialog {thread} open={$closeDialogFor === thread.id} />

<style>
  .btn { font: inherit; font-size: 12px; color: var(--fg, #222); background: transparent; border: 1px solid var(--border, #8884); border-radius: 6px; padding: 2px 10px; cursor: pointer; }
  .btn:hover { background: var(--bg-hover, #0001); }
</style>
