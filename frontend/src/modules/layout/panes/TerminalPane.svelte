<script lang="ts">
  import type { Leaf } from '../model';
  import TerminalTab from '../../terminal/TerminalTab.svelte';

  import { layout } from '../../../lib/stores/layout';

  let { pane, visible }: { pane: Leaf; visible: boolean } = $props();

  // Buang ikon/spinner di awal judul (mis. "✳ ", braille) agar nama stabil.
  function onTitle(raw: string) {
    const t = raw.replace(/^[^\p{L}\p{N}]+/u, '').trim();
    if (t) layout.renamePane(pane.id, t);
  }
</script>

{#if pane.sessionId}
  <TerminalTab sessionId={pane.sessionId} {visible} scrollback={1000} {onTitle} />
{:else}
  <div class="empty">Sesi tidak tersedia</div>
{/if}

<style>
  .empty { padding: 1.25rem; color: var(--fg-muted, #888); }
</style>
