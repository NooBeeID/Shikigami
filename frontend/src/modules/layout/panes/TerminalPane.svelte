<script lang="ts">
  import { tick } from 'svelte';
  import type { Tab } from '../model';
  import TerminalTab from '../../terminal/TerminalTab.svelte';

  import { layout } from '../../../lib/stores/layout';

  let { pane, paneId, visible }: { pane: Tab; paneId: string; visible: boolean } = $props();

  // Buang ikon/spinner di awal judul (mis. "✳ ", braille) agar nama stabil.
  function onTitle(raw: string) {
    const t = raw.replace(/^[^\p{L}\p{N}]+/u, '').trim();
    if (t) layout.renameTab(paneId, pane.id, t);
  }

  // Pengaman: saat tab jadi terlihat lagi, picu resize (tidak mengubah layout/fokus).
  // Jalur utama refit: TerminalTab.applyVisible(true) + ResizeObserver saat host dipasang ulang.
  let wasVisible = false;
  $effect(() => {
    const v = visible;
    if (v && !wasVisible) {
      void tick().then(() =>
        requestAnimationFrame(() =>
          requestAnimationFrame(() => window.dispatchEvent(new Event('resize'))),
        ),
      );
    }
    wasVisible = v;
  });
</script>

{#if pane.sessionId}
  {#key pane.sessionId}
    <TerminalTab sessionId={pane.sessionId} {visible} scrollback={1000} {onTitle} />
  {/key}
{:else}
  <div class="empty">Sesi tidak tersedia</div>
{/if}

<style>
  .empty { padding: 1.25rem; color: var(--fg-muted, #888); }
</style>
