<script lang="ts">
  import { getTicket, parseLarkError } from './boardClient';
  import type { TicketSource, TicketDetail, LarkError } from './boardClient';
  import { copyLink, sendToAgent } from '../actions';
  import { activeWorkspaceId } from '../../../lib/stores/workspaces';

  let { source, tableId, viewId, recordId, onClose }: {
    source: TicketSource;
    tableId: string;
    viewId?: string;
    recordId: string;
    onClose: () => void;
  } = $props();

  let status = $state<'loading' | 'ready' | 'error'>('loading');
  let detail = $state<TicketDetail | null>(null);
  let error = $state<LarkError | null>(null);
  let retryTick = $state(0);
  let busy = $state(false);
  let closeBtn = $state<HTMLButtonElement | null>(null);
  let seq = 0;

  $effect(() => {
    const sourceId = source.id;
    const tbl = tableId;
    const view = viewId;
    const rec = recordId;
    const workspaceId = $activeWorkspaceId;
    void retryTick;

    const mine = ++seq;
    detail = null;
    error = null;
    status = 'loading';

    if (!rec) {
      error = { code: 'invalid_params', message: 'Tiket tidak valid' };
      status = 'error';
      return () => { seq++; };
    }
    if (!workspaceId) {
      error = { code: 'invalid_params', message: 'Workspace aktif tidak ditemukan' };
      status = 'error';
      return () => { seq++; };
    }

    getTicket({ workspaceId, sourceId, recordId: rec, tableId: tbl, viewId: view || undefined })
      .then((d) => {
        if (mine !== seq) return;
        detail = d;
        status = 'ready';
      })
      .catch((e) => {
        if (mine !== seq) return;
        error = parseLarkError(e);
        status = 'error';
      });

    return () => { seq++; };
  });

  $effect(() => {
    closeBtn?.focus();
  });

  const canLink = $derived(detail?.recordLinkAvailable === true && !!detail?.url);

  function onWindowKey(e: KeyboardEvent) {
    if (e.key !== 'Escape' || e.defaultPrevented) return;
    e.stopPropagation();
    onClose();
  }

  async function run(fn: (url: string) => Promise<void>) {
    if (busy || !canLink || !detail?.url) return;
    busy = true;
    try {
      await fn(detail.url);
    } catch {
      /* actions sudah menampilkan toast; abaikan */
    } finally {
      busy = false;
    }
  }
  const onCopy = () => run(copyLink);
  const onSend = () => run(sendToAgent);
</script>

<svelte:window onkeydown={onWindowKey} />

<div class="drawer" role="dialog" aria-modal="false" aria-label="Detail tiket" tabindex="-1">
  <header class="head">
    <h2>Detail tiket</h2>
    <button
      type="button"
      class="close"
      aria-label="Tutup detail tiket"
      bind:this={closeBtn}
      onclick={() => onClose()}
    >&#10005;</button>
  </header>

  <div class="body">
    {#if status === 'loading'}
      <p class="msg" role="status" aria-live="polite">Memuat tiket...</p>
    {:else if status === 'error'}
      <div class="msg err" role="alert">
        <p>{error?.message || 'Gagal memuat tiket'}</p>
        <button type="button" class="btn" onclick={() => { retryTick++; }}>Coba lagi</button>
      </div>
    {:else if detail}
      {#if detail.fields.length === 0}
        <p class="msg">Tiket ini tidak punya isi untuk ditampilkan</p>
      {:else}
        <dl>
          {#each detail.fields as f, i (i + ':' + f.name)}
            <dt>{f.name}</dt>
            <dd>{f.text}</dd>
          {/each}
        </dl>
      {/if}
    {/if}
  </div>

  {#if status === 'ready'}
    <footer class="foot">
      {#if detail?.recordLinkAvailable === false}
        <p class="hint" id="ticket-link-hint">Link tiket tidak bisa dipastikan</p>
      {/if}
      <div class="actions">
        <button
          type="button"
          class="btn"
          disabled={!canLink || busy}
          aria-describedby={detail?.recordLinkAvailable === false ? 'ticket-link-hint' : undefined}
          onclick={onCopy}
        >Salin link</button>
        <button
          type="button"
          class="btn"
          disabled={!canLink || busy}
          aria-describedby={detail?.recordLinkAvailable === false ? 'ticket-link-hint' : undefined}
          onclick={onSend}
        >Kirim ke agen</button>
      </div>
    </footer>
  {/if}
</div>

<style>
  .drawer {
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    width: min(420px, 100%);
    z-index: 5;
    display: flex;
    flex-direction: column;
    color: var(--fg, inherit);
    background: var(--bg-elev, var(--bg));
    border-left: 1px solid var(--border, transparent);
    box-shadow: -4px 0 16px color-mix(in srgb, black 25%, transparent);
    font-size: 14px;
    animation: slide-in 150ms ease-out;
    outline: none;
  }
  @keyframes slide-in {
    from { transform: translateX(100%); }
    to { transform: translateX(0); }
  }
  @media (prefers-reduced-motion: reduce) {
    .drawer { animation: none; }
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    flex: none;
    border-bottom: 1px solid var(--border, transparent);
  }
  h2 { margin: 0; font-size: 15px; font-weight: 600; }
  .close, .btn {
    background: none;
    border: 1px solid transparent;
    color: inherit;
    font: inherit;
    cursor: pointer;
    min-height: 28px;
    min-width: 28px;
    padding: 0 10px;
    border-radius: 4px;
  }
  .btn { border-color: var(--border, currentColor); background: var(--bg-elev, transparent); }
  .close:hover, .btn:hover:not(:disabled) {
    background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent));
  }
  .close:focus-visible, .btn:focus-visible { outline: 2px solid var(--accent, currentColor); outline-offset: 1px; }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .body { flex: 1; min-height: 0; overflow: auto; padding: 12px; }
  .msg { color: var(--fg-muted, inherit); margin: 0; }
  .err { color: var(--danger, inherit); display: flex; flex-direction: column; gap: 8px; align-items: flex-start; }
  .err p { margin: 0; }
  dl { margin: 0; display: flex; flex-direction: column; gap: 10px; }
  dt { color: var(--fg-muted, inherit); font-size: 13px; font-weight: 600; }
  dd { margin: 2px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; }
  .foot {
    flex: none;
    padding: 8px 12px;
    border-top: 1px solid var(--border, transparent);
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .hint { margin: 0; color: var(--fg-muted, inherit); }
  .actions { display: flex; gap: 8px; }
</style>
