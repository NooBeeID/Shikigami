import { derived, get, writable, type Readable } from 'svelte/store';
import { call, onEvent } from '../ipc';
import { activeWorkspace, activeWorkspaceId, workspaces } from './workspaces';
import { sessions } from './sessions';
import { layout } from './layout';
import { flushNow } from './persist';
import { providers } from './providers';
import type { Workspace, WorkspaceGroup } from './types';
import { closePane as closePaneDoc, findLeafBy, hydrateDoc, leaves } from '../../modules/layout/model';
import { switchGroup } from '../../modules/persist/autosave';

export interface SessionGroup { id: string; name: string; order: number; main: boolean }
export const MAIN_GROUP_NAME = 'Utama';

interface GroupsState {
  workspaceId: string;
  activeGroupId: string;
  groups: Array<{ id: string; name: string; order: number; main?: boolean }>;
  membership: Record<string, string>;
}

const errStore = writable<string | null>(null);

export const groups: Readable<SessionGroup[]> = derived(activeWorkspace, (ws) =>
  [...(ws?.groups ?? [])]
    .map((g, i) => ({ g, i }))
    .sort((a, b) => a.g.order - b.g.order || a.i - b.i)
    .map(({ g }) => ({ id: g.id, name: g.name, order: g.order, main: !!g.main })),
);
export const mainGroupId: Readable<string> = derived(
  groups,
  ($g) => $g.find((g) => g.main)?.id ?? $g[0]?.id ?? '',
);
export const activeGroupId: Readable<string> = derived(
  [activeWorkspace, groups, mainGroupId],
  ([ws, $g, $main]) => (ws?.activeGroupId && $g.some((g) => g.id === ws.activeGroupId) ? ws.activeGroupId : $main),
);
export const activeGroup: Readable<SessionGroup | undefined> = derived(
  [groups, activeGroupId],
  ([$g, $a]) => $g.find((g) => g.id === $a),
);
export const sessionGroupMap: Readable<Record<string, string>> = derived(
  [activeWorkspace, sessions, groups, mainGroupId],
  ([ws, $s, $g, $main]) => {
    const out: Record<string, string> = {};
    if (!ws || $g.length === 0) return out;
    for (const s of $s) {
      if (s.workspaceId !== ws.id) continue;
      const gid = ws.membership?.[s.id];
      out[s.id] = gid && $g.some((g) => g.id === gid) ? gid : $main;
    }
    return out;
  },
);
export const groupSessionCounts: Readable<Record<string, number>> = derived(
  [groups, sessionGroupMap],
  ([$g, $m]) => {
    const out: Record<string, number> = {};
    for (const g of $g) out[g.id] = 0;
    for (const id of Object.keys($m)) out[$m[id]] = (out[$m[id]] ?? 0) + 1;
    return out;
  },
);
export const groupsError: Readable<string | null> = { subscribe: errStore.subscribe };

export function groupOfSession(sessionId: string): string | undefined {
  return get(sessionGroupMap)[sessionId];
}

function applyGroupsState(state: GroupsState): void {
  if (!state || typeof state.workspaceId !== 'string' || !Array.isArray(state.groups)) return;
  if (!get(workspaces).some((w) => w.id === state.workspaceId)) return;
  workspaces.update((list) =>
    list.map((w): Workspace => {
      if (w.id !== state.workspaceId) return w;
      const old = new Map((w.groups ?? []).map((g) => [g.id, g]));
      const next: WorkspaceGroup[] = state.groups.map((g) => {
        const o = old.get(g.id);
        const ng: WorkspaceGroup = { id: g.id, name: g.name, order: g.order, main: !!g.main };
        if (o && o.layout !== undefined) ng.layout = o.layout;
        return ng;
      });
      return { ...w, groups: next, activeGroupId: state.activeGroupId, membership: state.membership ?? {} };
    }),
  );
}

async function run<T>(label: string, fn: () => Promise<T>, fallback: T): Promise<T> {
  try {
    const res = await fn();
    errStore.set(null);
    return res;
  } catch (e) {
    const message = e instanceof Error && e.message ? e.message : `Gagal ${label}`;
    errStore.set(message);
    console.warn(`[groups] gagal ${label}`, e);
    return fallback;
  }
}

function wsId(): string {
  const id = get(activeWorkspaceId);
  if (!id) throw new Error('Pilih workspace dulu');
  return id;
}

async function groupCall<T = object>(method: string, extra: Record<string, unknown>): Promise<GroupsState & T> {
  const workspaceId = wsId();
  const res = await call<GroupsState & T>(method, { workspaceId, ...extra });
  if (res && !res.workspaceId) res.workspaceId = workspaceId;
  applyGroupsState(res);
  return res;
}

let switchChain: Promise<unknown> = Promise.resolve();

