<script lang="ts">
  import type { PipelineStage } from './pipelineClient';
  import { openInBrowser, copyLink, sendToAgent } from './actions';
  import StatusBadge from './StatusBadge.svelte';

  let { stages }: { stages: PipelineStage[] } = $props();

  function formatDuration(sec?: number | null): string {
    if (sec === undefined || sec === null || !(sec > 0)) return '-';
    const s = Math.round(sec);
    if (s < 60) return `${s}s`;
    if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
    return `${Math.floor(s / 3600)}j ${Math.floor((s % 3600) / 60)}m`;
  }
</script>

<div class="stages">
  {#if stages.length === 0}
    <p class="muted">Tidak ada stage</p>
  {:else}
    {#each stages as stage, i (stage.name + ':' + i)}
      {@const jobs = stage.jobs ?? []}
      <section class="stage">
        <div class="stage-head">
          <span class="stage-name">{stage.name}</span>
          <span class="count">{jobs.length} job</span>
        </div>
        {#if jobs.length === 0}
          <p class="muted">Tidak ada job</p>
        {:else}
          {#each jobs as job, j (job.name + ':' + j)}
            <div class="job">
              {#if job.url}
                <button type="button" class="link" title={job.name} onclick={() => openInBrowser(job.url!)}>{job.name}</button>
              {:else}
                <span class="name" title={job.name}>{job.name}</span>
              {/if}
              <StatusBadge status={job.status} kind="pipeline" />
              <span class="dur">{formatDuration(job.durationSec)}</span>
              <span class="acts">
                <button type="button" class="act" aria-label="Buka di browser" title={job.url ? 'Buka di browser' : 'Job ini tidak punya link'}
                  disabled={!job.url} onclick={() => openInBrowser(job.url!)}>↗</button>
                <button type="button" class="act" aria-label="Salin link" title={job.url ? 'Salin link' : 'Job ini tidak punya link'}
                  disabled={!job.url} onclick={() => void copyLink(job.url!)}>⧉</button>
                <button type="button" class="act" aria-label="Kirim ke agen" title={job.url ? 'Kirim ke agen' : 'Job ini tidak punya link'}
                  disabled={!job.url} onclick={() => void sendToAgent(job.url!)}>➤</button>
              </span>
            </div>
          {/each}
        {/if}
      </section>
    {/each}
  {/if}
</div>

<style>
  .stage { margin-bottom: 8px; }
  .stage-head { display: flex; align-items: baseline; gap: 8px; padding: 4px 12px; background: var(--bg-elev, #222); border-bottom: 1px solid var(--border, #333); font-size: 14px; }
  .stage-name { font-weight: 600; color: var(--fg, #ddd); }
  .count { font-size: 12px; color: var(--fg-muted, #888); }
  .job { display: flex; align-items: center; gap: 8px; padding: 3px 12px; font-size: 14px; color: var(--fg, #ddd); }
  .job:hover { background: var(--bg-hover, #2c2c2c); }
  .name, .link { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; text-align: left; }
  .link { background: none; border: none; padding: 0; font: inherit; color: var(--accent, #6aa8e8); cursor: pointer; }
  .link:hover { text-decoration: underline; }
  .link:focus-visible { outline: 2px solid var(--accent, #6aa8e8); outline-offset: 1px; }
  .dur { flex: none; font-size: 12px; color: var(--fg-muted, #888); min-width: 52px; text-align: right; }
  .acts { flex: none; display: flex; gap: 2px; }
  .act { background: none; border: none; color: var(--fg-muted, #888); cursor: pointer; font-size: 14px; padding: 0 4px; border-radius: 4px; }
  .act:hover:not(:disabled) { color: var(--fg, #ddd); background: var(--bg-elev, #222); }
  .act:focus-visible { outline: 2px solid var(--accent, #6aa8e8); }
  .act:disabled { opacity: .4; cursor: not-allowed; }
  .muted { margin: 0; padding: 4px 12px; color: var(--fg-muted, #888); font-size: 14px; }
</style>
