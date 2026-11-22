<script lang="ts">
  import type { Readable } from 'svelte/store';

  let { badge, onchange }: { badge: Readable<number | boolean>; onchange?: (on: boolean) => void } = $props();

  let on = $state(false);

  function isOn(v: number | boolean): boolean {
    return v === true || (typeof v === 'number' && v > 0);
  }

  // Subscribe hanya lewat store; tanpa timer. Cleanup: unsubscribe + laporkan nonaktif saat unmount.
  $effect(() => {
    const store = badge;
    const unsub = store.subscribe((v) => {
      on = isOn(v);
      onchange?.(on);
    });
    return () => {
      unsub();
      onchange?.(false);
    };
  });
</script>

{#if on}
  <span class="rail-badge" aria-hidden="true"></span>
{/if}

<style>
  .rail-badge {
    position: absolute; top: 6px; right: 6px; width: 9px; height: 9px;
    border-radius: 50%; background: var(--danger, #e5484d);
    pointer-events: none;
  }
</style>
