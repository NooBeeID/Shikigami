<script lang="ts">
  let { building }: { building: boolean } = $props();
</script>

{#if building}
  <span class="build-indicator" role="status" aria-live="polite"><span class="pulse" aria-hidden="true"></span>Sedang membangun...</span>
{/if}

<style>
  .build-indicator {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--fg-muted, inherit);
  }
  .pulse {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent, currentColor);
    animation: stale-pulse 1.2s ease-in-out infinite;
  }
  @keyframes stale-pulse {
    0%, 100% { opacity: 0.35; }
    50% { opacity: 1; }
  }
  @media (prefers-reduced-motion: reduce) {
    .pulse { animation: none; }
  }
</style>
