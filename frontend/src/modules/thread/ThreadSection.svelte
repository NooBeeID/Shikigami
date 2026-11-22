<script lang="ts">
  import { activeWorkspaceId } from '../../lib/stores';
  import { statusStore } from '../status/store';
  import { ReadyBadge } from './close';
  import NewThreadDialog from './NewThreadDialog.svelte';
  import { onMount, tick } from 'svelte';
  import { openThreadPane, threadError, threadPanelState, threads } from './thread-store';

  let dialogOpen = $state(false);
  let scroller: HTMLDivElement | undefined = $state();

  function saveScroll(): void {
    if (scroller) threadPanelState.set({ scrollTop: scroller.scrollTop });
  }

  onMount(() => {
    // Pulihkan setelah daftar dirender. Penyimpanan dilakukan lewat onscroll,
    // karena saat unmount elemen bisa sudah lepas dan scrollTop terbaca 0.
    void tick().then(() => {
      if (scroller) scroller.scrollTop = $threadPanelState.scrollTop;
    });
    return () => {
      if (scroller?.isConnected) saveScroll();
    };
  });

  function masterActive(t: (typeof $threads)[number]): boolean {
    const m = t.participants.find((p) => p.role === 'master');
    return !!m && $statusStore[m.sessionId]?.status === 'running';
  }
</script>

<section>
  <div class="head">
    <span class="label">Thread</span>
    <button type="button" class="new" disabled={!$activeWorkspaceId} onclick={() => (dialogOpen = true)}>Thread baru</button>
  </div>
  {#if $threadError}<div class="err">{$threadError}</div>{/if}
  <div class="scroll" bind:this={scroller} onscroll={saveScroll}>
  <ul>
    {#each $threads as t (t.id)}
      {@const active = masterActive(t)}
      <li>
        <button type="button" class="row" class:closed={t.state === 'closed'} onclick={() => openThreadPane(t)}>
          <span class="dot" class:on={active} title={active ? 'aktif' : 'idle'}></span>
          <span class="title">{t.title}</span>
          <ReadyBadge thread={t} />
          {#if t.state === 'closed'}<span class="tag">ditutup</span>{/if}
        </button>
      </li>
    {:else}
      <li class="empty">Belum ada thread</li>
    {/each}
  </ul>
  </div>
  <NewThreadDialog open={dialogOpen} onclose={() => (dialogOpen = false)} />
</section>

<style>
  section { display: flex; flex-direction: column; height: 100%; min-height: 0; padding: 8px 12px; box-sizing: border-box; }
  .scroll { flex: 1; min-height: 0; overflow-y: auto; }
  .head { flex: none; display: flex; align-items: center; justify-content: space-between; padding: 0 8px; }
  .label { font-size: 13px; color: var(--fg-muted, #888); text-transform: uppercase; letter-spacing: 0.04em; }
  .new { font: inherit; font-size: 13px; color: var(--fg, #222); background: transparent; border: 1px solid var(--border, #8884); border-radius: 4px; padding: 2px 8px; cursor: pointer; }
  .new:hover:not(:disabled) { background: var(--bg-hover, #0001); }
  .new:disabled { opacity: 0.5; cursor: default; }
  ul { list-style: none; margin: 4px 0 0; padding: 0; }
  .row { display: flex; align-items: center; gap: 6px; width: 100%; text-align: left; font: inherit; font-size: 14px; color: var(--fg, #222); background: transparent; border: 0; border-radius: 4px; padding: 5px 8px; cursor: pointer; min-width: 0; }
  .row:hover { background: var(--bg-hover, #0001); }
  .row.closed { opacity: 0.5; }
  .dot { width: 7px; height: 7px; border-radius: 50%; background: var(--fg-muted, #888); flex: none; }
  .dot.on { background: var(--ok, #30a46c); }
  .title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tag { font-size: 12px; color: var(--fg-muted, #888); flex: none; }
  .empty { padding: 4px 8px; font-size: 14px; color: var(--fg-muted, #888); }
  .err { flex: none; padding: 2px 8px; font-size: 13px; color: var(--danger, #e5484d); }
</style>
