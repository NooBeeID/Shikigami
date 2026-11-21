<script lang="ts">
  import { onMount } from 'svelte';
  import type { Snippet } from 'svelte';
  import './shell.css';
  import { leftSidebar, rightSidebar } from '../../lib/stores/shell';
  import Resizer from './Resizer.svelte';
  import { installShellShortcuts } from './shortcuts';

  let { topbar, left, center, right }: { topbar: Snippet; left: Snippet; center: Snippet; right: Snippet } = $props();

  onMount(() => installShellShortcuts());
</script>

<div
  class="shell-frame"
  style="--left-w:{$leftSidebar.visible ? $leftSidebar.width : 0}px; --right-w:{$rightSidebar.visible ? $rightSidebar.width : 0}px"
>
  <header class="topbar-slot">{@render topbar()}</header>
  <div class="body">
    <aside class="side left">
      {@render left()}
      <Resizer side="left" />
    </aside>
    <main class="center">{@render center()}</main>
    <aside class="side right">
      {@render right()}
      <Resizer side="right" />
    </aside>
  </div>
</div>

<style>
  .topbar-slot { min-width: 0; min-height: 0; overflow: hidden; }
  .body {
    display: grid;
    grid-template-columns: var(--left-w) minmax(0, 1fr) var(--right-w);
    min-width: 0;
    min-height: 0;
    overflow: hidden;
  }
  .side { position: relative; overflow: hidden; min-width: 0; min-height: 0; background: var(--bg-elev); }
  .side.left { border-right: 1px solid var(--border); }
  .side.right { border-left: 1px solid var(--border); }
  .center { min-width: 0; min-height: 0; overflow: hidden; }
</style>
