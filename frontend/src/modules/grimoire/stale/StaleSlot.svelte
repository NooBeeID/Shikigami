<script module lang="ts">
  export { staleState, isChapterStale, isBookStale } from './staleStore';
</script>

<script lang="ts">
  import { staleState, startStaleStore, type StaleState } from './staleStore';
  import BuildIndicator from './BuildIndicator.svelte';
  import UpdateButton from './UpdateButton.svelte';

  let { projectId, exists }: { projectId: string | null; exists: boolean } = $props();

  let s = $state<StaleState>({
    building: false,
    staleCount: 0,
    staleChapters: new Set(),
    staleBooks: new Set(),
    error: null,
  });

  $effect(() => staleState.subscribe((v) => { s = v; }));

  // Berhenti otomatis saat project berganti, exists false, atau unmount.
  $effect(() => {
    if (!exists || !projectId) return;
    const stop = startStaleStore(projectId);
    return stop;
  });
</script>

{#if exists && projectId}
  <div class="stale-slot">
    <BuildIndicator building={s.building} />
    <UpdateButton {projectId} building={s.building} staleCount={s.staleCount} />
    {#if s.error}<span class="err" title={s.error}>{s.error}</span>{/if}
  </div>
{/if}

<style>
  .stale-slot {
    display: flex;
    gap: 8px;
    align-items: center;
    justify-content: space-between;
    padding: 4px 12px;
  }
  .err {
    color: var(--danger, inherit);
    font-size: 13px;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
