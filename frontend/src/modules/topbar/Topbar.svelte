<script lang="ts">
  import { topbarItems } from '../shell';
  import WorkspaceTabs from './WorkspaceTabs.svelte';
  import CenterSummary from './CenterSummary.svelte';
  import SessionCounter from './SessionCounter.svelte';
  import SettingsButton from './SettingsButton.svelte';

  // Store dibuat sekali di level skrip.
  const leftItems = topbarItems('left');
  const centerItems = topbarItems('center');
  const rightItems = topbarItems('right');

  const rightBefore = $derived($rightItems.filter((i) => (i.order ?? 0) < 50));
  const rightAfter = $derived($rightItems.filter((i) => (i.order ?? 0) >= 50));
</script>

<header class="topbar">
  <div class="zone left">
    <WorkspaceTabs />
    {#each $leftItems as it (it.id)}
      {@const C = it.component}
      <div class="slot"><C /></div>
    {/each}
  </div>
  <div class="zone center">
    {#each $centerItems as it (it.id)}
      {@const C = it.component}
      <div class="slot"><C /></div>
    {/each}
    <CenterSummary />
  </div>
  <div class="zone right">
    {#each rightBefore as it (it.id)}
      {@const C = it.component}
      <div class="slot"><C /></div>
    {/each}
    <SessionCounter />
    {#each rightAfter as it (it.id)}
      {@const C = it.component}
      <div class="slot"><C /></div>
    {/each}
    <SettingsButton />
  </div>
</header>

<style>
  .topbar {
    --wails-draggable: drag;
    display: flex;
    height: var(--topbar-h, 38px);
    min-width: 0;
    padding-left: var(--topbar-inset-left, 0px);
    padding-right: 8px;
    background: var(--bg-elev, #222);
    border-bottom: 1px solid var(--border, #444);
    color: var(--fg, inherit);
  }
  .zone { display: flex; align-items: center; min-width: 0; }
  .left { flex: 0 1 auto; max-width: 50%; }
  .center { flex: 1 1 0; justify-content: center; gap: 8px; padding: 0 16px; }
  .right { flex: none; justify-content: flex-end; }
  .slot { --wails-draggable: no-drag; display: inline-flex; align-items: center; }
</style>
