<script lang="ts">
  import { formatTokens, formatDuration, type LlmCost as LlmCostData } from './llmStore';

  let { cost }: { cost: LlmCostData | null } = $props();
</script>

{#if cost}
  <div class="llm-cost">
    <div class="main" title={cost.at || undefined}>
      Build terakhir: {cost.modules} modul, <span title="Perkiraan, bukan tagihan">~{formatTokens(cost.estTokens)} token</span>, {formatDuration(cost.durationMs)}{#if cost.failed > 0}<span class="failed">, {cost.failed} gagal</span>{/if}{#if cost.skipped}, {cost.skipped} dilewati{/if}
    </div>
    {#if cost.note}<div class="note">{cost.note}</div>{/if}
  </div>
{/if}

<style>
  .llm-cost {
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 13px;
    color: var(--fg-muted, inherit);
    overflow-wrap: anywhere;
  }
  .failed {
    color: var(--danger, inherit);
  }
  .note {
    color: var(--fg-muted, inherit);
  }
</style>