async function doSwitch(id: string): Promise<boolean> {
  const workspaceId = wsId();
  if (id === get(activeGroupId)) return true;
  if (!get(groups).some((g) => g.id === id)) throw new Error('Grup tidak ditemukan');
  const prev = get(activeGroupId);
  await flushNow();
  await groupCall('workspace.group.setActive', { groupId: id });
  let res: { layout?: unknown };
  try {
    res = await call<{ layout?: unknown }>('workspace.layout.get', { workspaceId, groupId: id });
  } catch (e) {
    try {
      await groupCall('workspace.group.setActive', { groupId: prev });
    } catch {
      /* abaikan */
    }
    throw e;
  }
  if (get(activeWorkspaceId) !== workspaceId) return true;
  switchGroup(id, res?.layout ?? null);
  return true;
}

function setActiveGroupInner(id: string): Promise<boolean> {
  const p = switchChain.then(() => doSwitch(id));
  switchChain = p.catch(() => {});
  return p;
}

export async function setActiveGroup(id: string): Promise<void> {
  await run('mengganti grup', () => setActiveGroupInner(id), false);
}

export async function createGroup(name?: string): Promise<string> {
  return run(
    'membuat grup',
    async () => {
      const res = await groupCall<{ groupId: string }>('workspace.group.create', name ? { name } : {});
      const groupId = res.groupId;
      await setActiveGroupInner(groupId);
      return groupId;
    },
    '',
  );
}

export async function renameGroup(id: string, name: string): Promise<void> {
  await run('mengganti nama grup', () => groupCall('workspace.group.rename', { groupId: id, name }), null);
}

export async function reorderGroup(id: string, toIndex: number): Promise<void> {
  await run('mengurutkan grup', () => groupCall('workspace.group.reorder', { groupId: id, toIndex }), null);
}

export async function deleteGroup(id: string): Promise<void> {
  await run(
    'menghapus grup',
    async () => {
      const isMain = get(groups).find((g) => g.id === id)?.main;
      if (id === get(activeGroupId) && !isMain) await setActiveGroupInner(get(mainGroupId));
      return groupCall('workspace.group.delete', { groupId: id });
    },
    null,
  );
}

export async function moveSession(sessionId: string, toGroupId: string): Promise<void> {
  await run(
    'memindahkan sesi',
    async () => {
      const workspaceId = wsId();
      const from = groupOfSession(sessionId) ?? get(mainGroupId);
      if (!get(groups).some((g) => g.id === toGroupId)) throw new Error('Grup tujuan tidak ditemukan');
      if (from === toGroupId) return null;
      const isTerm = (l: { kind: string; sessionId?: string }) => l.kind === 'terminal' && l.sessionId === sessionId;
      if (from === get(activeGroupId)) {
        for (let leaf = findLeafBy(get(layout), isTerm); leaf; leaf = findLeafBy(get(layout), isTerm)) {
          layout.closePane(leaf.id);
        }
      } else {
        const res = await call<{ layout?: unknown }>('workspace.layout.get', { workspaceId, groupId: from });
        let doc = hydrateDoc(res?.layout ?? null);
        let closed = false;
        for (let leaf = leaves(doc).find(isTerm); leaf; leaf = leaves(doc).find(isTerm)) {
          doc = closePaneDoc(doc, leaf.id);
          closed = true;
        }
        if (closed) await call('workspace.layout.set', { workspaceId, groupId: from, layout: doc });
      }
      await groupCall('workspace.group.moveSession', { sessionId, toGroupId });
      if (toGroupId === get(activeGroupId) && !findLeafBy(get(layout), isTerm)) {
        const s = get(sessions).find((x) => x.id === sessionId);
        if (s) {
          const title = get(providers).find((p) => p.id === s.providerId)?.name ?? s.providerId;
          layout.openPane({ kind: 'terminal', sessionId, projectId: s.projectId, provider: s.providerId, title });
        }
      }
      return null;
    },
    null,
  );
}

let teardown: (() => void) | null = null;

export function initGroups(): () => void {
  if (teardown) return teardown;
  let known = new Set<string>();
  let pending = true;
  let curWs: string | null | undefined = undefined;
  const assigning = new Set<string>();

  const off = onEvent('workspace.groupsChanged', (_topic, payload) => applyGroupsState(payload as GroupsState));

  const unsubA = activeWorkspaceId.subscribe((id) => {
    if (id === curWs) return;
    curWs = id;
    known = new Set();
    pending = true;
  });

  const unsubB = sessions.subscribe((list) => {
    const ws = get(activeWorkspaceId);
    if (!ws) return;
    const mine = list.filter((s) => s.workspaceId === ws);
    if (list.length !== mine.length) return;
    if (pending) {
      for (const s of mine) known.add(s.id);
      pending = false;
      return;
    }
    for (const s of mine) {
      if (known.has(s.id)) continue;
      known.add(s.id);
      const gid = get(activeGroupId);
      if (get(activeWorkspace)?.membership?.[s.id]) continue;
      if (!gid || gid === get(mainGroupId) || assigning.has(s.id)) continue;
      assigning.add(s.id);
      void run('memindahkan sesi', () => groupCall('workspace.group.moveSession', { sessionId: s.id, toGroupId: gid }), null)
        .finally(() => assigning.delete(s.id));
    }
  });

  teardown = () => {
    off();
    unsubA();
    unsubB();
    known = new Set();
    pending = true;
    curWs = undefined;
    assigning.clear();
    teardown = null;
  };
  return teardown;
}
