<script lang="ts">
  import { onDestroy } from 'svelte';
  import Modal from '../Modal.svelte';
  import { getConnectorGuide, type ConnectorGuide, type ConnectorInfo, type ConnectorStatus } from './client';
  import { refreshConnector } from './store';

  let {
    connector,
    open,
    onclose,
  }: { connector: ConnectorInfo; open: boolean; onclose: () => void } = $props();

  let guide = $state<ConnectorGuide | null>(null);
  let guideLoading = $state(false);
  let guideError = $state<string | null>(null);
  let checking = $state(false);
  let checkError = $state<string | null>(null);
  let copiedIndex = $state<number | null>(null);
  let copyError = $state<string | null>(null);
  let success = $state(false);
  let closeTimer: ReturnType<typeof setTimeout> | undefined;

  function clearCloseTimer() {
    if (closeTimer !== undefined) {
      clearTimeout(closeTimer);
      closeTimer = undefined;
    }
  }

  function badgeText(s: ConnectorStatus): string {
    if (s.state === 'cli_missing') return 'CLI belum terpasang';
    if (s.state === 'not_logged_in') return 'Belum login';
    return s.user ? `Tersambung sebagai ${s.user}` : 'Tersambung';
  }

  $effect(() => {
    const id = connector.id;
    if (!open) return;
    success = false;
    checkError = null;
    copyError = null;
    guide = null;
    guideError = null;
    guideLoading = true;
    let cancelled = false;
    getConnectorGuide(id)
      .then((g) => {
        if (!cancelled) guide = g;
      })
      .catch((e) => {
        if (!cancelled) guideError = e instanceof Error ? e.message : String(e);
      })
      .finally(() => {
        if (!cancelled) guideLoading = false;
      });
    return () => {
      cancelled = true;
      clearCloseTimer();
    };
  });

  onDestroy(clearCloseTimer);

  function close() {
    clearCloseTimer();
    onclose();
  }

  async function copy(cmd: string, i: number) {
    try {
      await navigator.clipboard.writeText(cmd);
      copiedIndex = i;
      copyError = null;
      setTimeout(() => {
        if (copiedIndex === i) copiedIndex = null;
      }, 1500);
    } catch {
      copyError = 'Gagal menyalin ke clipboard. Salin perintah secara manual.';
    }
  }

  async function recheck() {
    if (checking) return;
    checking = true;
    checkError = null;
    try {
      const s = await refreshConnector(connector.id);
      if (s.state === 'connected') {
        success = true;
        clearCloseTimer();
        closeTimer = setTimeout(close, 1200);
      }
    } catch (e) {
      checkError = e instanceof Error ? e.message : String(e);
    } finally {
      checking = false;
    }
  }
</script>

<Modal {open} onclose={close} title={`Hubungkan ${connector.name}`}>
  <div class="body">
    <p class="status">
      Status: <strong>{badgeText(connector.status)}</strong>
      {#if connector.status.message}
        <span class="muted"> - {connector.status.message}</span>
      {/if}
    </p>

    {#if guideLoading}
      <p class="muted">Memuat panduan...</p>
    {:else if guideError}
      <p class="error" role="alert">{guideError}</p>
    {:else if guide}
      {#if guide.steps.length > 0}
        <ol>
          {#each guide.steps as step}
            <li>{step}</li>
          {/each}
        </ol>
      {/if}
      {#if guide.commands.length > 0}
        {#each guide.commands as cmd, i}
          <div class="cmd">
            <pre><code>{cmd}</code></pre>
            <button type="button" onclick={() => copy(cmd, i)}>
              {copiedIndex === i ? 'Tersalin' : 'Salin'}
            </button>
          </div>
        {/each}
        {#if copyError}
          <p class="error" role="alert">{copyError}</p>
        {/if}
      {/if}
    {/if}

    {#if success}
      <p class="ok" role="status">
        Berhasil tersambung{connector.status.user ? ` sebagai ${connector.status.user}` : ''}
      </p>
    {:else if checkError}
      <p class="error" role="alert">{checkError}</p>
    {/if}

    <div class="actions">
      <button type="button" onclick={recheck} disabled={checking || success}>
        {checking ? 'Memeriksa...' : 'Cek ulang'}
      </button>
      <button type="button" onclick={close}>Tutup</button>
    </div>
  </div>
</Modal>

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: 10px;
    color: var(--fg, #ddd);
    font-size: 15px;
    min-width: 0;
  }
  p {
    margin: 0;
  }
  .muted {
    color: var(--fg-muted, #888);
  }
  .error {
    color: #e49a9a;
  }
  .ok {
    color: #8fd6a3;
  }
  ol {
    margin: 0;
    padding-left: 20px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .cmd {
    display: flex;
    align-items: flex-start;
    gap: 8px;
  }
  pre {
    flex: 1;
    min-width: 0;
    margin: 0;
    padding: 8px 10px;
    background: var(--bg-elev, #222);
    border: 1px solid var(--border, #333);
    border-radius: 4px;
    overflow-x: auto;
    font-family: ui-monospace, Menlo, monospace;
    font-size: 14px;
    white-space: pre-wrap;
    word-break: break-all;
  }
  .actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }
  button {
    background: var(--bg-elev, #222);
    color: var(--fg, #ddd);
    border: 1px solid var(--border, #333);
    border-radius: 4px;
    padding: 4px 12px;
    font-size: 15px;
    cursor: pointer;
  }
  button:hover:not(:disabled) {
    background: var(--bg-hover, #2c2c2c);
  }
  button:disabled {
    opacity: 0.5;
    cursor: default;
  }
</style>
