<script lang="ts">
  import Modal from '../Modal.svelte';
  import { deleteCategory, type Category } from './client';
  import { loadAgents } from './store';

  let {
    open,
    onclose,
    category,
    agentCount,
    otherCategories,
  }: {
    open: boolean;
    onclose: () => void;
    category: Category;
    agentCount: number;
    otherCategories: Category[];
  } = $props();

  // svelte-ignore state_referenced_locally
  let mode = $state<'move' | 'delete'>(otherCategories.length > 0 ? 'move' : 'delete');
  // svelte-ignore state_referenced_locally
  let moveTo = $state<string>(otherCategories[0]?.id ?? '');
  let busy = $state(false);
  let error = $state<string | null>(null);

  const canSubmit = $derived(!busy && (agentCount === 0 || mode === 'delete' || moveTo !== ''));

  async function submit() {
    if (!canSubmit) return;
    busy = true;
    error = null;
    try {
      if (agentCount === 0) await deleteCategory({ id: category.id });
      else if (mode === 'move') await deleteCategory({ id: category.id, agents: 'move', moveTo });
      else await deleteCategory({ id: category.id, agents: 'delete' });
      await loadAgents();
      onclose();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }
</script>

<Modal {open} {onclose} title="Hapus kategori">
  <div class="wrap">
    {#if agentCount === 0}
      <p>Hapus kategori <strong>{category.name}</strong>?</p>
    {:else}
      <p>Kategori <strong>{category.name}</strong> masih punya {agentCount} agent.</p>
      <div role="radiogroup" aria-label="Tindakan untuk agent di kategori ini" class="group">
        <label class="radio">
          <input
            type="radio"
            name="delete-mode"
            value="move"
            bind:group={mode}
            disabled={otherCategories.length === 0}
          />
          Pindahkan agent ke kategori lain
        </label>
        <select
          aria-label="Kategori tujuan"
          bind:value={moveTo}
          disabled={mode !== 'move' || otherCategories.length === 0}
        >
          {#each otherCategories as c (c.id)}
            <option value={c.id}>{c.name}</option>
          {/each}
        </select>
        <label class="radio">
          <input type="radio" name="delete-mode" value="delete" bind:group={mode} />
          Hapus agent juga
        </label>
      </div>
    {/if}
    {#if error}<div class="error" role="alert">{error}</div>{/if}
    <div class="actions">
      <button type="button" onclick={onclose}>Batal</button>
      <button type="button" class="danger" disabled={!canSubmit} onclick={submit}>
        {busy ? 'Menghapus...' : 'Hapus'}
      </button>
    </div>
  </div>
</Modal>

<style>
  .wrap { display: flex; flex-direction: column; gap: 12px; padding: 16px; overflow: auto; max-height: 100%; box-sizing: border-box; }
  p { margin: 0; font-size: 15px; }
  .group { display: flex; flex-direction: column; gap: 8px; }
  .radio { display: flex; align-items: center; gap: 8px; font-size: 15px; }
  select { background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 6px 8px; font-size: 15px; margin-left: 24px; }
  .error { color: #e49a9a; font-size: 15px; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  button { background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 4px 12px; font-size: 15px; cursor: pointer; }
  button:hover:not(:disabled) { background: var(--bg-hover, #2c2c2c); }
  button:disabled { opacity: 0.6; cursor: default; }
  button:focus-visible, select:focus-visible, input:focus-visible { outline: 2px solid var(--fg-muted, #888); outline-offset: 2px; }
  .danger { border-color: rgba(200, 70, 70, 0.6); color: #e49a9a; }
</style>
