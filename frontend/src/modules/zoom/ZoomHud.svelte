<script lang="ts">
  import { zoomPercent } from './index';

  const HIDE_MS = 1500;
  let visible = $state(false);
  let first = true;
  let timer: ReturnType<typeof setTimeout> | undefined;

  // Tampilkan persen tiap kali zoom berubah (abaikan nilai awal saat mount).
  $effect(() => {
    void $zoomPercent;
    if (first) {
      first = false;
      return;
    }
    visible = true;
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => (visible = false), HIDE_MS);
  });
</script>

{#if visible}
  <div class="hud" role="status" aria-live="polite">{$zoomPercent}%</div>
{/if}

<style>
  .hud {
    position: fixed;
    left: 50%;
    bottom: 48px;
    transform: translateX(-50%);
    z-index: 2000;
    padding: 8px 18px;
    font-size: 20px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--fg);
    background: var(--bg-elev);
    border: 1px solid var(--border);
    border-radius: 10px;
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.35);
    pointer-events: none;
  }
</style>
