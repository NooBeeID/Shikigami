<script lang="ts">
  import type { Thread, ThreadMessage } from '../types';
  import { recoveryKind } from './util';

  let { message }: { thread: Thread; message?: ThreadMessage } = $props();
  const kind = $derived(recoveryKind(message));
  const hint = $derived(
    kind === 'dead'
      ? 'Agent akan dihidupkan lagi otomatis saat ada pesan berikutnya.'
      : kind === 'respawn'
        ? 'Agent baru memakai ringkasan log thread sebagai konteks.'
        : kind === 'failed'
          ? 'Kirim pesan lagi untuk mencoba ulang.'
          : ''
  );
</script>

{#if kind !== null}
  <div class="callout" class:failed={kind === 'failed'} role="note">
    <span>{hint}</span>
  </div>
{/if}

<style>
  .callout { margin: 2px 8px 6px; padding: 4px 8px; border-left: 3px solid #3b82c4; background: rgba(59, 130, 196, 0.12); border-radius: 4px; font-size: 13px; color: var(--fg, #222); display: flex; gap: 6px; flex-wrap: wrap; }
  .callout.failed { border-left-color: #c0392b; background: rgba(192, 57, 43, 0.12); }
</style>
