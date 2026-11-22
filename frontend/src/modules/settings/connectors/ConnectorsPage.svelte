<script lang="ts">
  import { onMount } from 'svelte';
  import type { ConnectorStatus } from './client';
  import { connectors, connectorsLoading, connectorsError, loadConnectors } from './store';
  import ConnectDialog from './ConnectDialog.svelte';

  onMount(() => {
    void loadConnectors();
  });

  let dialogId = $state<string | null>(null);
  const dialogConnector = $derived($connectors.find((c) => c.id === dialogId) ?? null);

  function badgeText(s: ConnectorStatus): string {
    if (s.state === 'cli_missing') return 'CLI belum terpasang';
    if (s.state === 'not_logged_in') return 'Belum login';
    return s.user ? `Tersambung sebagai ${s.user}` : 'Tersambung';
  }
  function badgeClass(s: ConnectorStatus): string {
    if (s.state === 'cli_missing') return 'is-missing';
    if (s.state === 'not_logged_in') return 'is-pending';
    return 'is-ok';
  }
  function canConnect(s: ConnectorStatus): boolean {
    return s.state === 'cli_missing' || s.state === 'not_logged_in';
  }
</script>

<section>
  <h2>Connector</h2>
  {#if $connectorsLoading && $connectors.length === 0}
    <p class="muted">Memuat...</p>
  {:else if $connectorsError && $connectors.length === 0}
    <div class="error" role="alert">
      <span>{$connectorsError}</span>
      <button type="button" onclick={() => loadConnectors()}>Coba lagi</button>
    </div>
  {:else if $connectors.length === 0}
    <p class="muted">Belum ada connector</p>
  {:else}
    {#if $connectorsError}
      <div class="error" role="alert">
        <span>{$connectorsError}</span>
        <button type="button" onclick={() => loadConnectors()}>Coba lagi</button>
      </div>
    {/if}
    <ul>
      {#each $connectors as c (c.id)}
        <li>
          <div class="info">
            <div class="head">
              <span class="name">{c.name}</span>
              <span class="badge {badgeClass(c.status)}">{badgeText(c.status)}</span>
            </div>
            {#if c.status.message}
              <div class="muted msg">{c.status.message}</div>
            {/if}
          </div>
          {#if canConnect(c.status)}
            <button
              type="button"
              aria-label={`Connect ${c.name}`}
              onclick={() => (dialogId = c.id)}>Connect</button
            >
          {/if}
        </li>
      {/each}
    </ul>
  {/if}

  {#if dialogConnector}
    <ConnectDialog connector={dialogConnector} open={true} onclose={() => (dialogId = null)} />
  {/if}
</section>

<style>
  h2 {
    margin: 0 0 8px;
    font-size: 18px;
    color: var(--fg, #ddd);
  }
  .muted {
    margin: 0;
    color: var(--fg-muted, #888);
    font-size: 15px;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 12px;
    border: 1px solid var(--border, #333);
    background: var(--bg-elev, #222);
    border-radius: 6px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .name {
    color: var(--fg, #ddd);
    font-size: 16px;
  }
  .msg {
    margin-top: 4px;
  }
  .badge {
    font-size: 13px;
    padding: 2px 8px;
    border-radius: 10px;
    border: 1px solid transparent;
  }
  .is-ok {
    background: rgba(60, 160, 90, 0.2);
    border-color: rgba(60, 160, 90, 0.5);
    color: #8fd6a3;
  }
  .is-pending {
    background: rgba(200, 160, 40, 0.2);
    border-color: rgba(200, 160, 40, 0.5);
    color: #e0c470;
  }
  .is-missing {
    background: rgba(200, 70, 70, 0.2);
    border-color: rgba(200, 70, 70, 0.5);
    color: #e49a9a;
  }
  .error {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 8px;
    color: #e49a9a;
    font-size: 15px;
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
</style>
