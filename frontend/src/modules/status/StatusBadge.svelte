<script lang="ts">
  import { statusOf, type Status } from './store';

  let { sessionId, compact = false }: { sessionId: string; compact?: boolean } = $props();

  const labels: Record<Status, string> = {
    running: 'Berjalan',
    waiting: 'Menunggu',
    done: 'Selesai',
    idle: 'Diam',
  };

  const st = $derived(statusOf(sessionId));
  const status = $derived($st);
  const label = $derived(labels[status]);
</script>

<span class="badge" role="status" aria-label={label} title={label}>
  <span class="dot {status}"></span>
  {#if !compact}<span class="text">{label}</span>{/if}
</span>

<style>
  .badge { display: inline-flex; align-items: center; gap: 5px; font-size: 13px; color: var(--fg-muted, #8b949e); }
  .dot { width: 8px; height: 8px; border-radius: 50%; flex: none; }
  .running { background: var(--accent, #4f8cff); animation: pulse 1.4s ease-in-out infinite; }
  .waiting { background: #e5b13a; }
  .done { background: var(--ok, #3fb950); }
  .idle { background: var(--fg-muted, #8b949e); }
  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.5; transform: scale(0.8); }
  }
  @media (prefers-reduced-motion: reduce) {
    .running { animation: none; }
  }
</style>
