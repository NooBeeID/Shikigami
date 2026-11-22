<script lang="ts">
  import { railItems } from './registry';
  import RailBadge from './RailBadge.svelte';
  import { leftSidebar, setActiveRail, resolveActiveRail } from '../../lib/stores/shell';

  const items = railItems();
  let badged = $state<Record<string, boolean>>({});

  function setBadged(id: string, on: boolean) {
    if (!!badged[id] === on) return;
    badged = { ...badged, [id]: on };
  }

  const active = $derived(resolveActiveRail($items, $leftSidebar.activeRail));
</script>

<div class="rail">
  {#each $items as spec (spec.id)}
    {@const Icon = spec.icon}
    <button
      type="button"
      class="rail-btn"
      class:active={spec.id === active}
      aria-label={badged[spec.id] ? `${spec.title} (ada sesi menunggu)` : spec.title}
      title={spec.title}
      aria-current={spec.id === active ? 'true' : undefined}
      onclick={() => setActiveRail(spec.id)}
    >
      <Icon />
      {#if spec.badge}
        <RailBadge badge={spec.badge} onchange={(on) => setBadged(spec.id, on)} />
      {/if}
    </button>
  {/each}
</div>
