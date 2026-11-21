<script lang="ts">
  import { groups, mainGroupId, moveSession, sessionGroupMap } from '../../lib/stores/groups';
  import type { SessionMenuItemProps } from '../sidebar-left/registry';

  let { sessionId, onDone }: SessionMenuItemProps = $props();
  let open = $state(false);

  const current = $derived($sessionGroupMap[sessionId] ?? $mainGroupId);

  async function pick(id: string) {
    try {
      await moveSession(sessionId, id);
    } catch {
      // galat tampil lewat groupsError
    }
    onDone?.();
  }

  function onkey(e: KeyboardEvent) {
    if (e.key === 'ArrowRight' && !open) {
      e.preventDefault();
      open = true;
    } else if (e.key === 'Escape' && open) {
      e.preventDefault();
      e.stopPropagation();
      open = false;
    }
  }
</script>

<div class="gsm" onkeydown={onkey} role="presentation">
  <button type="button" role="menuitem" class="head" aria-haspopup="menu" aria-expanded={open} onclick={() => (open = !open)}>
    <span>Pindah ke grup</span><span class="arrow" aria-hidden="true">{open ? '▾' : '▸'}</span>
  </button>
  {#if open}
    <div class="sub" role="menu" aria-label="Pilih grup">
      {#if $groups.length <= 1}
        <div class="none">Tidak ada grup lain</div>
      {:else}
        {#each $groups as g (g.id)}
          {@const cur = g.id === current}
          <button type="button" role="menuitem" disabled={cur} aria-disabled={cur} onclick={() => pick(g.id)}>
            {g.name}{cur ? ' (saat ini)' : ''}
          </button>
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
  .gsm { display: flex; flex-direction: column; gap: 2px; }
  button { text-align: left; font: inherit; font-size: 0.75rem; color: var(--fg); background: transparent; border: 0; border-radius: 0.25rem; padding: 0.3rem 0.5rem; cursor: pointer; }
  button:hover:not(:disabled), button:focus-visible { background: var(--bg-hover); outline: none; }
  button:disabled { opacity: 0.5; cursor: default; }
  .head { display: flex; justify-content: space-between; align-items: center; width: 100%; }
  .arrow { color: var(--fg-muted); }
  .sub { display: flex; flex-direction: column; gap: 2px; padding-left: 0.5rem; border-left: 1px solid var(--border); margin-left: 0.5rem; }
  .none { font-size: 0.75rem; color: var(--fg-muted); padding: 0.3rem 0.5rem; }
</style>
