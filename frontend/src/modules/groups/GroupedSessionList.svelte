<script lang="ts">
  import {
    activeGroupId,
    groupSessionCounts,
    groups,
    mainGroupId,
    sessionGroupMap,
    setActiveGroup,
  } from '../../lib/stores/groups';
  import type { Session } from '../../lib/stores';
  import { focusOrOpenSession, sessionTitle } from '../shell/helpers';
  import type { SessionListWrapperProps } from '../sidebar-left/registry';
  import { startSessionDrag } from './sessionDrag';

  let { onerror, list }: SessionListWrapperProps = $props();

  let collapsed = $state<Record<string, boolean>>({});

  const ordered = $derived.by(() => {
    const a = $groups.find((g) => g.id === $activeGroupId);
    return a ? [a, ...$groups.filter((g) => g.id !== a.id)] : $groups;
  });
  const known = $derived(new Set($groups.map((g) => g.id)));

  function isOpen(id: string): boolean {
    const v = collapsed[id];
    return v === undefined ? id === $activeGroupId : !v;
  }
  function toggle(id: string) {
    collapsed = { ...collapsed, [id]: isOpen(id) };
  }

  // Grup aktif baru otomatis terbuka.
  $effect(() => {
    const id = $activeGroupId;
    if (id && collapsed[id] !== undefined) {
      const { [id]: _drop, ...rest } = collapsed;
      collapsed = rest;
    }
  });

  function filterFor(gid: string): (s: Session) => boolean {
    const map = $sessionGroupMap;
    const k = known;
    const main = $mainGroupId;
    return (s) => {
      const e = map[s.id];
      return (e && k.has(e) ? e : main) === gid;
    };
  }

  const msg = (e: unknown) => (e instanceof Error ? e.message : String(e));

  async function onRowClickCapture(e: MouseEvent) {
    const t = e.target as HTMLElement | null;
    const row = t?.closest('[data-session-id]') as HTMLElement | null;
    if (!row) return;
    const gid = (t!.closest('[data-group-section-id]') as HTMLElement | null)?.dataset.groupSectionId;
    if (!gid || gid === $activeGroupId) return;
    const ctl = t!.closest('button, [role=menu]');
    if (ctl && ctl !== row && row.contains(ctl)) {
      // hanya tombol badan baris yang ditangani; kontrol "..." dibiarkan
      if (!ctl.classList.contains('main')) return;
    }
    const sessionId = row.dataset.sessionId!;
    e.stopPropagation();
    e.preventDefault();
    try {
      await setActiveGroup(gid);
    } catch (err) {
      onerror(msg(err));
      return;
    }
    focusOrOpenSession(sessionId);
  }

  function onRowPointerDown(e: PointerEvent) {
    if (e.button !== 0) return;
    const t = e.target as HTMLElement | null;
    const row = t?.closest('[data-session-id]') as HTMLElement | null;
    if (!row) return;
    const ctl = t!.closest('button, input, [role=menu]');
    if (ctl && ctl !== row && !ctl.classList.contains('main')) return;
    const id = row.dataset.sessionId!;
    startSessionDrag(e, id, sessionTitle(id));
  }
</script>

<div class="grouped-list" role="presentation" onclickcapture={onRowClickCapture} onpointerdown={onRowPointerDown}>
  {#each ordered as g (g.id)}
    {@const count = $groupSessionCounts[g.id] ?? 0}
    {@const open = isOpen(g.id)}
    <section class="gl-section" class:active={g.id === $activeGroupId} data-group-section-id={g.id}>
      <button type="button" class="gl-head" aria-expanded={open} onclick={() => toggle(g.id)}>
        <span class="chev" aria-hidden="true">{open ? '▾' : '▸'}</span>
        <span class="name">{g.name}</span>
        <span class="count">{count}</span>
      </button>
      {#if open}
        <div class="gl-body">
          {@render list(filterFor(g.id))}
          {#if count === 0}<div class="empty">Belum ada sesi</div>{/if}
        </div>
      {/if}
    </section>
  {/each}
</div>

<style>
  .grouped-list { display: flex; flex-direction: column; gap: 0.25rem; min-width: 0; }
  .gl-section.active > .gl-head { color: var(--fg); box-shadow: inset 2px 0 0 var(--accent); }
  .gl-head { display: flex; align-items: center; gap: 0.375rem; width: 100%; font: inherit; font-size: 0.6875rem; text-transform: uppercase; letter-spacing: 0.04em; color: var(--fg-muted); background: transparent; border: 0; border-radius: 0.25rem; padding: 0.25rem 0.5rem; cursor: pointer; text-align: left; }
  .gl-head:hover { background: var(--bg-hover); }
  .chev { flex: none; width: 0.75rem; }
  .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .count { flex: none; font-size: 0.625rem; color: var(--fg-muted); }
  .empty { font-size: 0.6875rem; color: var(--fg-muted); padding: 0.25rem 0.875rem; }
</style>
