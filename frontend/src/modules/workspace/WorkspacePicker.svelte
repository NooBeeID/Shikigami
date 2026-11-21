<script lang="ts">
  import {
    workspaces, activeWorkspaceId, selectWorkspace, createWorkspace, renameWorkspace, deleteWorkspace,
  } from '../../lib/stores';
  import { errMessage } from './errors';

  let { onerror }: { onerror: (msg: string) => void } = $props();

  let mode = $state<'idle' | 'create' | 'rename'>('idle');
  let draft = $state('');
  let busy = $state(false);
  let confirmDelete = $state(false);

  async function run(fn: () => Promise<void>) {
    busy = true;
    try { await fn(); } catch (e) { onerror(errMessage(e)); } finally { busy = false; }
  }

  function onSelect(e: Event) {
    const id = (e.currentTarget as HTMLSelectElement).value;
    run(() => selectWorkspace(id));
  }

  function startCreate() { draft = ''; mode = 'create'; confirmDelete = false; }
  function startRename() {
    draft = $workspaces.find((w) => w.id === $activeWorkspaceId)?.name ?? '';
    mode = 'rename'; confirmDelete = false;
  }
  function cancel() { mode = 'idle'; draft = ''; }

  async function submit() {
    const name = draft.trim();
    if (!name) { onerror('Nama tidak boleh kosong'); return; }
    const id = $activeWorkspaceId;
    const m = mode;
    await run(async () => {
      if (m === 'create') {
        const ws = await createWorkspace(name);
        await selectWorkspace(ws.id);
      } else if (id) {
        await renameWorkspace(id, name);
      }
      mode = 'idle'; draft = '';
    });
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Enter') submit();
    else if (e.key === 'Escape') cancel();
  }

  async function doDelete() {
    const id = $activeWorkspaceId;
    if (!id) return;
    await run(async () => { await deleteWorkspace(id); });
    confirmDelete = false;
  }
</script>

<div class="picker">
  {#if $workspaces.length === 0}
    <div class="muted">Belum ada workspace</div>
  {:else}
    <select aria-label="Workspace" value={$activeWorkspaceId ?? ''} onchange={onSelect} disabled={busy}>
      {#each $workspaces as w (w.id)}
        <option value={w.id}>{w.name}</option>
      {/each}
    </select>
  {/if}

  {#if mode === 'idle'}
    <div class="row">
      <button type="button" onclick={startCreate} disabled={busy}>Baru</button>
      <button type="button" onclick={startRename} disabled={busy || !$activeWorkspaceId}>Ganti nama</button>
      {#if confirmDelete}
        <button type="button" class="danger" onclick={doDelete} disabled={busy}>Yakin?</button>
        <button type="button" onclick={() => (confirmDelete = false)} disabled={busy}>Batal</button>
      {:else}
        <button type="button" onclick={() => (confirmDelete = true)} disabled={busy || !$activeWorkspaceId}>Hapus</button>
      {/if}
    </div>
  {:else}
    <div class="row">
      <input type="text" aria-label="Nama workspace" placeholder="Nama workspace" bind:value={draft} onkeydown={onKey} disabled={busy} />
      <button type="button" onclick={submit} disabled={busy}>OK</button>
      <button type="button" onclick={cancel} disabled={busy}>Batal</button>
    </div>
  {/if}
</div>

<style>
  .picker { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
  .row { display: flex; gap: 4px; flex-wrap: wrap; min-width: 0; }
  select, input { width: 100%; min-width: 0; box-sizing: border-box; font: inherit; color: var(--fg, #222); background: var(--bg-elev, transparent); border: 1px solid var(--border, #8884); border-radius: 4px; padding: 3px 6px; }
  .row input { flex: 1; width: auto; }
  button { font: inherit; font-size: 12px; color: var(--fg, #222); background: transparent; border: 1px solid var(--border, #8884); border-radius: 4px; padding: 2px 8px; cursor: pointer; }
  button:hover:not(:disabled) { background: var(--bg-hover, #0001); }
  button:disabled { opacity: 0.5; cursor: default; }
  button.danger { color: var(--danger, #d33); border-color: var(--danger, #d33); }
  .muted { color: var(--fg-muted, #888); }
</style>
