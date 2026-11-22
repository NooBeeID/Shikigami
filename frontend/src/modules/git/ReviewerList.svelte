<script lang="ts">
  import type { MrReviewer } from './mrDetailClient';

  let { reviewers }: { reviewers: MrReviewer[] } = $props();

  function info(state: string): { label: string; tone: string; glyph: string } {
    if (state === 'approved') return { label: 'Disetujui', tone: 'good', glyph: '✓' };
    if (state === 'changes_requested') return { label: 'Diminta perubahan', tone: 'bad', glyph: '✕' };
    return { label: 'Belum', tone: 'warn', glyph: '◌' };
  }
</script>

{#if reviewers.length === 0}
  <p class="muted">Belum ada reviewer</p>
{:else}
  <ul>
    {#each reviewers as r, i (r.name + i)}
      {@const s = info(r.state)}
      <li>
        <span class="name">{r.name}</span>
        <span class="state {s.tone}"><span aria-hidden="true">{s.glyph}</span> {s.label}</span>
      </li>
    {/each}
  </ul>
{/if}

<style>
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
  li { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; font-size: 14px; }
  .name { min-width: 0; overflow-wrap: anywhere; }
  .state { flex: none; font-size: 12px; }
  .good { color: var(--success, #6fbf73); }
  .bad { color: var(--danger, #e49a9a); }
  .warn { color: var(--warn, #d9b35f); }
  .muted { margin: 0; color: var(--fg-muted, #888); font-size: 14px; }
</style>
