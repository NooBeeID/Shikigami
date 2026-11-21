<script lang="ts">
  import type { ActivityEntry } from './types';

  let { entry, name, alive, onclick }: {
    entry: ActivityEntry;
    name: string;
    alive: boolean;
    onclick: () => void;
  } = $props();

  const p2 = (n: number) => String(n).padStart(2, '0');
  const time = $derived.by(() => {
    const d = new Date(entry.ts);
    return `${p2(d.getHours())}:${p2(d.getMinutes())}:${p2(d.getSeconds())}`;
  });
</script>

<button type="button" class="row" class:dead={!alive} disabled={!alive} {onclick}>
  <span class="time">{time}</span>
  <span class="name">{name}</span>
  <span class="tool">{entry.tool}</span>
  <span class="summary" title={entry.summary}>{entry.summary || '-'}</span>
</button>

<style>
  .row {
    display: flex; align-items: center; gap: 6px; width: 100%;
    padding: 3px 8px; border: 0; background: transparent; color: var(--fg, inherit);
    font: inherit; font-size: 12px; text-align: left; cursor: pointer;
  }
  .row:hover:not(.dead) { background: var(--bg-hover, rgba(128, 128, 128, 0.15)); }
  .row.dead { opacity: 0.5; cursor: default; }
  .time { font-family: ui-monospace, monospace; color: var(--fg-muted, #888); flex: none; }
  .name { max-width: 80px; flex: none; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tool {
    flex: none; padding: 0 5px; font-size: 11px; border-radius: 3px;
    background: var(--bg-elev, transparent); border: 1px solid var(--border, #444);
  }
  .summary { flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: var(--fg-muted, #888); }
</style>
