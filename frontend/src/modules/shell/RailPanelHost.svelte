<script lang="ts">
  import { railItems } from './registry';
  import { leftSidebar, setActiveRail, resolveActiveRail, DEFAULT_RAIL } from '../../lib/stores/shell';

  const items = railItems();
  const activeId = $derived(resolveActiveRail($items, $leftSidebar.activeRail));
  const Active = $derived($items.find((i) => i.id === activeId)?.component);

  $effect(() => {
    const cur = $leftSidebar.activeRail;
    if ($items.length === 0 || !cur) return;
    if (cur !== DEFAULT_RAIL && !$items.some((i) => i.id === cur)) setActiveRail(DEFAULT_RAIL);
  });
</script>

{#key activeId}
  {#if Active}<Active />{/if}
{/key}
