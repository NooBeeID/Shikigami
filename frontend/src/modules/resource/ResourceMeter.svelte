<script lang="ts">
  import { resourceStore, type Proc } from './store';
  import { formatBytes, formatCpu, rowNames, rowKey } from './format';
  import { inWails } from '../../lib/ipc';
  import { sessions } from '../../lib/stores/sessions';
  import { sessionTitle } from '../shell/helpers';

  let open = $state(false);
  let root: HTMLDivElement | undefined = $state();

  const snap = $derived($resourceStore.snapshot);
  const summary = $derived(snap ? `CPU ${formatCpu(snap.totalCpuPercent)} · ${formatBytes(snap.totalRssBytes)}` : '–');

  // Sesi memakai judul yang sama dengan sidebar/Activity; id pendek hanya bila judul kembar.
  const names = $derived(
    rowNames(snap?.procs ?? [], (id) => ($sessions.some((x) => x.id === id) ? sessionTitle(id) : undefined)),
  );
  const nameOf = (p: Proc) => names.get(rowKey(p)) ?? p.label;

  $effect(() => {
    if (!open) return;
    const down = (e: PointerEvent) => {
      if (root && !root.contains(e.target as Node)) open = false;
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') open = false;
    };
    window.addEventListener('pointerdown', down);
    window.addEventListener('keydown', key);
    return () => {
      window.removeEventListener('pointerdown', down);
      window.removeEventListener('keydown', key);
    };
  });
</script>

{#if inWails}
  <div class="wrap" bind:this={root}>
    <button type="button" class="meter" aria-haspopup="true" aria-expanded={open} title="Pemakaian resource" onclick={() => (open = !open)}>{summary}</button>
    {#if open}
      <div class="menu" role="dialog" aria-label="Pemakaian resource">
        {#if snap}
          {#each snap.procs as p (rowKey(p))}
            <div class="row">
              <span class="name" title={nameOf(p)}>{nameOf(p)}</span>
              <span class="num">{formatBytes(p.rssBytes)}</span>
              <span class="num">{formatCpu(p.cpuPercent)}</span>
            </div>
          {/each}
        {:else}
          <div class="row"><span class="name">–</span></div>
        {/if}
        {#if $resourceStore.error}<div class="err">{$resourceStore.error}</div>{/if}
        <div class="foot">Diperbarui tiap 5 detik · RSS termasuk memori shared</div>
      </div>
    {/if}
  </div>
{/if}

<style>
  .wrap { --wails-draggable: no-drag; position: relative; display: inline-flex; align-items: center; padding: 0 6px; }
  .meter {
    padding: 4px 8px; background: transparent; border-color: transparent; color: var(--fg-muted, #888);
    font-size: 13px; font-variant-numeric: tabular-nums; border-radius: 4px; cursor: pointer; --wails-draggable: no-drag;
  }
  .meter:hover, .meter[aria-expanded='true'] { background: var(--bg-hover, #333); color: var(--fg, inherit); }
  .menu {
    position: absolute; top: calc(100% + 4px); right: 0; z-index: 1000; width: 320px; max-height: 60vh; overflow-y: auto; padding: 4px;
    background: var(--bg-elev, #222); border: 1px solid var(--border, #444); border-radius: 6px; box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
    --wails-draggable: no-drag;
  }
  .row { display: flex; align-items: center; gap: 12px; padding: 5px 8px; font-size: 14px; color: var(--fg, inherit); }
  .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .num { flex: none; min-width: 52px; text-align: right; font-variant-numeric: tabular-nums; color: var(--fg-muted, #888); }
  .err { padding: 4px 8px; font-size: 13px; color: var(--danger, #f85149); }
  .foot { margin-top: 4px; padding: 6px 8px 4px; border-top: 1px solid var(--border, #444); font-size: 12px; color: var(--fg-muted, #888); }
</style>
