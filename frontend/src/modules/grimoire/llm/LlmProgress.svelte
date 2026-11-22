<script lang="ts">
  import type { LlmProgress as LlmProgressData } from './llmStore';

  let { building, progress }: { building: boolean; progress: LlmProgressData | null } = $props();
</script>

{#if building && progress}
  <span class="llm-progress" role="status" aria-live="polite"><span class="pulse" aria-hidden="true"></span>
    {#if progress.current}<span class="current">Meringkas {progress.current} ({progress.done} selesai{#if progress.failed > 0}, {progress.failed} gagal{/if})</span>{:else}<span class="current">Meringkas modul ({progress.done} selesai...)</span>{/if}
  </span>
{/if}

<style>
  .llm-progress {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    font-size: 13px;
    color: var(--fg-muted, inherit);
  }
  .current {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pulse {
    flex: none;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent, currentColor);
    animation: llm-pulse 1.2s ease-in-out infinite;
  }
  @keyframes llm-pulse {
    0%, 100% { opacity: 0.35; }
    50% { opacity: 1; }
  }
  @media (prefers-reduced-motion: reduce) {
    .pulse { animation: none; }
  }
</style>
