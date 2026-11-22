<script lang="ts">
  import { get } from 'svelte/store';
  import { activeWorkspaceId } from '../../../lib/stores/workspaces';
  import { larkSourceRemove, larkSourceRename, parseLarkError } from '../client';
  import { refreshSources } from '../store';

  let {
    id,
    name,
    host,
    label,
    onopen,
  }: { id: string; name: string; host: string; label: string; onopen: () => void } = $props();

  let menuOpen = $state(false);
  let editing = $state(false);
  let confirming = $state(false);
  let draft = $state('');
  let err = $state<string | null>(null);
  let busy = $state(false);
  let root: HTMLDivElement | undefined = $state();

  $effect(() => {
    if (!menuOpen) return;
    const onDown = (e: MouseEvent) => {
      if (root && !root.contains(e.target as Node)) menuOpen = false;
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') menuOpen = false;
    };
    window.addEventListener('mousedown', onDown);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('mousedown', onDown);
      window.removeEventListener('keydown', onKey);
    };
  });

  function focusEl(node: HTMLInputElement) {
    node.focus();
    node.select();
  }

  function startRename() {
    menuOpen = false;
    err = null;
    draft = name;
    editing = true;
  }

  function startRemove() {
    menuOpen = false;
    err = null;
    confirming = true;
  }

  async function saveRename() {
    const v = draft.trim();
    if (!v) {
      err = 'Nama tidak boleh kosong';
      return;
    }
    const ws = get(activeWorkspaceId);
    if (!ws) {
      err = 'Tidak ada workspace aktif';
      return;
    }
    busy = true;
    try {
      await larkSourceRename(ws, id, v);
      err = null;
      editing = false;
      void refreshSources();
    } catch (e) {
      err = parseLarkError(e).message;
    } finally {
      busy = false;
    }
  }

  async function doRemove() {
    const ws = get(activeWorkspaceId);
    if (!ws) {
      err = 'Tidak ada workspace aktif';
      return;
    }
    busy = true;
    try {
      await larkSourceRemove(ws, id);
      confirming = false;
      await refreshSources();
    } catch (e) {
      err = parseLarkError(e).message;
    } finally {
      busy = false;
    }
  }

  function cancelEdit() {
    editing = false;
    err = null;
  }
</script>

<div class="item" bind:this={root}>
  {#if confirming}
    <div class="confirm">
      <span>Hapus '{name}'?</span>
      <button type="button" class="danger" disabled={busy} onclick={doRemove}>Hapus</button>
      <button type="button" disabled={busy} onclick={() => { confirming = false; err = null; }}>Batal</button>
    </div>
  {:else if editing}
    <input
      class="edit"
      type="text"
      aria-label="Nama baru"
      bind:value={draft}
      use:focusEl
      disabled={busy}
      onkeydown={(e) => {
        if (e.key === 'Enter') void saveRename();
        else if (e.key === 'Escape') cancelEdit();
      }}
      onblur={cancelEdit}
    />
  {:else}
    <div class="line">
      <button type="button" class="open" title={host ? `${name} (${host})` : name} onclick={onopen}>
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M5 4h9a4 4 0 0 1 4 4v12H9a4 4 0 0 1-4-4z" />
        </svg>
        <span class="name">{name}</span>
      </button>
      <button type="button" class="more" aria-label={`Aksi ${label} ${name}`} aria-haspopup="menu" aria-expanded={menuOpen} onclick={() => (menuOpen = !menuOpen)}>...</button>
    </div>
    {#if menuOpen}
      <div class="menu" role="menu">
        <button type="button" role="menuitem" onclick={startRename}>Ganti nama</button>
        <button type="button" role="menuitem" onclick={startRemove}>Hapus</button>
      </div>
    {/if}
  {/if}
  {#if err}<div class="err" role="alert">{err}</div>{/if}
</div>

<style>
  .item {
    position: relative;
    display: flex;
    flex-direction: column;
    font-size: 13px;
    color: var(--fg, #ddd);
  }
  .line {
    display: flex;
    align-items: center;
  }
  .open {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 6px;
    background: none;
    border: none;
    color: inherit;
    text-align: left;
    padding: 4px 6px;
    cursor: pointer;
    border-radius: 4px;
    font-size: 13px;
  }
  .open:hover,
  .more:hover {
    background: var(--bg-hover, #2c2c2c);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .more {
    background: none;
    border: none;
    color: var(--fg-muted, #888);
    cursor: pointer;
    padding: 2px 6px;
    border-radius: 4px;
  }
  .menu {
    position: absolute;
    right: 0;
    top: 100%;
    z-index: 10;
    display: flex;
    flex-direction: column;
    background: var(--bg-elev, #222);
    border: 1px solid var(--border, #333);
    border-radius: 4px;
    min-width: 110px;
  }
  .menu button {
    background: none;
    border: none;
    color: var(--fg, #ddd);
    text-align: left;
    padding: 5px 10px;
    cursor: pointer;
    font-size: 12px;
  }
  .menu button:hover {
    background: var(--bg-hover, #2c2c2c);
  }
  .edit {
    margin: 2px 4px;
    background: var(--bg-elev, #222);
    color: var(--fg, #ddd);
    border: 1px solid var(--border, #333);
    border-radius: 4px;
    padding: 3px 6px;
    font-size: 13px;
  }
  .confirm {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 3px 6px;
    flex-wrap: wrap;
  }
  .confirm button {
    background: var(--bg-elev, #222);
    color: var(--fg, #ddd);
    border: 1px solid var(--border, #333);
    border-radius: 4px;
    padding: 2px 8px;
    font-size: 12px;
    cursor: pointer;
  }
  .confirm .danger {
    color: var(--danger, #e49a9a);
  }
  .err {
    color: var(--danger, #e49a9a);
    font-size: 11px;
    padding: 0 6px 2px;
  }
</style>
