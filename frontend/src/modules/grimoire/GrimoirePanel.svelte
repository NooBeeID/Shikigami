<script lang="ts">
  import { grimoire, startGrimoireStore, stopGrimoireStore, generate, type GrimoireState } from './store';
  import { activeProject } from '../../lib/stores';
  import { openGrimoirePage } from './openPage';
  import type { GrimoireChapter } from './client';
  import LawsSlot from './laws/LawsSlot.svelte';
  import { lawsViewOpen } from './laws/lawsStore';
  import StaleSlot from './stale/StaleSlot.svelte';
  import StaleBadge from './stale/StaleBadge.svelte';
  import LlmSettings from './llm/LlmSettings.svelte';

  let s = $state<GrimoireState>({ projectId: null, exists: false, building: false, books: [], error: null });
  let expanded = $state<Record<string, boolean>>({});
  let openError = $state<string | null>(null);
  let lastProject: string | null = null;

  $effect(() => grimoire.subscribe((v) => { s = v; }));
  $effect(() => {
    startGrimoireStore();
    return () => stopGrimoireStore();
  });

  // Reset status lipat dan galat lokal saat project berganti
  $effect(() => {
    const id = s.projectId;
    if (id !== lastProject) {
      lastProject = id;
      expanded = {};
      openError = null;
    }
  });

  function toggleBook(name: string) {
    expanded = { ...expanded, [name]: !expanded[name] };
  }

  function openChapter(c: GrimoireChapter) {
    if (!s.projectId) return;
    openError = null;
    void openGrimoirePage(s.projectId, c.path).catch((e) => {
      openError = e instanceof Error ? e.message : String(e);
    });
  }

  function rebuild() {
    void generate();
  }

  let errText = $derived(s.error ?? openError);
</script>

<section class="grimoire" aria-label="Grimoire">
  <div class="header">
    <span class="title">GRIMOIRE</span>
    {#if s.exists && !s.building && s.projectId}
      <button class="icon" type="button" title="Bangun ulang grimoire" aria-label="Bangun ulang grimoire" onclick={rebuild}>&#8635;</button>
    {/if}
  </div>
  <LawsSlot projectId={s.projectId} exists={s.exists} />
  <StaleSlot projectId={s.projectId} exists={s.exists} />
  {#if $lawsViewOpen && s.projectId && s.exists}
    <!-- editor laws menggantikan daftar buku -->
  {:else if !$activeProject || !s.projectId}
    <div class="msg">Pilih project</div>
  {:else}
    {#if s.building}
      <div class="msg" role="status">Membangun...</div>
    {/if}
    {#if !s.exists}
      {#if s.building}
        <button class="generate" type="button" disabled>Generate Grimoire</button>
      {:else}
        <button class="generate" type="button" onclick={generate}>Generate Grimoire</button>
      {/if}
    {:else if s.books.length === 0}
      <div class="msg">Grimoire kosong</div>
    {:else}
      <div class="list">
        {#each s.books as b (b.name)}
          <button class="book" type="button" aria-expanded={!!expanded[b.name]} onclick={() => toggleBook(b.name)}>
            <span class="arrow">{expanded[b.name] ? '▾' : '▸'}</span>
            <span class="name">{b.name}</span>
            {#if b.chapters.some((c) => c.stale)}<StaleBadge />{/if}
            <span class="count">{b.chapters.length}</span>
          </button>
          {#if expanded[b.name]}
            <ul class="chapters">
              {#each b.chapters as c (c.path)}
                <li>
                  <button class="chapter" type="button" onclick={() => openChapter(c)} title={c.path}>
                    <span class="name">{c.name}</span>
                    {#if c.stale}<StaleBadge />{/if}
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        {/each}
      </div>
    {/if}
  {/if}
  {#if errText}
    <div class="msg err" role="alert" title={errText}>{errText}</div>
  {/if}
  <LlmSettings projectId={s.projectId} exists={s.exists} />
</section>

<style>
  .grimoire {
    height: 100%;
    display: flex;
    flex-direction: column;
    font-size: 14px;
    color: var(--fg, inherit);
    background: color-mix(in srgb, var(--bg, #1b1b1f) 88%, black);
    border-top: 1px solid var(--border, transparent);
    min-height: 0;
  }
  .header { display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; flex: none; }
  .title { font-weight: 600; text-transform: uppercase; font-size: 13px; color: var(--fg-muted); }
  .icon { background: none; border: none; color: inherit; font: inherit; cursor: pointer; padding: 0 4px; }
  .icon:hover, .icon:focus-visible { background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
  .book, .chapter, .generate {
    background: none; border: none; color: inherit; font: inherit; text-align: left; cursor: pointer; width: 100%;
    height: 28px; display: flex; align-items: center; gap: 4px; overflow: hidden;
  }
  .book:hover, .chapter:hover, .generate:hover:not(:disabled),
  .book:focus-visible, .chapter:focus-visible, .generate:focus-visible {
    background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent));
    outline: 1px solid var(--accent, currentColor);
    outline-offset: -1px;
  }
  .book { padding-left: 12px; padding-right: 12px; }
  .chapter { padding-left: 26px; padding-right: 12px; }
  .count { color: var(--fg-muted); flex: none; }
  .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .generate {
    margin: 8px 12px; width: auto; justify-content: center; padding: 0 12px;
    border: 1px solid var(--border, currentColor); border-radius: 4px; background: var(--bg-elev, transparent);
  }
  .generate:disabled { opacity: 0.6; cursor: default; }
  .err { color: var(--danger, inherit); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .msg { padding: 12px; color: var(--fg-muted, inherit); }
  .chapters { list-style: none; margin: 0; padding: 0; }
  .list { flex: 1; overflow: auto; min-height: 0; }
</style>
