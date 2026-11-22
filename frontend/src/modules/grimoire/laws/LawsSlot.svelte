<script lang="ts">
  import { lawsViewOpen, startLawsStore } from './lawsStore';
  import LawsEditor from './LawsEditor.svelte';
  import InjectionInfo from './InjectionInfo.svelte';

  let { projectId, exists }: { projectId: string | null; exists: boolean } = $props();

  // Teardown menutup editor saat project/exists berubah dan saat unmount.
  $effect(() => {
    const id = projectId;
    if (!id || !exists) return;
    const stop = startLawsStore(id);
    return () => {
      stop();
      lawsViewOpen.set(false);
    };
  });
</script>

{#if projectId && exists}
  {#if $lawsViewOpen}
    {#key projectId}
      <LawsEditor {projectId} onback={() => lawsViewOpen.set(false)} />
    {/key}
  {:else}
    <button class="laws-entry" onclick={() => lawsViewOpen.set(true)}>Laws</button>
    <InjectionInfo {projectId} />
  {/if}
{/if}

<style>
  .laws-entry {
    display: block; width: 100%; text-align: left; background: transparent;
    color: var(--fg); border: 0; padding: 6px 12px; cursor: pointer;
  }
  .laws-entry:hover { background: var(--bg-hover); }
</style>
