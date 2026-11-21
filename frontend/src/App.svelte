<script lang="ts">
  import { onMount } from 'svelte';
  import { call, inWails } from './lib/ipc';
  import { panels, tabTypes } from './lib/registry';

  let status = $state<'idle' | 'loading' | 'ok' | 'error'>('idle');
  let result = $state<unknown>(null);
  let errorMessage = $state('');

  async function ping() {
    status = 'loading';
    try {
      result = await call('ping');
      status = 'ok';
    } catch (e) {
      errorMessage = e instanceof Error ? e.message : String(e);
      status = 'error';
    }
  }

  onMount(ping);

  const sidebar = panels('sidebar');
  const mainTab = panels('main-tab');
  const statusbar = panels('statusbar');
  const tabs = tabTypes();
</script>

<main>
  <h1>Hello World</h1>
  <p>
    {#if status === 'loading' || status === 'idle'}
      Memeriksa daemon...
    {:else if status === 'ok'}
      pong: {JSON.stringify(result)}
    {:else}
      Galat: {errorMessage}
    {/if}
  </p>
  <button onclick={ping}>Ping ulang</button>
  <p>
    Panel terdaftar: sidebar {$sidebar.length}, main-tab {$mainTab.length}, statusbar {$statusbar.length}, tipe tab {Object.keys($tabs).length}
  </p>
  {#each $mainTab as panel (panel.id)}
    <section class="panel-main-tab"><panel.component /></section>
  {/each}
  <small>{inWails ? 'Mode: Wails' : 'Mode: browser (bukan di dalam Wails)'}</small>
</main>

<style>
  :global(body) {
    margin: 0;
    font-family: system-ui, sans-serif;
    background: #fff;
    color: #111;
  }
  main { padding: 2rem; }
  button { padding: 0.4rem 0.8rem; }
  .panel-main-tab { height: 70vh; margin-top: 1rem; }
  @media (prefers-color-scheme: dark) {
    :global(body) { background: #1b1b1f; color: #eee; }
  }
</style>
