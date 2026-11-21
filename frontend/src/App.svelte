<script lang="ts">
  import './styles/theme.css';
  import './styles/base.css';
  import { onMount, onDestroy } from 'svelte';
  import { connection, initStores } from './lib/stores';
  import LayoutArea from './modules/layout/LayoutArea.svelte';
  import { ShellFrame, RightPanel } from './modules/shell';
  import './modules/shell/register';
  import Topbar from './modules/topbar/Topbar.svelte';
  import AgentSidebar from './modules/sidebar-left/AgentSidebar.svelte';
  import Palette from './modules/palette/Palette.svelte';
  import Toast from './modules/openfolder/Toast.svelte';

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
  <ShellFrame>
    {#snippet topbar()}<Topbar />{/snippet}
    {#snippet left()}<AgentSidebar />{/snippet}
    {#snippet center()}<LayoutArea />{/snippet}
    {#snippet right()}<RightPanel />{/snippet}
  </ShellFrame>
  <Palette />
  <div class="toast-host"><Toast /></div>
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
  .toast-host { position: fixed; right: 12px; bottom: 12px; z-index: 50; pointer-events: none; }
</style>
