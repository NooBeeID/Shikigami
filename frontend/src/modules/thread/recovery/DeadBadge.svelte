<script lang="ts">
  import type { Participant } from '../types';

  let { participant }: { participant: Participant } = $props();
</script>

{#if participant.dead === true}
  <span class="badge" role="status" title="Sesi agent tidak hidup; akan dihidupkan lagi saat ada pesan berikutnya">agent mati, akan dihidupkan saat pesan berikutnya</span>
{/if}

<style>
  .badge { display: inline-block; padding: 1px 7px; border-radius: 999px; font-size: 10px; line-height: 1.5; background: rgba(217, 164, 0, 0.14); color: var(--fg, #222); }
</style>
