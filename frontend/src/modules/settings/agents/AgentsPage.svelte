<script lang="ts">
  import { onMount } from 'svelte';
  import { providers, loadProviders } from '../../../lib/stores/providers';
  import { deleteAgent, setDefaultAgent, upsertCategory, type Agent, type Category } from './client';
  import {
    agents,
    agentsError,
    agentsLoading,
    categories,
    loadAgents,
    selectCategory,
    selectedCategoryId,
    startAgentsSync,
  } from './store';
  import AgentFormDialog from './AgentFormDialog.svelte';
  import CategoryDeleteDialog from './CategoryDeleteDialog.svelte';
  import CategoryNameDialog from './CategoryNameDialog.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';

  onMount(() => {
    void loadAgents();
    void loadProviders().catch(() => {});
    const stop = startAgentsSync();
    return stop;
  });

  const selected = $derived($categories.find((c) => c.id === $selectedCategoryId) ?? null);
  const categoryAgents = $derived(
    $agents.filter((a) => a.categoryId === $selectedCategoryId).sort((a, b) => a.name.localeCompare(b.name)),
  );
  const otherCategories = $derived($categories.filter((c) => c.id !== $selectedCategoryId));

  let roleText = $state('');
  let roleDirty = $state(false);
  let roleSaving = $state(false);
  let roleError = $state<string | null>(null);
  let roleSaved = $state(false);
  let rowError = $state<string | null>(null);
  let lastRoleCat: string | null = null;

  let agentDialog = $state<{ open: boolean; agent: Agent | null }>({ open: false, agent: null });
  let categoryNameDialog = $state<{ open: boolean; category: Category | null }>({
    open: false,
    category: null,
  });
  let deleteCategoryOpen = $state(false);
  let deleteAgentTarget = $state<Agent | null>(null);

  // Sinkronkan teks peran: timpa bila kategori berganti, atau bila tidak ada edit tertunda.
  $effect(() => {
    const id = selected?.id ?? null;
    const role = selected?.role ?? '';
    if (id !== lastRoleCat) {
      lastRoleCat = id;
      roleText = role;
      roleDirty = false;
      roleSaved = false;
      roleError = null;
    } else if (!roleDirty) {
      roleText = role;
    }
  });

  async function saveRole() {
    if (!selected || !roleDirty || roleSaving) return;
    roleSaving = true;
    roleError = null;
    roleSaved = false;
    try {
      await upsertCategory({ id: selected.id, name: selected.name, role: roleText });
      roleDirty = false;
      roleSaved = true;
      await loadAgents();
    } catch (e) {
      roleError = e instanceof Error ? e.message : String(e);
    } finally {
      roleSaving = false;
    }
  }

  async function makeDefault(a: Agent) {
    rowError = null;
    try {
      await setDefaultAgent(a.id);
      await loadAgents();
    } catch (e) {
      rowError = e instanceof Error ? e.message : String(e);
    }
  }

  function providerName(id: string): string {
    return $providers.find((p) => p.id === id)?.name || id;
  }

  function onTabKey(e: KeyboardEvent, idx: number) {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    e.preventDefault();
    const list = $categories;
    if (list.length === 0) return;
    const next = (idx + (e.key === 'ArrowDown' ? 1 : -1) + list.length) % list.length;
    selectCategory(list[next].id);
    document.getElementById(`agents-tab-${list[next].id}`)?.focus();
  }
</script>

