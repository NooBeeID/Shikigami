<script lang="ts">
  import { railItems } from './registry';
  import { leftSidebar, setActiveRail, resolveActiveRail } from '../../lib/stores/shell';

  const items = railItems();
  const active = $derived(resolveActiveRail($items, $leftSidebar.activeRail));
</script>

<div class="rail">
  {#each $items as spec (spec.id)}
    {@const Icon = spec.icon}
    <button
      type="button"
      class="rail-btn"
      class:active={spec.id === active}
      aria-label={spec.title}
      title={spec.title}
      aria-current={spec.id === active ? 'true' : undefined}
      onclick={() => setActiveRail(spec.id)}
    >
      <Icon />
    </button>
  {/each}
</div>
