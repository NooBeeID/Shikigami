<script lang="ts">
  import type { Session } from '../../lib/stores';
  import { killSession, providers } from '../../lib/stores';
  import { layout } from '../../lib/stores/layout';
  import { focusOrOpenSession, sessionTitle } from '../shell';
  import StatusBadge from '../status/StatusBadge.svelte';
  import { errMessage } from '../workspace/errors';
  import { sessionMenuItems } from './registry';

  const menuItems = sessionMenuItems();

  let { session, inLayout, onerror }: { session: Session; inLayout: boolean; onerror: (msg: string) => void } = $props();

  let menuOpen = $state(false);
  let confirming = $state(false);
  let root: HTMLElement | undefined = $state();

  // Baca layout dan providers agar judul ikut reaktif.
  const title = $derived.by(() => {
    void $layout;
    void $providers;
    return sessionTitle(session.id);
  });
  const exited = $derived(session.status === 'exited');
  const badge = $derived(
    session.providerId === 'claude' ? 'CL'
      : session.providerId === 'codex' ? 'CX'
      : session.providerId === 'shell' ? 'SH'
      : session.providerId.slice(0, 2).toUpperCase(),
  );

  $effect(() => {
    if (!menuOpen) return;
    const onDown = (e: PointerEvent) => {
      if (root && !root.contains(e.target as Node)) closeMenu();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeMenu();
    };
    window.addEventListener('pointerdown', onDown);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('pointerdown', onDown);
      window.removeEventListener('keydown', onKey);
    };
  });

  function closeMenu() {
    menuOpen = false;
    confirming = false;
  }

  function openIt() {
    if (!focusOrOpenSession(session.id)) onerror('Sesi tidak ditemukan');
  }

  async function kill() {
    try {
      await killSession(session.id);
    } catch (e) {
      onerror(errMessage(e));
    }
    closeMenu();
  }
</script>

<div class="row" class:exited bind:this={root} data-session-id={session.id}>
  <button type="button" class="main" onclick={openIt} title={title}>
    <StatusBadge sessionId={session.id} compact />
    <span class="title">{title}</span>
    <span class="prov">{badge}</span>
    {#if inLayout}<span class="in" title="Tampil di layout" aria-label="Tampil di layout"></span>{/if}
  </button>
  <button
    type="button"
    class="more"
    aria-label="Menu sesi"
    aria-haspopup="menu"
    aria-expanded={menuOpen}
    onclick={(e) => { e.stopPropagation(); menuOpen = !menuOpen; confirming = false; }}
  >…</button>
  {#if menuOpen}
    <div class="menu" role="menu">
      {#if confirming}
        <div class="ask">Akhiri sesi ini?</div>
        <div class="btns">
          <button type="button" class="danger" onclick={kill}>Akhiri</button>
          <button type="button" onclick={() => (confirming = false)}>Batal</button>
        </div>
      {:else}
        <button type="button" role="menuitem" onclick={() => { openIt(); closeMenu(); }}>Buka di layout</button>
        {#each $menuItems as It (It.id)}
          <It.component sessionId={session.id} onDone={closeMenu} />
        {/each}
        <button type="button" role="menuitem" disabled={exited} onclick={() => (confirming = true)}>Akhiri sesi</button>
      {/if}
    </div>
  {/if}
</div>

<style>
  .row { position: relative; display: flex; align-items: center; min-width: 0; border-radius: 6px; margin-bottom: 2px; }
  .row:hover { background: var(--bg-hover, #0001); }
  .row.exited { opacity: 0.5; }
  .main { flex: 1; min-width: 0; display: flex; align-items: center; gap: 10px; text-align: left; font: inherit; font-size: 15px; color: var(--fg, #222); background: transparent; border: 0; padding: 10px 8px 10px 22px; cursor: pointer; }
  .title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .prov { font-size: 12px; color: var(--fg-muted, #888); border: 1px solid var(--border, #8884); border-radius: 3px; padding: 1px 5px; flex: none; }
  .in { width: 6px; height: 6px; border-radius: 50%; background: var(--accent, #3b82f6); flex: none; }
  .more { flex: none; font: inherit; color: var(--fg-muted, #888); background: transparent; border: 0; padding: 4px 10px; margin-right: 4px; cursor: pointer; border-radius: 4px; }
  .more:hover { color: var(--fg, #222); }
  .menu { position: absolute; top: 100%; right: 0; z-index: 20; min-width: 140px; padding: 4px; background: var(--bg-elev, #fff); border: 1px solid var(--border, #8884); border-radius: 6px; box-shadow: 0 6px 18px #0004; display: flex; flex-direction: column; gap: 2px; }
  .menu button { text-align: left; font: inherit; font-size: 14px; color: var(--fg, #222); background: transparent; border: 0; border-radius: 4px; padding: 5px 8px; cursor: pointer; }
  .menu button:hover:not(:disabled) { background: var(--bg-hover, #0001); }
  .menu button:disabled { opacity: 0.4; cursor: default; }
  .ask { font-size: 14px; padding: 4px 8px; }
  .btns { display: flex; gap: 4px; }
  .menu .btns button { flex: 1; text-align: center; border: 1px solid var(--border, #8884); }
  .menu .btns .danger { color: var(--danger, #e5484d); border-color: var(--danger, #e5484d); }
</style>