<div class="layout">
  <nav class="left" aria-label="Kategori agent">
    <div role="tablist" aria-orientation="vertical" class="tabs">
      {#each $categories as c, i (c.id)}
        <button
          type="button"
          role="tab"
          class="tab"
          id={`agents-tab-${c.id}`}
          aria-selected={c.id === $selectedCategoryId}
          aria-controls="agents-panel"
          tabindex={c.id === $selectedCategoryId ? 0 : -1}
          onclick={() => selectCategory(c.id)}
          onkeydown={(e) => onTabKey(e, i)}>{c.name}</button
        >
      {/each}
    </div>
    <button type="button" class="add" onclick={() => (categoryNameDialog = { open: true, category: null })}
      >Kategori baru</button
    >
  </nav>

  <div
    class="right"
    role="tabpanel"
    id="agents-panel"
    aria-labelledby={selected ? `agents-tab-${selected.id}` : undefined}
  >
    {#if $agentsLoading && $categories.length === 0}
      <p class="muted">Memuat...</p>
    {:else}
      {#if $agentsError}
        <div class="error" role="alert">
          <span>{$agentsError}</span>
          <button type="button" onclick={() => loadAgents()}>Coba lagi</button>
        </div>
      {/if}
      {#if selected}
        <header class="head">
          <h2>{selected.name}</h2>
          <div class="head-actions">
            <button
              type="button"
              onclick={() => (categoryNameDialog = { open: true, category: selected })}>Ubah nama</button
            >
            {#if selected.builtin}
              <span class="muted small">Kategori bawaan tidak dapat dihapus</span>
            {:else}
              <button type="button" onclick={() => (deleteCategoryOpen = true)}>Hapus kategori</button>
            {/if}
          </div>
        </header>

        <section class="role">
          <label for="agents-role">Peran</label>
          <textarea
            id="agents-role"
            rows="4"
            bind:value={roleText}
            oninput={() => {
              roleDirty = true;
              roleSaved = false;
            }}
            onblur={() => void saveRole()}
          ></textarea>
          <div class="role-actions">
            <button type="button" disabled={!roleDirty || roleSaving} onclick={() => void saveRole()}
              >Simpan peran</button
            >
            <span role="status" class="muted small">
              {#if roleSaving}Menyimpan...{:else if roleSaved}Tersimpan{/if}
            </span>
          </div>
          {#if roleError}<div class="error" role="alert">{roleError}</div>{/if}
        </section>

        <section>
          <div class="sub">
            <h3>Agent</h3>
            <button type="button" onclick={() => (agentDialog = { open: true, agent: null })}
              >Agent baru</button
            >
          </div>
          {#if rowError}<div class="error" role="alert">{rowError}</div>{/if}
          {#if categoryAgents.length === 0}
            <p class="muted">Belum ada agent di kategori ini.</p>
          {:else}
            <ul>
              {#each categoryAgents as a (a.id)}
                <li>
                  <div class="info">
                    <div class="line">
                      <span class="name">{a.name}</span>
                      {#if a.isDefault}
                        <span class="badge is-default"><span aria-hidden="true">★ </span>Default</span>
                      {/if}
                    </div>
                    <div class="muted small">
                      {providerName(a.providerId)} · {a.model || 'default provider'}
                    </div>
                  </div>
                  <div class="row-actions">
                    {#if !a.isDefault}
                      <button
                        type="button"
                        aria-label={`Jadikan default ${a.name}`}
                        onclick={() => void makeDefault(a)}>Jadikan default</button
                      >
                    {/if}
                    <button
                      type="button"
                      aria-label={`Edit ${a.name}`}
                      onclick={() => (agentDialog = { open: true, agent: a })}>Edit</button
                    >
                    <button
                      type="button"
                      aria-label={`Hapus ${a.name}`}
                      onclick={() => (deleteAgentTarget = a)}>Hapus</button
                    >
                  </div>
                </li>
              {/each}
            </ul>
          {/if}
        </section>
      {/if}
    {/if}
  </div>
</div>

{#if agentDialog.open && selected}
  <AgentFormDialog
    open={true}
    onclose={() => (agentDialog = { open: false, agent: null })}
    categoryId={selected.id}
    agent={agentDialog.agent}
  />
{/if}
{#if categoryNameDialog.open}
  <CategoryNameDialog
    open={true}
    onclose={() => (categoryNameDialog = { open: false, category: null })}
    category={categoryNameDialog.category}
  />
{/if}
{#if deleteCategoryOpen && selected}
  <CategoryDeleteDialog
    open={true}
    onclose={() => (deleteCategoryOpen = false)}
    category={selected}
    agentCount={categoryAgents.length}
    {otherCategories}
  />
{/if}
{#if deleteAgentTarget}
  {@const target = deleteAgentTarget}
  <ConfirmDialog
    open={true}
    onclose={() => (deleteAgentTarget = null)}
    title="Hapus agent"
    message={`Hapus agent ${target.name}?`}
    confirmLabel="Hapus"
    onconfirm={async () => {
      await deleteAgent(target.id);
      await loadAgents();
    }}
  />
{/if}

<style>
  .layout { display: flex; height: 100%; min-height: 0; }
  .left { width: 200px; flex: none; border-right: 1px solid var(--border, #333); padding: 8px; display: flex; flex-direction: column; gap: 8px; box-sizing: border-box; overflow: auto; }
  .tabs { display: flex; flex-direction: column; gap: 2px; }
  .tab { text-align: left; background: transparent; border: none; border-left: 3px solid transparent; border-radius: 4px; color: var(--fg, #ddd); padding: 6px 10px; font-size: 15px; cursor: pointer; }
  .tab:hover { background: var(--bg-hover, #2c2c2c); }
  .tab[aria-selected='true'] { background: var(--bg-hover, #2c2c2c); border-left-color: var(--fg, #ddd); font-weight: 600; }
  .right { flex: 1; min-width: 0; padding: 12px 16px; overflow: auto; display: flex; flex-direction: column; gap: 16px; box-sizing: border-box; }
  h2 { margin: 0; font-size: 18px; color: var(--fg, #ddd); }
  h3 { margin: 0; font-size: 16px; color: var(--fg, #ddd); }
  .head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
  .head-actions, .row-actions, .role-actions { display: flex; align-items: center; gap: 8px; }
  .role { display: flex; flex-direction: column; gap: 6px; }
  .role label { font-size: 14px; color: var(--fg-muted, #888); }
  textarea { background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 6px 8px; font-size: 15px; font-family: inherit; resize: vertical; }
  .sub { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
  .muted { margin: 0; color: var(--fg-muted, #888); font-size: 15px; }
  .small { font-size: 13px; }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
  li { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 12px; border: 1px solid var(--border, #333); background: var(--bg-elev, #222); border-radius: 6px; }
  .line { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .name { color: var(--fg, #ddd); font-size: 16px; font-weight: 600; }
  .badge { font-size: 13px; padding: 2px 8px; border-radius: 10px; border: 1px solid transparent; }
  .is-default { background: rgba(60, 160, 90, 0.2); border-color: rgba(60, 160, 90, 0.5); color: #8fd6a3; }
  .error { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; color: #e49a9a; font-size: 15px; }
  button:not(.tab) { background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 4px 12px; font-size: 15px; cursor: pointer; }
  button:hover:not(:disabled):not(.tab) { background: var(--bg-hover, #2c2c2c); }
  button:disabled { opacity: 0.6; cursor: default; }
  button:focus-visible, textarea:focus-visible { outline: 2px solid var(--fg-muted, #888); outline-offset: 2px; }
</style>
