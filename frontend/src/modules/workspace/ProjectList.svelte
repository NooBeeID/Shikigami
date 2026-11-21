<script lang="ts">
  import { activeWorkspace, activeProjectId, selectProject, addProject, removeProject } from '../../lib/stores';
  import { errMessage, hasNativeFolderDialog, pickFolder } from './errors';

  let { onerror }: { onerror: (msg: string) => void } = $props();

  let adding = $state(false);
  let path = $state('');
  let busy = $state(false);
  let confirmId = $state<string | null>(null);

  const projects = $derived($activeWorkspace?.projects ?? []);

  async function submit() {
    const p = path.trim();
    if (!p) { onerror('Path tidak boleh kosong'); return; }
    busy = true;
    try {
      await addProject(p);
      path = ''; adding = false;
    } catch (e) { onerror(errMessage(e)); } finally { busy = false; }
  }

  async function remove(id: string) {
    busy = true;
    try { await removeProject(id); } catch (e) { onerror(errMessage(e)); } finally { busy = false; confirmId = null; }
  }

  async function browse() {
    const p = await pickFolder();
    if (p) path = p;
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Enter') submit();
    else if (e.key === 'Escape') { adding = false; path = ''; }
  }
</script>

<div class="projects">
  {#if projects.length === 0}
    <div class="muted">Belum ada project</div>
  {:else}
    <ul>
      {#each projects as p (p.id)}
        <li class:active={p.id === $activeProjectId}>
          <button type="button" class="main" title={p.path} onclick={() => selectProject(p.id)}>
            <span class="dot"></span>
            <span class="text">
              <span class="name">{p.name}</span>
              <span class="path">{p.path}</span>
            </span>
          </button>
          {#if confirmId === p.id}
            <button type="button" class="danger" onclick={() => remove(p.id)} disabled={busy}>Yakin?</button>
            <button type="button" onclick={() => (confirmId = null)} disabled={busy}>Batal</button>
          {:else}
            <button type="button" aria-label="Hapus project {p.name}" title="Hapus project" onclick={() => (confirmId = p.id)} disabled={busy}>×</button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}

  {#if adding}
    <div class="form">
      <input type="text" aria-label="Path project" placeholder="/path/ke/folder" bind:value={path} onkeydown={onKey} disabled={busy} />
      <div class="row">
        <button type="button" onclick={submit} disabled={busy}>Tambah</button>
        {#if hasNativeFolderDialog()}
          <button type="button" onclick={browse} disabled={busy}>Pilih folder...</button>
        {/if}
        <button type="button" onclick={() => { adding = false; path = ''; }} disabled={busy}>Batal</button>
      </div>
    </div>
  {:else}
    <button type="button" class="add" onclick={() => (adding = true)} disabled={!$activeWorkspace}>Tambah project</button>
  {/if}
</div>

<style>
  .projects { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  li { display: flex; align-items: center; gap: 4px; min-width: 0; border-radius: 4px; }
  li.active { background: var(--bg-hover, #0001); }
  li.active .dot { background: var(--accent, #3b82f6); }
  .main { flex: 1; min-width: 0; display: flex; align-items: center; gap: 6px; text-align: left; border: none; padding: 4px 6px; }
  .dot { width: 7px; height: 7px; border-radius: 50%; flex: none; background: transparent; border: 1px solid var(--border, #8884); }
  .text { display: flex; flex-direction: column; min-width: 0; }
  .name, .path { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .path { font-size: 11px; color: var(--fg-muted, #888); }
  .form { display: flex; flex-direction: column; gap: 4px; }
  .row { display: flex; gap: 4px; flex-wrap: wrap; }
  input { width: 100%; box-sizing: border-box; font: inherit; color: var(--fg, #222); background: var(--bg-elev, transparent); border: 1px solid var(--border, #8884); border-radius: 4px; padding: 3px 6px; }
  button { font: inherit; font-size: 12px; color: var(--fg, #222); background: transparent; border: 1px solid var(--border, #8884); border-radius: 4px; padding: 2px 8px; cursor: pointer; }
  button.main { border: none; font-size: 13px; }
  button:hover:not(:disabled) { background: var(--bg-hover, #0001); }
  button:disabled { opacity: 0.5; cursor: default; }
  button.danger { color: var(--danger, #d33); border-color: var(--danger, #d33); }
  .add { align-self: flex-start; }
  .muted { color: var(--fg-muted, #888); }
</style>
