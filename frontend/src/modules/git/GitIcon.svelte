<script lang="ts">
  import type { Provider } from './client';
  let { provider }: { provider?: Provider } = $props();
</script>

<svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" data-provider={provider}>
  <circle cx="6" cy="5" r="2" />
  <circle cx="6" cy="19" r="2" />
  <circle cx="18" cy="9" r="2" />
  <path d="M6 7v10" />
  <path d="M18 11c0 4-6 3-12 6" />
  {#if provider === 'gitlab'}
    <path d="M14 3l3 3 3-3" />
  {:else if provider === 'github'}
    <path d="M13 20h8" />
  {/if}
</svg>
