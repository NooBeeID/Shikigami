import { writable, derived, get, type Readable, type Writable } from 'svelte/store';
import { call } from '../ipc';
import type { Workspace, Project } from './types';
import { sessions, loadSessions } from './sessions';
import { normalizeList } from './util';

export const workspaces: Writable<Workspace[]> = writable<Workspace[]>([]);
export const activeWorkspaceId: Writable<string | null> = writable<string | null>(null);
export const activeProjectId: Writable<string | null> = writable<string | null>(null);

export const activeWorkspace: Readable<Workspace | null> = derived(
  [workspaces, activeWorkspaceId],
  ([$w, $id]) => $w.find((x) => x.id === $id) ?? null,
);
export const activeProject: Readable<Project | null> = derived(
  [activeWorkspace, activeProjectId],
  ([$ws, $pid]) => $ws?.projects?.find((p) => p.id === $pid) ?? null,
);

// Memori saja, tidak dipersist; urutan = urutan tab.
export const openWorkspaceIds: Writable<string[]> = writable<string[]>([]);

// Tetangga kanan, kalau tidak ada tetangga kiri, dari urutan `order` (id sudah ada di dalamnya).
function neighborOf(order: string[], id: string): string | null {
  const i = order.indexOf(id);
  if (i < 0) return null;
  return order[i + 1] ?? order[i - 1] ?? null;
}

function clearActive(): void {
  activeWorkspaceId.set(null);
  activeProjectId.set(null);
  sessions.set([]);
}

function firstProjectId(ws: Workspace | undefined | null): string | null {
  return ws?.projects?.[0]?.id ?? null;
}

function replaceWorkspace(ws: Workspace): void {
  workspaces.update((l) => (l.some((x) => x.id === ws.id) ? l.map((x) => (x.id === ws.id ? ws : x)) : [...l, ws]));
}

// FINAL(shell-layout)
export async function loadWorkspaces(): Promise<void> {
  const res = await call('workspace.list');
  const list = normalizeList<Workspace>(res, 'workspaces');
  workspaces.set(list);
  openWorkspaceIds.update((ids) => ids.filter((x) => list.some((w) => w.id === x)));

  const curId = get(activeWorkspaceId);
  const cur = list.find((w) => w.id === curId);
  if (!cur) {
    clearActive();
    return;
  }
  const pid = get(activeProjectId);
  if (!pid || !cur.projects?.some((p) => p.id === pid)) {
    activeProjectId.set(firstProjectId(cur));
  }
}

// FINAL(shell-layout)
export async function selectWorkspace(id: string): Promise<void> {
  const ws = get(workspaces).find((w) => w.id === id);
  activeWorkspaceId.set(id);
  activeProjectId.set(firstProjectId(ws));
  await loadSessions();
}

// FINAL(foundation): hanya menentukan cwd sesi baru, tidak menyentuh sessions.
export function selectProject(id: string): void {
  activeProjectId.set(id);
}

// FINAL(shell-layout)
export async function createWorkspace(name: string): Promise<Workspace> {
  const res = await call<Workspace | { workspace: Workspace }>('workspace.create', { name });
  const ws = (res && 'workspace' in res ? res.workspace : res) as Workspace;
  workspaces.update((l) => (l.some((x) => x.id === ws.id) ? l : [...l, ws]));
  return ws;
}

// FINAL(shell-layout)
export async function renameWorkspace(id: string, name: string): Promise<void> {
  await call('workspace.rename', { id, name });
  workspaces.update((l) => l.map((w) => (w.id === id ? { ...w, name } : w)));
}

// FINAL(shell-layout)
export async function deleteWorkspace(id: string): Promise<void> {
  await call('workspace.delete', { id });
  const order = get(openWorkspaceIds);
  const wasActive = get(activeWorkspaceId) === id;
  const rest = get(workspaces).filter((w) => w.id !== id);
  // tetangga dihitung dari urutan tab terbuka sebelum id dikeluarkan, hanya id yang masih ada
  const candidates = order.filter((x) => x === id || rest.some((w) => w.id === x));
  const next = neighborOf(candidates, id);
  workspaces.set(rest);
  openWorkspaceIds.set(order.filter((x) => x !== id));
  if (wasActive) {
    if (next) await selectWorkspace(next);
    else clearActive();
  }
}

export async function openWorkspace(id: string): Promise<void> {
  openWorkspaceIds.update((ids) => (ids.includes(id) ? ids : [...ids, id]));
  await selectWorkspace(id);
  try {
    const res = await call<Workspace | { workspace: Workspace }>('workspace.touch', { id });
    const ws = (res && 'workspace' in res ? res.workspace : res) as Workspace | undefined;
    if (ws && ws.id) replaceWorkspace(ws);
  } catch {
    // touch best-effort: gagal tidak membatalkan pembukaan
  }
}

export async function closeWorkspaceTab(id: string): Promise<void> {
  const order = get(openWorkspaceIds);
  const next = neighborOf(order, id);
  openWorkspaceIds.set(order.filter((x) => x !== id));
  if (get(activeWorkspaceId) !== id) return;
  if (next) await selectWorkspace(next);
  else clearActive();
}

// FINAL(shell-layout)
export async function addProject(path: string, name?: string): Promise<Project> {
  const workspaceId = get(activeWorkspaceId);
  if (!workspaceId) throw new Error('pilih workspace dulu');
  const res = await call<{ workspace?: Workspace; project: Project }>('workspace.addProject', {
    workspaceId,
    path,
    name,
  });
  const project = res.project;
  if (res.workspace) {
    replaceWorkspace(res.workspace);
  } else {
    workspaces.update((l) =>
      l.map((w) => (w.id === workspaceId ? { ...w, projects: [...(w.projects ?? []), project] } : w)),
    );
  }
  if (!get(activeProjectId)) activeProjectId.set(project.id);
  return project;
}

// FINAL(shell-layout)
export async function removeProject(id: string): Promise<void> {
  const workspaceId = get(activeWorkspaceId);
  if (!workspaceId) throw new Error('pilih workspace dulu');
  await call('workspace.removeProject', { workspaceId, projectId: id });
  let remaining: Project[] = [];
  workspaces.update((l) =>
    l.map((w) => {
      if (w.id !== workspaceId) return w;
      remaining = (w.projects ?? []).filter((p) => p.id !== id);
      return { ...w, projects: remaining };
    }),
  );
  if (get(activeProjectId) === id) activeProjectId.set(remaining[0]?.id ?? null);
}
