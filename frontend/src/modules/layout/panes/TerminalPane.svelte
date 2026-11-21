<script lang="ts">
  import type { Leaf } from '../model';
  import TerminalTab from '../../terminal/TerminalTab.svelte';

  let { pane, visible }: { pane: Leaf; visible: boolean } = $props();
</script>

{#if pane.sessionId}
  <TerminalTab sessionId={pane.sessionId} {visible} scrollback={1000} />
{:else}
  <div class="empty">Sesi tidak tersedia</div>
{/if}

<style>
  .empty { padding: 0.75rem; color: var(--fg-muted, #888); }
</style>
