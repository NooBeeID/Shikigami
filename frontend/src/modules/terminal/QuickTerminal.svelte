<script lang="ts">
  import { onMount } from 'svelte';
  import TerminalTab from './TerminalTab.svelte';
  import { createSession, killSession, listSessions } from './terminal-client';

  let activeId = $state<string | null>(null);
  let busy = $state(false);
  let errorMessage = $state('');

  const msg = (e: unknown) => (e instanceof Error ? e.message : String(e));

  onMount(async () => {
    try {
      const list = await listSessions();
      const running = list.filter((s) => s.status === 'running');
      if (running.length) activeId = running[running.length - 1].id;
    } catch (e) {
      errorMessage = msg(e);
    }
  });

  async function newSession() {
    busy = true;
    errorMessage = '';
    try {
      const s = await createSession();
      activeId = s.id;
    } catch (e) {
      errorMessage = 'Gagal membuat sesi: ' + msg(e);
    } finally {
      busy = false;
    }
  }

  async function closeSession() {
    if (activeId === null) return;
    const id = activeId;
    busy = true;
    errorMessage = '';
    try {
      await killSession(id);
      activeId = null;
    } catch (e) {
      errorMessage = 'Gagal menutup sesi: ' + msg(e);
    } finally {
      busy = false;
    }
  }
</script>

<div class="quick">
  <div class="toolbar">
    <button onclick={newSession} disabled={busy}>Sesi baru</button>
    <button onclick={closeSession} disabled={activeId === null || busy}>Tutup sesi</button>
    {#if errorMessage}<span class="err">{errorMessage}</span>{/if}
  </div>
  <div class="area">
    {#if activeId}
      {#key activeId}
        <TerminalTab sessionId={activeId} />
      {/key}
    {:else}
      <p class="empty">Belum ada sesi. Tekan Sesi baru.</p>
    {/if}
  </div>
</div>

<style>
  .quick { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .toolbar {
    display: flex; align-items: center; gap: 0.5rem; height: 2.5rem; flex: none;
    padding: 0 0.5rem; background: #eee; color: #111;
  }
  .area { flex: 1; min-height: 320px; }
  .empty { padding: 1rem; opacity: 0.7; }
  .err { font-size: 0.8rem; color: #c33; }
  @media (prefers-color-scheme: dark) {
    .toolbar { background: #2a2a30; color: #eee; }
  }
</style>
