<script lang="ts">
  import { workspaces, activeWorkspaceId, activeWorkspace, sessions, selectWorkspace } from '../../lib/stores';
  import { errMessage } from '../workspace/errors';

  let { onerror }: { onerror: (msg: string) => void } = $props();

  let open = $state(false);
  let root: HTMLElement | undefined = $state();

  const projectCount = $derived($activeWorkspace?.projects.length ?? 0);
  const sessionCount = $derived($sessions.filter((s) => s.workspaceId === $activeWorkspaceId).length);

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

  async function pick(id: string) {
    open = false;
    try {
      await selectWorkspace(id);
    } catch (e) {
      onerror(errMessage(e));
    }
  }
</script>

<div class="wrap" bind:this={root}>
  <button type="button" class="head" aria-haspopup="listbox" aria-expanded={open} onclick={() => (open = !open)}>
    <span class="text">
      <span class="name">{$activeWorkspace?.name ?? 'Tanpa workspace'}</span>
      <span class="sub">{projectCount} project · {sessionCount} sesi</span>
    </span>
    <span class="chev" aria-hidden="true">▾</span>
  </button>
  {#if open}
    <ul class="menu" role="listbox">
      {#each $workspaces as w (w.id)}
        <li role="option" aria-selected={w.id === $activeWorkspaceId}>
          <button type="button" class:active={w.id === $activeWorkspaceId} onclick={() => pick(w.id)}>
            <span class="mark">{w.id === $activeWorkspaceId ? '✓' : ''}</span>{w.name}
          </button>
        </li>
      {:else}
        <li class="empty">Belum ada workspace</li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .wrap { position: relative; min-width: 0; }
  .head { display: flex; align-items: center; justify-content: space-between; gap: 6px; width: 100%; text-align: left; font: inherit; color: var(--fg, #222); background: transparent; border: 1px solid transparent; border-radius: 6px; padding: 12px 20px; cursor: pointer; }
  .head:hover { background: var(--bg-hover, #0001); }
  .text { display: flex; flex-direction: column; min-width: 0; }
  .name { font-weight: 600; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sub { font-size: 11px; color: var(--fg-muted, #888); }
  .chev { color: var(--fg-muted, #888); font-size: 11px; flex: none; }
  .menu { position: absolute; top: 100%; left: 0; right: 0; z-index: 20; margin: 2px 0 0; padding: 4px; list-style: none; max-height: 240px; overflow: auto; background: var(--bg-elev, #fff); border: 1px solid var(--border, #8884); border-radius: 6px; box-shadow: 0 6px 18px #0004; }
  .menu button { display: flex; align-items: center; gap: 4px; width: 100%; text-align: left; font: inherit; font-size: 12px; color: var(--fg, #222); background: transparent; border: 0; border-radius: 4px; padding: 5px 6px; cursor: pointer; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .menu button:hover { background: var(--bg-hover, #0001); }
  .menu button.active { color: var(--accent, #3b82f6); font-weight: 600; }
  .mark { display: inline-block; width: 12px; flex: none; }
  .empty { padding: 6px; font-size: 12px; color: var(--fg-muted, #888); }
</style>
