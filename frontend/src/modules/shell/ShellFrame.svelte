<script lang="ts">
  import { onMount } from 'svelte';
  import type { Snippet } from 'svelte';
  import './shell.css';
  import { leftSidebar, rightSidebar } from '../../lib/stores/shell';
  import Resizer from './Resizer.svelte';
  import Rail from './Rail.svelte';
  import RailPanelHost from './RailPanelHost.svelte';
  import LayoutHeaderHost from './LayoutHeaderHost.svelte';
  import { installShellShortcuts } from './shortcuts';

  let { topbar, left, center, right }: { topbar: Snippet; left?: Snippet; center: Snippet; right: Snippet } = $props();

  onMount(() => installShellShortcuts());
</script>

<div
  class="shell-frame"
  style="--left-w:{$leftSidebar.visible ? $leftSidebar.width : 0}px; --right-w:{$rightSidebar.visible ? $rightSidebar.width : 0}px"
>
  <header class="topbar-slot">{@render topbar()}</header>
  <div class="body">
    <nav class="rail-slot" aria-label="Menu utama"><Rail /></nav>
    <aside class="side left" class:hidden={!$leftSidebar.visible}>
      {#if left}{@render left()}{:else}<RailPanelHost />{/if}
      <Resizer side="left" />
    </aside>
    <main class="center">
      <LayoutHeaderHost />
      <div class="center-body">{@render center()}</div>
    </main>
    <aside class="side right">
      {@render right()}
      <Resizer side="right" />
    </aside>
  </div>
</div>

<style>
  .topbar-slot { position: relative; z-index: 30; min-width: 0; min-height: 0; overflow: visible; }
  .body {
    display: grid;
    grid-template-columns: var(--rail-w) var(--left-w) minmax(0, 1fr) var(--right-w);
    min-width: 0;
    min-height: 0;
    overflow: hidden;
  }
  .side { position: relative; overflow: hidden; min-width: 0; min-height: 0; background: var(--bg-elev); }
  .rail-slot { width: 48px; min-width: 0; min-height: 0; overflow: hidden; background: var(--bg-elev); border-right: 1px solid var(--border); }
  .side.left { border-right: 1px solid var(--border); }
  .side.left.hidden { border-right: 0; }
  .side.right { border-left: 1px solid var(--border); }
  .center { min-width: 0; min-height: 0; overflow: hidden; display: grid; grid-template-rows: auto minmax(0, 1fr); }
  .center-body { min-width: 0; min-height: 0; overflow: hidden; }
</style>
