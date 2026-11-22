<script lang="ts">
  import { activeWorkspace, activeProjectId, providers } from '../../lib/stores';
  import { createAndOpenSession, openFolder } from '../shell';
  import { errMessage } from '../workspace/errors';

  let { onerror }: { onerror: (msg: string) => void } = $props();

  let open = $state(false);
  let busy = $state(false);
  let selectedProjectId = $state('');
  let root: HTMLElement | undefined = $state();

  const noWorkspace = $derived(!$activeWorkspace);
  const projects = $derived($activeWorkspace?.projects ?? []);

  // Tanpa workspace popup tidak boleh terbuka.
  $effect(() => {
    if (noWorkspace) open = false;
  });

  $effect(() => {
    if (!open) return;
    selectedProjectId = projects.some((p) => p.id === $activeProjectId) ? $activeProjectId! : (projects[0]?.id ?? '');
  });

  $effect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (root && !root.contains(e.target as Node)) open = false;
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') open = false;
    };
    window.addEventListener('pointerdown', onDown);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('pointerdown', onDown);
      window.removeEventListener('keydown', onKey);
    };
  });

  async function start(providerId: string) {
    if (busy || noWorkspace) return;
    busy = true;
    try {
      await createAndOpenSession(providerId, selectedProjectId || undefined);
      open = false;
    } catch (e) {
      onerror(errMessage(e));
    } finally {
      busy = false;
    }
  }
</script>

<div class="wrap" bind:this={root}>
  {#if open && !noWorkspace}
    <div class="pop">
      {#if projects.length === 0}
        <div class="dim">Belum ada project</div>
        <button type="button" class="item" onclick={() => { openFolder(); open = false; }}>Open Folder…</button>
      {:else}
        <label class="lbl" for="ns-project">Project</label>
        <select id="ns-project" bind:value={selectedProjectId} disabled={busy}>
          {#each projects as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
        </select>
        <div class="lbl">Provider</div>
        {#each $providers as p (p.id)}
          <button type="button" class="item" disabled={busy || p.available === false} title={p.available === false ? 'Tidak tersedia' : p.name} onclick={() => start(p.id)}>{p.name}</button>
        {/each}
      {/if}
    </div>
  {/if}
  <button type="button" class="main" aria-haspopup="menu" aria-expanded={open} disabled={noWorkspace} title={noWorkspace ? 'Buka atau buat workspace dulu' : undefined} onclick={() => (open = !open)}>+ New Session</button>
</div>

<style>
  .wrap { position: relative; padding: 14px 16px; border-top: 1px solid var(--border, #8884); }
  .main { width: 100%; font: inherit; font-size: 14px; color: var(--fg, #222); background: transparent; border: 1px solid var(--accent, #3b82f6); border-radius: 6px; padding: 10px 14px; cursor: pointer; }
  .main:hover { background: var(--bg-hover, #0001); }
  .pop { position: absolute; bottom: 100%; left: 8px; right: 8px; z-index: 20; margin-bottom: 2px; padding: 6px; display: flex; flex-direction: column; gap: 3px; max-height: 300px; overflow: auto; background: var(--bg-elev, #fff); border: 1px solid var(--border, #8884); border-radius: 6px; box-shadow: 0 6px 18px #0004; }
  .lbl { font-size: 13px; color: var(--fg-muted, #888); padding: 2px 2px 0; }
  .dim { font-size: 14px; color: var(--fg-muted, #888); padding: 4px 2px; }
  select { font: inherit; font-size: 14px; min-width: 0; color: var(--fg, #222); background: var(--bg-elev, transparent); border: 1px solid var(--border, #8884); border-radius: 4px; padding: 3px 6px; }
  .item { text-align: left; font: inherit; font-size: 14px; color: var(--fg, #222); background: transparent; border: 0; border-radius: 4px; padding: 5px 6px; cursor: pointer; }
  .item:hover:not(:disabled) { background: var(--bg-hover, #0001); }
  .item:disabled, .main:disabled { opacity: 0.5; cursor: default; }
</style>
