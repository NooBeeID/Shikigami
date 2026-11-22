<script lang="ts">
  import type { Thread, ThreadMessage } from '../types';

  let { message }: { thread: Thread; message?: ThreadMessage } = $props();
  const paused = $derived(message?.kind === 'system' && message.text.startsWith('Thread dijeda'));
</script>

{#if paused}
  <div class="callout" role="note">
    <strong>Dijeda</strong>
    <span>Kirim pesan sebagai user atau master untuk melanjutkan.</span>
  </div>
{/if}

<style>
  .callout { margin: 2px 8px 6px; padding: 4px 8px; border-left: 3px solid #d9a400; background: rgba(217, 164, 0, 0.14); border-radius: 4px; font-size: 11px; color: var(--fg, #222); display: flex; gap: 6px; flex-wrap: wrap; }
</style>
