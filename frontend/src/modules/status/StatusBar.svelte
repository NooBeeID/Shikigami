<script lang="ts">
  import { sessions } from '../../lib/stores';
  import { statusStore, type Status } from './store';

  const counts = $derived.by(() => {
    const c: Record<Status, number> = { running: 0, waiting: 0, done: 0, idle: 0 };
    let total = 0;
    for (const s of $sessions) {
      if (s.status !== 'running') continue;
      total++;
      c[$statusStore[s.id]?.status ?? 'idle']++;
    }
    return { c, total };
  });

  const text = $derived.by(() => {
    const { c } = counts;
    const parts: string[] = [];
    if (c.running > 0) parts.push(`${c.running} berjalan`);
    if (c.waiting > 0) parts.push(`${c.waiting} menunggu`);
    if (c.done > 0) parts.push(`${c.done} selesai`);
    if (c.idle > 0) parts.push(`${c.idle} diam`);
    return parts.join(', ');
  });
</script>

<span class="summary" aria-live="polite">{#if counts.total > 0}{text}{/if}</span>

<style>
  .summary { font-size: 12px; color: var(--fg-muted, #8b949e); white-space: nowrap; }
</style>
