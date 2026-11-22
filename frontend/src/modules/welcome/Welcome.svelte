<script lang="ts">
  import { workspaces, openWorkspaceIds, openWorkspace, renameWorkspace } from '../../lib/stores';
  import { recentWorkspaces, openFromDisk, createNew, openRecent } from './logic';
  import { showToast } from '../openfolder/toast-store';
  import { errMessage } from '../openfolder/logic';

  let pendingId = $state<string | null>(null);
  let draftName = $state('');
  let initialName = '';
  let committing = $state(false);
  let cancelled = false;
  let creating = $state(false);

  const recent = $derived(recentWorkspaces($workspaces, $openWorkspaceIds));
  const inputActive = $derived(pendingId !== null);

  function focusSelect(node: HTMLInputElement) {
    node.focus();
    node.select();
  }

  async function onCreate() {
    if (creating || inputActive) return;
    creating = true;
    try {
      const ws = await createNew();
      if (ws) {
        pendingId = ws.id;
        initialName = ws.name;
        draftName = ws.name;
        cancelled = false;
        committing = false;
      }
    } finally {
      creating = false;
    }
  }

  async function commit() {
    if (committing || pendingId === null) return;
    committing = true;
    const id = pendingId;
    const name = draftName.trim();
    if (!cancelled && name !== '' && name !== initialName) {
      try {
        await renameWorkspace(id, name);
      } catch (e) {
        showToast(errMessage(e), 'error');
      }
    }
    try {
      await openWorkspace(id);
    } catch (e) {
      showToast(errMessage(e), 'error');
    } finally {
      pendingId = null;
      committing = false;
    }
  }

  function onEditKey(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      commit();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      cancelled = true;
      commit();
    }
  }
</script>

<section class="welcome">
  <h1>Selamat datang</h1>
  <div class="actions">
    <button type="button" class="action" disabled={inputActive || creating} onclick={() => openFromDisk()}>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" /></svg>
      Open Workspace
    </button>
    {#if inputActive}
      <input
        class="name-input"
        type="text"
        aria-label="Nama workspace"
        bind:value={draftName}
        use:focusSelect
        onkeydown={onEditKey}
        onblur={commit}
      />
    {:else}
      <button type="button" class="action" disabled={creating} onclick={onCreate}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        Create New Workspace
      </button>
    {/if}
  </div>
  <div class="recent">
    <h2>Terakhir dibuka</h2>
    {#if recent.length === 0}
      <p class="empty">Belum ada workspace terakhir</p>
    {:else}
      <ul>
        {#each recent as ws (ws.id)}
          <li>
            <button type="button" class="row" disabled={inputActive} onclick={() => openRecent(ws.id)}>
              <span class="name">{ws.name}</span>
              <span class="path">{ws.projects[0]?.path ?? ''}</span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</section>

<style>
  .welcome { height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 24px; padding: 24px; box-sizing: border-box; background: var(--bg); color: var(--fg); overflow-x: hidden; min-width: 0; }
  h1 { margin: 0; font-size: 20px; font-weight: 600; }
  .actions { display: flex; gap: 12px; flex-wrap: wrap; justify-content: center; }
  .action { display: inline-flex; align-items: center; gap: 8px; padding: 10px 16px; background: var(--bg-elev); color: var(--fg); border: 1px solid var(--border); border-radius: 8px; cursor: pointer; font: inherit; }
  .action:hover:not(:disabled) { background: var(--bg-hover); }
  .action:disabled { opacity: 0.5; cursor: default; }
  .name-input { padding: 10px 12px; min-width: 240px; background: var(--bg); color: var(--fg); border: 1px solid var(--accent); border-radius: 8px; font: inherit; outline: none; }
  .recent { width: min(480px, 100%); }
  .recent h2 { font-size: 12px; text-transform: uppercase; color: var(--fg-muted); margin: 0 0 8px; }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
  .row { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; width: 100%; padding: 8px 12px; background: transparent; color: var(--fg); border: 1px solid transparent; border-radius: 6px; cursor: pointer; text-align: left; font: inherit; }
  .row:hover:not(:disabled) { background: var(--bg-hover); }
  .row:disabled { opacity: 0.5; cursor: default; }
  .name { font-weight: 600; }
  .path { color: var(--fg-muted); font-size: 12px; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .empty { color: var(--fg-muted); margin: 0; }
  .action:focus-visible, .row:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
</style>
