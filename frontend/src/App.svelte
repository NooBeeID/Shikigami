<script lang="ts">
  import './styles/theme.css';
  import './styles/base.css';
  import { onMount, onDestroy } from 'svelte';
  import { connection, initStores } from './lib/stores';
  import LayoutArea from './modules/layout/LayoutArea.svelte';
  import SidebarArea from './modules/shell/SidebarArea.svelte';
  import StatusBar from './modules/shell/StatusBar.svelte';

  let dispose: (() => void) | null = null;

  async function start() {
    dispose?.();
    dispose = null;
    dispose = await initStores();
  }

  onMount(() => {
    void start();
  });
  onDestroy(() => dispose?.());
</script>

{#if $connection.state === 'error'}
  <div class="center">
    <h2>Tidak bisa terhubung ke daemon</h2>
    <p class="msg">{$connection.message}</p>
    <button onclick={start}>Coba lagi</button>
  </div>
{:else if $connection.state === 'connecting'}
  <div class="center"><p>Menghubungkan ke daemon...</p></div>
{:else}
  <div class="shell">
    <div class="side"><SidebarArea /></div>
    <main class="main">
      <div class="content"><LayoutArea /></div>
    </main>
    <div class="status"><StatusBar /></div>
  </div>
{/if}

<style>
  .center {
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    text-align: center;
  }
  .msg { color: var(--fg-muted); max-width: 60ch; margin: 0; }
  h2 { margin: 0; }
  .shell {
    height: 100%;
    display: grid;
    grid-template-columns: var(--sidebar-w) 1fr;
    grid-template-rows: 1fr var(--statusbar-h);
  }
  .side { grid-column: 1; grid-row: 1 / span 2; min-height: 0; }
  .main {
    grid-column: 2;
    grid-row: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .content { flex: 1; min-height: 0; overflow: hidden; }
  .status { grid-column: 2; grid-row: 2; min-width: 0; }
</style>
