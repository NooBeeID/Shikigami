<script lang="ts">
  import type { Provider } from './client';
  import type { MergeRequest } from './mrClient';
  import { openInBrowser, copyLink, sendToAgent } from './actions';
  import StatusBadge from './StatusBadge.svelte';

  let { mr, provider, onOpen }: { mr: MergeRequest; provider: Provider; onOpen: (mr: MergeRequest) => void } = $props();

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

  let prefix = $derived(provider === 'gitlab' ? '!' : '#');
  let rel = $derived(relativeTime(mr.updatedAt));

  function onKey(e: KeyboardEvent) {
    if (e.target !== e.currentTarget) return;
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onOpen(mr);
    }
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
<div class="row" role="button" tabindex="0" onclick={() => onOpen(mr)} onkeydown={onKey}>
  <div class="line1">
    <span class="num">{prefix}{mr.number}</span>
    <span class="title" title={mr.title}>{mr.title}</span>
    {#if mr.draft}<span class="draft">Draft</span>{/if}
    <span class="acts">
      <button type="button" class="act" aria-label="Buka di browser" title="Buka di browser"
        onclick={(e) => { e.stopPropagation(); openInBrowser(mr.url); }}
        onkeydown={(e) => e.stopPropagation()}>↗</button>
      <button type="button" class="act" aria-label="Salin link" title="Salin link"
        onclick={(e) => { e.stopPropagation(); void copyLink(mr.url); }}
        onkeydown={(e) => e.stopPropagation()}>⧉</button>
      <button type="button" class="act" aria-label="Kirim ke agen" title="Kirim ke agen"
        onclick={(e) => { e.stopPropagation(); void sendToAgent(mr.url); }}
        onkeydown={(e) => e.stopPropagation()}>➤</button>
    </span>
  </div>
  <div class="meta">
    <span>{mr.author}</span>
    <span class="branches" title="{mr.sourceBranch} → {mr.targetBranch}">{mr.sourceBranch} → {mr.targetBranch}</span>
    {#if mr.reviewState}<StatusBadge kind="review" status={mr.reviewState} />{/if}
    {#if mr.pipelineStatus}<StatusBadge kind="pipeline" status={mr.pipelineStatus} />{/if}
    {#if rel}<span title={mr.updatedAt}>{rel}</span>{/if}
  </div>
</div>

<style>
  .row { padding: 4px 10px; cursor: pointer; color: var(--fg, #ddd); border-radius: 4px; }
  .row:hover { background: var(--bg-hover, #2c2c2c); }
  .row:focus-visible { outline: 2px solid var(--accent, #6aa8e8); outline-offset: -2px; }
  .line1 { display: flex; align-items: center; gap: 6px; font-size: 14px; }
  .num { color: var(--fg-muted, #888); flex: none; }
  .title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .draft { flex: none; font-size: 12px; color: var(--fg-muted, #888); border: 1px solid var(--border, #333); border-radius: 8px; padding: 0 6px; }
  .acts { flex: none; display: flex; gap: 2px; }
  .act { background: none; border: none; color: var(--fg-muted, #888); cursor: pointer; font-size: 14px; padding: 0 4px; border-radius: 4px; }
  .act:hover { color: var(--fg, #ddd); background: var(--bg-elev, #222); }
  .act:focus-visible { outline: 2px solid var(--accent, #6aa8e8); }
  .meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; font-size: 12px; color: var(--fg-muted, #888); margin-top: 2px; }
  .branches { min-width: 0; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
