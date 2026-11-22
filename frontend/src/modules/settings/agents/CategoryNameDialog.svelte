<script lang="ts">
  import Modal from '../Modal.svelte';
  import { upsertCategory, type Category } from './client';
  import { loadAgents, selectCategory } from './store';

  let {
    open,
    onclose,
    category,
  }: { open: boolean; onclose: () => void; category: Category | null } = $props();

  let name = $state('');
  let saving = $state(false);
  let error = $state<string | null>(null);
  let submitted = $state(false);

  $effect(() => {
    if (!open) return;
    name = category?.name ?? '';
    error = null;
    submitted = false;
    saving = false;
  });

  const nameInvalid = $derived(submitted && name.trim() === '');

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (saving) return;
    submitted = true;
    if (name.trim() === '') return;
    saving = true;
    error = null;
    try {
      if (category) {
        await upsertCategory({ id: category.id, name: name.trim(), role: category.role });
        await loadAgents();
      } else {
        const res = await upsertCategory({ name: name.trim(), role: '' });
        await loadAgents();
        selectCategory(res.id);
      }
      onclose();
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      saving = false;
    }
  }
</script>

<Modal {open} {onclose} title={category ? 'Ubah nama kategori' : 'Kategori baru'}>
  <form class="wrap" onsubmit={submit}>
    <label for="category-name">Nama</label>
    <input
      id="category-name"
      type="text"
      autocomplete="off"
      bind:value={name}
      aria-invalid={nameInvalid}
      aria-describedby={nameInvalid ? 'category-name-err' : undefined}
    />
    {#if nameInvalid}<div class="error" id="category-name-err" role="alert">Nama wajib diisi.</div>{/if}
    {#if error}<div class="error" role="alert">{error}</div>{/if}
    <div class="actions">
      <button type="button" onclick={onclose}>Batal</button>
      <button type="submit" disabled={saving}>{saving ? 'Menyimpan...' : 'Simpan'}</button>
    </div>
  </form>
</Modal>

<style>
  .wrap { display: flex; flex-direction: column; gap: 8px; padding: 16px; overflow: auto; max-height: 100%; box-sizing: border-box; }
  label { font-size: 14px; color: var(--fg-muted, #888); }
  input { background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 6px 8px; font-size: 15px; }
  .error { color: #e49a9a; font-size: 15px; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px; }
  button { background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 4px 12px; font-size: 15px; cursor: pointer; }
  button:hover:not(:disabled) { background: var(--bg-hover, #2c2c2c); }
  button:disabled { opacity: 0.6; cursor: default; }
  button:focus-visible, input:focus-visible { outline: 2px solid var(--fg-muted, #888); outline-offset: 2px; }
</style>
