<script lang="ts">
  import {
    MAIN_GROUP_NAME,
    activeGroupId,
    createGroup,
    deleteGroup,
    groupSessionCounts,
    groups,
    groupsError,
    renameGroup,
    reorderGroup,
    setActiveGroup,
  } from '../../lib/stores/groups';
  import { groupDropState, watchPaneDrag } from './sessionDrag';

  let root: HTMLDivElement | undefined = $state();
  let editingId = $state<string | null>(null);
  let draft = $state('');
  let menuFor = $state<string | null>(null);
  let menuPos = $state({ x: 0, y: 0 });
  let confirmDeleteId = $state<string | null>(null);
  let reorderDrag = $state<{ id: string; overIndex: number } | null>(null);
  let localError = $state<string | null>(null);
  let pendingEditId = $state<string | null>(null);
  let menuEl: HTMLDivElement | undefined = $state();

  const shownError = $derived($groupsError ?? localError);
  const menuGroup = $derived($groups.find((g) => g.id === menuFor));
  const confirmGroup = $derived($groups.find((g) => g.id === confirmDeleteId));

  $effect(() => watchPaneDrag());

  // Gulirkan tab aktif ke tampilan.
  $effect(() => {
    const id = $activeGroupId;
    void $groups;
    if (!root || !id) return;
    root.querySelector<HTMLElement>(`[data-group-tab-id="${CSS.escape(id)}"]`)?.scrollIntoView({ inline: 'nearest', block: 'nearest' });
  });

  // Edit nama untuk grup yang baru dibuat setelah muncul di store.
  $effect(() => {
    if (pendingEditId && $groups.some((g) => g.id === pendingEditId)) {
      const g = $groups.find((x) => x.id === pendingEditId)!;
      pendingEditId = null;
      editingId = g.id;
      draft = g.name;
    }
  });

  function focusSelect(node: HTMLInputElement) {
    node.focus();
    node.select();
  }

  function startEdit(id: string) {
    const g = $groups.find((x) => x.id === id);
    if (!g) return;
    closeMenu();
    editingId = id;
    draft = g.name;
  }

  async function commitEdit(fromBlur = false) {
    const id = editingId;
    if (!id) return;
    const g = $groups.find((x) => x.id === id);
    const name = draft.trim();
    if (!name) {
      localError = 'Nama grup tidak boleh kosong';
      if (fromBlur) editingId = null;
      return;
    }
    if (g && name === g.name) {
      editingId = null;
      return;
    }
    localError = null;
    editingId = null;
    try {
      await renameGroup(id, name);
    } catch {
      // tampil lewat groupsError
    }
  }

  function onInputKey(e: KeyboardEvent) {
    e.stopPropagation();
    if (e.key === 'Enter') {
      e.preventDefault();
      void commitEdit();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      editingId = null;
      localError = null;
    }
  }

  function onInputBlur() {
    if (!editingId) return;
    const g = $groups.find((x) => x.id === editingId);
    if (!draft.trim() || (g && draft.trim() === g.name)) {
      editingId = null;
      return;
    }
    void commitEdit(true);
  }

  let swallowNextClick = false;

  async function activate(id: string) {
    if (swallowNextClick || editingId) return;
    try {
      await setActiveGroup(id);
    } catch {
      // tampil lewat groupsError
    }
  }

  async function addGroup() {
    try {
      pendingEditId = await createGroup();
    } catch {
      // tampil lewat groupsError
    }
  }

  function openMenuAt(id: string, x: number, y: number) {
    confirmDeleteId = null;
    menuFor = id;
    menuPos = { x, y };
    queueMicrotask(placeMenu);
  }

  function openMenuFor(id: string, el: HTMLElement) {
    const r = el.getBoundingClientRect();
    openMenuAt(id, r.left, r.bottom + 4);
  }

  function placeMenu() {
    if (!menuEl) return;
    const w = menuEl.offsetWidth;
    const h = menuEl.offsetHeight;
    menuPos = {
      x: Math.max(4, Math.min(menuPos.x, window.innerWidth - w - 4)),
      y: Math.max(4, Math.min(menuPos.y, window.innerHeight - h - 4)),
    };
    menuEl.querySelector<HTMLElement>('button')?.focus();
  }

  function closeMenu() {
    menuFor = null;
  }

  $effect(() => {
    if (!menuFor && !confirmDeleteId) return;
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node | null;
      if (t && (menuEl?.contains(t) || (t as HTMLElement).closest?.('.confirm'))) return;
      closeMenu();
      confirmDeleteId = null;
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        closeMenu();
        confirmDeleteId = null;
      }
    };
    window.addEventListener('pointerdown', onDown);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('pointerdown', onDown);
      window.removeEventListener('keydown', onKey);
    };
  });

  function requestDelete(id: string) {
    const g = $groups.find((x) => x.id === id);
    if (!g || g.main) return;
    closeMenu();
    if (($groupSessionCounts[id] ?? 0) > 0) {
      confirmDeleteId = id;
    } else {
      void doDelete(id);
    }
  }

  async function doDelete(id: string) {
    confirmDeleteId = null;
    try {
      await deleteGroup(id);
    } catch {
      // tampil lewat groupsError
    }
  }

  function tabEls(): HTMLElement[] {
    return root ? Array.from(root.querySelectorAll<HTMLElement>('[data-group-tab-id]')) : [];
  }

  async function onTabKey(e: KeyboardEvent, id: string) {
    if (editingId) return;
    const list = $groups;
    const i = list.findIndex((g) => g.id === id);
    const go = async (n: number) => {
      const target = list[n];
      if (!target) return;
      e.preventDefault();
      tabEls().find((el) => el.dataset.groupTabId === target.id)?.focus();
      try {
        await setActiveGroup(target.id);
      } catch {
        // tampil lewat groupsError
      }
    };
    if (e.key === 'ArrowRight') await go((i + 1) % list.length);
    else if (e.key === 'ArrowLeft') await go((i - 1 + list.length) % list.length);
    else if (e.key === 'Home') await go(0);
    else if (e.key === 'End') await go(list.length - 1);
    else if (e.key === 'F2') {
      e.preventDefault();
      startEdit(id);
    } else if (e.key === 'Delete') {
      e.preventDefault();
      requestDelete(id);
    } else if (e.key === 'ContextMenu' || (e.key === 'F10' && e.shiftKey)) {
      e.preventDefault();
      openMenuFor(id, e.currentTarget as HTMLElement);
    }
  }

  // Drag urut tab.
  function insertIndex(x: number, dragId: string): number {
    const others = tabEls().filter((el) => el.dataset.groupTabId !== dragId);
    let idx = 0;
    for (const el of others) {
      const r = el.getBoundingClientRect();
      if (x > r.left + r.width / 2) idx++;
    }
    return idx;
  }

  function onTabPointerDown(e: PointerEvent, id: string) {
    if (e.button !== 0 || editingId) return;
    const t = e.target as HTMLElement;
    if (t.closest('input, .more')) return;
    const startX = e.clientX;
    let started = false;
    const onMove = (ev: PointerEvent) => {
      if (!started) {
        if (Math.abs(ev.clientX - startX) < 4) return;
        started = true;
      }
      reorderDrag = { id, overIndex: insertIndex(ev.clientX, id) };
    };
    const end = () => {
      window.removeEventListener('pointermove', onMove, true);
      window.removeEventListener('pointerup', onUp, true);
      window.removeEventListener('pointercancel', onCancel, true);
      window.removeEventListener('keydown', onKey, true);
    };
    const finish = (commit: boolean) => {
      const d = reorderDrag;
      end();
      reorderDrag = null;
      if (!started) return;
      swallowNextClick = true;
      setTimeout(() => (swallowNextClick = false), 0);
      const from = $groups.findIndex((g) => g.id === id);
      if (commit && d && d.overIndex !== from) {
        reorderGroup(id, d.overIndex).catch(() => {});
      }
    };
    const onUp = () => finish(true);
    const onCancel = () => finish(false);
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key === 'Escape') finish(false);
    };
    window.addEventListener('pointermove', onMove, true);
    window.addEventListener('pointerup', onUp, true);
    window.addEventListener('pointercancel', onCancel, true);
    window.addEventListener('keydown', onKey, true);
  }

  // Penanda sisip: kelas pada tab target.
  function insertClass(id: string): string {
    const d = reorderDrag;
    if (!d || d.id === id) return '';
    const order = $groups.filter((g) => g.id !== d.id);
    const pos = order.findIndex((g) => g.id === id);
    if (pos === d.overIndex) return 'insert-before';
    if (pos === order.length - 1 && d.overIndex >= order.length) return 'insert-after';
    return '';
  }
