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

  const curId = get(activeWorkspaceId);
  const cur = list.find((w) => w.id === curId);
  if (!cur) {
    const first = list[0] ?? null;
    activeWorkspaceId.set(first ? first.id : null);
    activeProjectId.set(firstProjectId(first));
    await loadSessions();
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
  const rest = get(workspaces).filter((w) => w.id !== id);
  workspaces.set(rest);
  if (get(activeWorkspaceId) === id) {
    if (rest.length > 0) {
      await selectWorkspace(rest[0].id);
    } else {
      activeWorkspaceId.set(null);
      activeProjectId.set(null);
      sessions.set([]);
    }
  }
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
