<script lang="ts">
  import { onDestroy } from 'svelte';
  import {
    workspaces, openWorkspaceIds, activeWorkspaceId, selectWorkspace, openWorkspace, closeWorkspaceTab,
    createWorkspace, renameWorkspace, deleteWorkspace,
  } from '../../lib/stores';
  import { defaultWorkspaceName, errText } from './util';

  let editingId = $state<string | null>(null);
  let draft = $state('');
  let menu = $state<{ id: string; x: number; y: number } | null>(null);
  let confirmId = $state<string | null>(null);
  let notice = $state('');
  let busy = $state(false);
  let addMenu = $state(false);
  let showOpenList = $state(false);

  const openTabs = $derived(
    $openWorkspaceIds
      .map((id) => $workspaces.find((w) => w.id === id))
      .filter((w): w is NonNullable<typeof w> => !!w),
  );

  const closedList = $derived($workspaces.filter((w) => !$openWorkspaceIds.includes(w.id)));

  let noticeTimer: ReturnType<typeof setTimeout> | undefined;
  let cancelled = false;
  let committing = false;
  let tabEls: Record<string, HTMLElement | undefined> = {};
  let rootEl: HTMLDivElement | undefined = $state();

  function say(msg: string) {
    notice = msg;
    if (noticeTimer) clearTimeout(noticeTimer);
    noticeTimer = setTimeout(() => (notice = ''), 3000);
  }
  onDestroy(() => { if (noticeTimer) clearTimeout(noticeTimer); });

  async function pick(id: string) {
    if (id === $activeWorkspaceId) return;
    try { await selectWorkspace(id); } catch (e) { say(errText(e)); }
  }

  function toggleAdd() {
    addMenu = !addMenu;
    showOpenList = false;
  }

  async function openExisting(id: string) {
    addMenu = false;
    showOpenList = false;
    try { await openWorkspace(id); } catch (e) { say(errText(e)); }
  }

  function askDelete(id: string) {
    if ($workspaces.length <= 1) { say('Tidak bisa menghapus workspace terakhir'); return; }
    addMenu = false;
    showOpenList = false;
    confirmId = id;
  }

  async function add() {
    if (busy) return;
    addMenu = false;
    showOpenList = false;
    busy = true;
    try {
      const ws = await createWorkspace(defaultWorkspaceName($workspaces));
      await openWorkspace(ws.id);
      startEdit(ws.id, ws.name);
    } catch (e) {
      say(errText(e));
    } finally {
      busy = false;
    }
  }

  function startEdit(id: string, name: string) {
    cancelled = false;
    committing = false;
    draft = name;
    editingId = id;
    menu = null;
  }

  function focusSelect(node: HTMLInputElement) {
    node.focus();
    node.select();
  }

  async function commit() {
    if (cancelled || committing || editingId === null) return;
    committing = true;
    const id = editingId;
    const name = draft.trim();
    const old = $workspaces.find((w) => w.id === id)?.name;
    editingId = null;
    try {
      if (name && name !== old) await renameWorkspace(id, name);
    } catch (e) {
      say(errText(e));
    } finally {
      committing = false;
    }
  }

  function cancelEdit() {
    cancelled = true;
    editingId = null;
  }

  function onEditKey(e: KeyboardEvent) {
    if (e.key === 'Enter') { e.preventDefault(); commit(); }
    else if (e.key === 'Escape') { e.preventDefault(); cancelEdit(); }
  }

  function onContext(e: MouseEvent, id: string) {
    e.preventDefault();
    const x = Math.max(0, Math.min(e.clientX, window.innerWidth - 160));
    const y = Math.max(0, Math.min(e.clientY, window.innerHeight - 120));
    menu = { id, x, y };
  }

  function menuRename() {
    if (!menu) return;
    const ws = $workspaces.find((w) => w.id === menu!.id);
    if (ws) startEdit(ws.id, ws.name);
    menu = null;
  }

  async function menuClose() {
    if (!menu) return;
    const id = menu.id;
    menu = null;
    try { await closeWorkspaceTab(id); } catch (e) { say(errText(e)); }
  }

  function menuDelete() {
    if (!menu) return;
    if ($workspaces.length <= 1) {
      say('Tidak bisa menghapus workspace terakhir');
      menu = null;
      return;
    }
    confirmId = menu.id;
    menu = null;
  }

  async function doDelete() {
    const id = confirmId;
    confirmId = null;
    if (!id) return;
    if ($workspaces.length <= 1) { say('Tidak bisa menghapus workspace terakhir'); return; }
    try { await deleteWorkspace(id); } catch (e) { say(errText(e)); }
  }

  const confirmName = $derived($workspaces.find((w) => w.id === confirmId)?.name ?? '');

  $effect(() => {
    if (!menu) return;
    const close = () => (menu = null);
    const down = (e: PointerEvent) => {
      if (!(e.target as HTMLElement).closest?.('.ctx')) close();
    };
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') close(); };
    window.addEventListener('pointerdown', down);
    window.addEventListener('keydown', key);
    window.addEventListener('scroll', close, true);
    window.addEventListener('resize', close);
    return () => {
      window.removeEventListener('pointerdown', down);
      window.removeEventListener('keydown', key);
      window.removeEventListener('scroll', close, true);
      window.removeEventListener('resize', close);
    };
  });

  $effect(() => {
    if (!addMenu) return;
    const down = (e: PointerEvent) => {
      const t = e.target as HTMLElement;
      if (!t.closest?.('.addmenu') && !t.closest?.('.add')) { addMenu = false; showOpenList = false; }
    };
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') { addMenu = false; showOpenList = false; } };
    window.addEventListener('pointerdown', down);
    window.addEventListener('keydown', key);
    return () => {
      window.removeEventListener('pointerdown', down);
      window.removeEventListener('keydown', key);
    };
  });

  $effect(() => {
    if (!confirmId) return;
    const down = (e: PointerEvent) => {
      if (!(e.target as HTMLElement).closest?.('.confirm')) confirmId = null;
    };
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') confirmId = null; };
    window.addEventListener('pointerdown', down);
    window.addEventListener('keydown', key);
    return () => {
      window.removeEventListener('pointerdown', down);
      window.removeEventListener('keydown', key);
    };
  });

  $effect(() => {
    const id = $activeWorkspaceId;
    const el = id ? tabEls[id] : undefined;
    if (el && typeof el.scrollIntoView === 'function') el.scrollIntoView({ inline: 'nearest', block: 'nearest' });
  });
