<script lang="ts">
  import { toast } from './toast-store';
</script>

{#if $toast}
  <span class="toast" class:error={$toast.kind === 'error'} role="status" aria-live="polite">{$toast.text}</span>
{/if}

<style>
  .toast {
    font-size: 14px;
    white-space: nowrap;
    max-width: 420px;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--fg-muted, #8b949e);
  }
  .error {
    color: #e5534b;
  }
</style>