</script>

<div class="group-tabs" role="tablist" aria-label="Grup sesi" bind:this={root}>
  {#each $groups as g (g.id)}
    <div class="tab-wrap">
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <div
        class="tab {insertClass(g.id)}"
        class:active={g.id === $activeGroupId}
        class:drop={!!$groupDropState}
        class:drop-over={$groupDropState?.overGroupId === g.id}
        class:dragging={reorderDrag?.id === g.id}
        role="tab"
        tabindex={g.id === $activeGroupId ? 0 : -1}
        aria-selected={g.id === $activeGroupId}
        data-group-tab-id={g.id}
        onclick={() => activate(g.id)}
        ondblclick={() => startEdit(g.id)}
        oncontextmenu={(e) => { e.preventDefault(); openMenuAt(g.id, e.clientX, e.clientY); }}
        onkeydown={(e) => onTabKey(e, g.id)}
        onpointerdown={(e) => onTabPointerDown(e, g.id)}
      >
        {#if editingId === g.id}
          <input
            class="edit"
            aria-label="Nama grup"
            bind:value={draft}
            use:focusSelect
            onkeydown={onInputKey}
            onpointerdown={(e) => e.stopPropagation()}
            ondblclick={(e) => e.stopPropagation()}
            onclick={(e) => e.stopPropagation()}
            onblur={onInputBlur}
          />
        {:else}
          <span class="label" title={g.name}>{g.name}</span>
          <span class="badge">{$groupSessionCounts[g.id] ?? 0}</span>
          <button
            type="button"
            class="more"
            aria-label="Menu grup"
            aria-haspopup="menu"
            aria-expanded={menuFor === g.id}
            tabindex="-1"
            onclick={(e) => { e.stopPropagation(); openMenuFor(g.id, e.currentTarget as HTMLElement); }}
          >…</button>
        {/if}
      </div>
    </div>
  {/each}
  <button type="button" class="add" aria-label="Grup baru" title="Grup baru" onclick={addGroup}>+</button>

  {#if shownError}
    <span class="err" role="status" title={shownError}>
      <span class="err-text">{shownError}</span>
      {#if !$groupsError && localError}
        <button type="button" class="err-x" aria-label="Tutup pesan" onclick={() => (localError = null)}>×</button>
      {/if}
    </span>
  {/if}
</div>

{#if menuFor && menuGroup}
  <div class="menu" role="menu" bind:this={menuEl} style="left:{menuPos.x}px;top:{menuPos.y}px">
    <button type="button" role="menuitem" onclick={() => startEdit(menuGroup.id)}>Ganti nama</button>
    {#if !menuGroup.main}
      <button type="button" role="menuitem" class="danger" onclick={() => requestDelete(menuGroup.id)}>Hapus</button>
    {/if}
  </div>
{/if}

{#if confirmGroup}
  <div class="confirm" role="alertdialog" aria-label="Konfirmasi hapus grup">
    <span>Hapus grup ini? Sesi di dalamnya dipindah ke "{MAIN_GROUP_NAME}" dan tidak dihentikan.</span>
    <button type="button" class="danger" onclick={() => doDelete(confirmGroup.id)}>Hapus</button>
    <button type="button" onclick={() => (confirmDeleteId = null)}>Batal</button>
  </div>
{/if}

<style>
  .group-tabs { display: flex; flex-wrap: nowrap; align-items: stretch; overflow-x: auto; background: var(--bg-elev); border-bottom: 1px solid var(--border); min-height: 2rem; }
  .tab-wrap { flex: 0 0 auto; display: flex; }
  .tab { position: relative; display: flex; align-items: center; gap: 0.375rem; padding: 0 0.625rem; font-size: 0.75rem; color: var(--fg-muted); cursor: pointer; border-bottom: 2px solid transparent; user-select: none; outline: none; }
  .tab:hover, .tab:focus-visible { background: var(--bg-hover); }
  .tab.active { color: var(--accent); border-bottom-color: var(--accent); }
  .tab.drop { outline: 1px dashed var(--border); outline-offset: -3px; }
  .tab.drop-over { background: var(--bg-hover); outline: 1px solid var(--accent); outline-offset: -2px; }
  .tab.dragging { opacity: 0.5; }
  .tab.insert-before::before, .tab.insert-after::after { content: ''; position: absolute; top: 0.25rem; bottom: 0.25rem; width: 2px; background: var(--accent); }
  .tab.insert-before::before { left: 0; }
  .tab.insert-after::after { right: 0; }
  .label { max-width: 10rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .badge { font-size: 0.625rem; color: var(--fg-muted); }
  .more { visibility: hidden; font: inherit; color: var(--fg-muted); background: transparent; border: 0; border-radius: 0.25rem; padding: 0 0.25rem; cursor: pointer; }
  .tab:hover .more, .tab:focus-within .more, .tab.active .more { visibility: visible; }
  .more:hover { color: var(--fg); background: var(--bg-hover); }
  .edit { font: inherit; font-size: 0.75rem; color: var(--fg); background: var(--bg); border: 1px solid var(--accent); border-radius: 0.25rem; padding: 0.125rem 0.25rem; width: 9rem; outline: none; }
  .add { flex: 0 0 auto; font: inherit; font-size: 0.875rem; color: var(--fg-muted); background: transparent; border: 0; padding: 0 0.75rem; cursor: pointer; }
  .add:hover { color: var(--fg); background: var(--bg-hover); }
  .err { margin-left: auto; flex: 0 1 auto; min-width: 0; display: flex; align-items: center; gap: 0.25rem; padding: 0 0.5rem; font-size: 0.6875rem; color: var(--danger); }
  .err-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 20rem; }
  .err-x { font: inherit; color: var(--danger); background: transparent; border: 0; cursor: pointer; padding: 0 0.25rem; }
  .menu { position: fixed; z-index: 9000; min-width: 9rem; padding: 0.25rem; display: flex; flex-direction: column; background: var(--bg-elev); border: 1px solid var(--border); border-radius: 0.375rem; box-shadow: 0 4px 14px color-mix(in srgb, var(--fg) 30%, transparent); color: var(--fg); font-size: 0.75rem; }
  .menu button, .confirm button { font: inherit; color: inherit; background: none; border: 0; text-align: left; padding: 0.375rem 0.625rem; border-radius: 0.25rem; cursor: pointer; }
  .menu button:hover, .menu button:focus-visible { background: var(--bg-hover); outline: none; }
  .danger { color: var(--danger) !important; }
  .confirm { position: fixed; z-index: 9000; left: 50%; top: 2.5rem; transform: translateX(-50%); display: flex; align-items: center; gap: 0.5rem; padding: 0.5rem 0.75rem; background: var(--bg-elev); border: 1px solid var(--border); border-radius: 0.375rem; box-shadow: 0 4px 14px color-mix(in srgb, var(--fg) 30%, transparent); color: var(--fg); font-size: 0.75rem; max-width: 90vw; }
  .confirm button { border: 1px solid var(--border); text-align: center; }
  .confirm button:hover { background: var(--bg-hover); }
</style>