</script>

<div class="tabs-wrap" bind:this={rootEl}>
  <div class="tabs" role="tablist">
    {#each openTabs as w (w.id)}
      {#if editingId === w.id}
        <input
          class="edit"
          aria-label="Nama workspace"
          bind:value={draft}
          use:focusSelect
          onkeydown={onEditKey}
          onblur={commit}
        />
      {:else}
        <button
          type="button"
          role="tab"
          class="tab"
          class:active={w.id === $activeWorkspaceId}
          aria-selected={w.id === $activeWorkspaceId}
          title={w.name}
          bind:this={tabEls[w.id]}
          onclick={() => pick(w.id)}
          oncontextmenu={(e) => onContext(e, w.id)}
        >{w.name}</button>
      {/if}
    {/each}
  </div>
  <button type="button" class="add" aria-label="Workspace baru" title="Workspace baru" aria-haspopup="menu" aria-expanded={addMenu} disabled={busy} onclick={toggleAdd}>+</button>
  {#if addMenu}
    <div class="addmenu" role="menu">
      <button type="button" role="menuitem" onclick={add}>Create New</button>
      <button type="button" role="menuitem" aria-expanded={showOpenList} onclick={() => (showOpenList = !showOpenList)}>
        Open Workspace <span class="chev">{showOpenList ? '▾' : '›'}</span>
      </button>
      {#if showOpenList}
        <div class="openlist" role="group" aria-label="Workspace tersedia">
          {#each closedList as w (w.id)}
            <div class="orow">
              <button type="button" role="menuitem" class="oname" title={w.name} onclick={() => openExisting(w.id)}>{w.name}</button>
              <button type="button" class="odel" aria-label={`Hapus ${w.name}`} title="Hapus workspace" onclick={() => askDelete(w.id)}>🗑</button>
            </div>
          {:else}
            <div class="empty">Tidak ada workspace lain</div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
  {#if confirmId}
    <div class="confirm" role="alertdialog" aria-label="Konfirmasi hapus">
      <span>Hapus "{confirmName}"?</span>
      <button type="button" class="danger" onclick={doDelete}>Hapus</button>
      <button type="button" onclick={() => (confirmId = null)}>Batal</button>
    </div>
  {/if}
  {#if notice}<span class="notice" role="status">{notice}</span>{/if}
</div>

{#if menu}
  <div class="ctx" role="menu" style:left="{menu.x}px" style:top="{menu.y}px">
    <button type="button" role="menuitem" onclick={menuRename}>Ganti nama</button>
    <button type="button" role="menuitem" onclick={menuClose}>Tutup</button>
    <button type="button" role="menuitem" class:dim={$workspaces.length <= 1} aria-disabled={$workspaces.length <= 1} onclick={menuDelete}>Hapus</button>
  </div>
{/if}

<style>
  .tabs-wrap {
    --wails-draggable: no-drag;
    position: relative;
    display: flex;
    align-items: center;
    gap: 4px;
    flex: 1 1 auto;
    min-width: 0;
    height: 100%;
  }
  .tabs {
    display: flex;
    align-items: stretch;
    flex: 0 1 auto;
    max-width: 100%;
    height: 100%;
    min-width: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tabs::-webkit-scrollbar { display: none; }
  .tab, .add, .edit {
    --wails-draggable: no-drag;
    font-size: 14px;
  }
  .tab {
    flex: none;
    max-width: 160px;
    padding: 0 16px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    border-radius: 0;
    color: var(--fg-muted, #888);
    cursor: pointer;
  }
  .tab:hover { background: var(--bg-hover, #333); color: var(--fg, inherit); }
  .tab.active { color: var(--accent, #4c8dff); border-bottom-color: var(--accent, #4c8dff); }
  .edit {
    flex: none;
    width: 130px;
    align-self: center;
    margin: 0 4px;
    padding: 2px 6px;
  }
  .add {
    flex: none;
    width: 24px;
    height: 24px;
    padding: 0;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--fg-muted, #888);
    cursor: pointer;
    font-size: 18px;
  }
  .add:hover { background: var(--bg-hover, #333); color: var(--fg, inherit); }
  .addmenu {
    --wails-draggable: no-drag;
    position: absolute;
    top: calc(100% + 4px);
    right: 0;
    z-index: 1000;
    min-width: 200px;
    max-width: 280px;
    padding: 4px;
    background: var(--bg-elev, #222);
    border: 1px solid var(--border, #444);
    border-radius: 6px;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  }
  .addmenu > button {
    display: flex;
    justify-content: space-between;
    width: 100%;
    padding: 5px 8px;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--fg, inherit);
    font-size: 14px;
    text-align: left;
    cursor: pointer;
  }
  .addmenu > button:hover, .oname:hover, .odel:hover { background: var(--bg-hover, #333); }
  .chev { color: var(--fg-muted, #888); }
  .openlist { margin-top: 4px; padding-top: 4px; border-top: 1px solid var(--border, #444); max-height: 240px; overflow-y: auto; }
  .orow { display: flex; align-items: center; gap: 2px; }
  .oname {
    flex: 1 1 auto;
    min-width: 0;
    padding: 5px 8px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--fg, inherit);
    font-size: 14px;
    text-align: left;
    cursor: pointer;
  }
  .odel { flex: none; padding: 3px 6px; background: transparent; border: none; border-radius: 4px; font-size: 13px; cursor: pointer; }
  .empty { padding: 5px 8px; font-size: 13px; color: var(--fg-muted, #888); }
  .confirm {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    z-index: 1000;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    font-size: 14px;
    white-space: nowrap;
    background: var(--bg-elev, #222);
    border: 1px solid var(--border, #444);
    border-radius: 6px;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  }
  .danger { color: var(--danger, #f85149); }
  .notice { flex: none; font-size: 13px; color: var(--danger, #f85149); white-space: nowrap; }
  .ctx {
    --wails-draggable: no-drag;
    position: fixed;
    z-index: 1100;
    min-width: 150px;
    padding: 4px;
    background: var(--bg-elev, #222);
    border: 1px solid var(--border, #444);
    border-radius: 6px;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  }
  .ctx button {
    display: block;
    width: 100%;
    padding: 5px 8px;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--fg, inherit);
    font-size: 14px;
    text-align: left;
    cursor: pointer;
  }
  .ctx button:hover { background: var(--bg-hover, #333); }
  .ctx button.dim { color: var(--fg-muted, #888); opacity: 0.6; }
</style>
