<script lang="ts">
  import type { Provider } from './client';
  import type { Pipeline } from './pipelineClient';
  import { openInBrowser, copyLink, sendToAgent } from './actions';
  import StatusBadge from './StatusBadge.svelte';

  let { pipeline, provider, onOpen }: { pipeline: Pipeline; provider: Provider; onOpen: (p: Pipeline) => void } = $props();

  function relativeTime(iso: string): string {
    const t = Date.parse(iso);
    if (Number.isNaN(t)) return '';
    const d = Math.max(0, (Date.now() - t) / 1000);
    if (d < 60) return 'baru saja';
    const m = Math.floor(d / 60);
    if (m < 60) return `${m} menit lalu`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h} jam lalu`;
    const days = Math.floor(h / 24);
    if (days < 30) return `${days} hari lalu`;
    const months = Math.floor(days / 30);
    if (months < 12) return `${months} bulan lalu`;
    return `${Math.floor(days / 365) || 1} tahun lalu`;
  }

  function formatDuration(sec?: number | null): string {
    if (sec === undefined || sec === null || Number.isNaN(sec) || sec <= 0) return '-';
    const s = Math.round(sec);
    if (s < 60) return `${s}s`;
    if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
    return `${Math.floor(s / 3600)}j ${Math.floor((s % 3600) / 60)}m`;
  }

  let isTag = $derived(pipeline.ref.startsWith('refs/tags/'));
  let shortSha = $derived(pipeline.sha.slice(0, 7));
  let rel = $derived(relativeTime(pipeline.updatedAt));
  let hasDuration = $derived((pipeline.durationSec ?? 0) > 0);

  function onKey(e: KeyboardEvent) {
    if (e.target !== e.currentTarget) return;
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onOpen(pipeline);
    }
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
<div class="row" role="button" tabindex="0" onclick={() => onOpen(pipeline)} onkeydown={onKey}>
  <div class="line1">
    <span class="glyph" title={isTag ? 'Tag' : 'Branch'}>{isTag ? '◆' : '⑂'}</span>
    <span class="ref" title={pipeline.ref}>{pipeline.ref}</span>
    <StatusBadge status={pipeline.status} kind="pipeline" />
    <span class="acts">
      <button type="button" class="act" aria-label="Buka di browser" title="Buka di browser"
        onclick={(e) => { e.stopPropagation(); openInBrowser(pipeline.url); }}
        onkeydown={(e) => e.stopPropagation()}>↗</button>
      <button type="button" class="act" aria-label="Salin link" title="Salin link"
        onclick={(e) => { e.stopPropagation(); void copyLink(pipeline.url); }}
        onkeydown={(e) => e.stopPropagation()}>⧉</button>
      <button type="button" class="act" aria-label="Kirim ke agen" title="Kirim ke agen"
        onclick={(e) => { e.stopPropagation(); void sendToAgent(pipeline.url); }}
        onkeydown={(e) => e.stopPropagation()}>➤</button>
    </span>
  </div>
  <div class="meta">
    <span class="sha">{shortSha}</span>
    {#if pipeline.author}<span>{pipeline.author}</span>{/if}
    {#if hasDuration}<span>{formatDuration(pipeline.durationSec)}</span>{:else if provider === 'github'}<span>-</span>{/if}
    {#if rel}<span title={pipeline.updatedAt}>{rel}</span>{/if}
  </div>
</div>

<style>
  .row { padding: 4px 10px; cursor: pointer; color: var(--fg, #ddd); border-radius: 4px; }
  .row:hover { background: var(--bg-hover, #2c2c2c); }
  .row:focus-visible { outline: 2px solid var(--accent, #6aa8e8); outline-offset: -2px; }
  .line1 { display: flex; align-items: center; gap: 6px; font-size: 14px; }
  .glyph { color: var(--fg-muted, #888); flex: none; }
  .ref { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .acts { flex: none; display: flex; gap: 2px; }
  .act { background: none; border: none; color: var(--fg-muted, #888); cursor: pointer; font-size: 14px; padding: 0 4px; border-radius: 4px; }
  .act:hover { color: var(--fg, #ddd); background: var(--bg-elev, #222); }
  .act:focus-visible { outline: 2px solid var(--accent, #6aa8e8); }
  .meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; font-size: 12px; color: var(--fg-muted, #888); margin-top: 2px; }
  .sha { font-family: var(--font-mono, monospace); }
</style>
