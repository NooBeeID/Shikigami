// Logika layar welcome: daftar "Terakhir dibuka", pencocokan path, dan aksi buka/buat workspace.
import { get } from 'svelte/store';
import { workspaces, openWorkspace, createWorkspace } from '../../lib/stores';
import type { Workspace } from '../../lib/stores';
import { newWorkspaceFromFolder, errMessage } from '../openfolder/logic';
import { pickDirectory } from '../appmenu';
import { showToast } from '../openfolder/toast-store';
import { defaultWorkspaceName } from '../topbar/util';

// Buang slash/backslash di akhir (root tetap utuh).
function normalizePath(p: string): string {
  let s = p;
  while (s.length > 1 && (s.endsWith('/') || s.endsWith('\\'))) s = s.slice(0, -1);
  return s;
}

// Workspace yang pernah dibuka dan belum terbuka sebagai tab, terbaru dulu.
export function recentWorkspaces(workspaces: Workspace[], openIds: string[], limit = 5): Workspace[] {
  return workspaces
    .filter((w) => (w.lastOpenedAt ?? 0) > 0 && !openIds.includes(w.id))
    .sort((a, b) => (b.lastOpenedAt ?? 0) - (a.lastOpenedAt ?? 0))
    .slice(0, limit);
}

// Cari workspace yang punya project dengan path ini.
export function findWorkspaceByPath(workspaces: Workspace[], path: string): Workspace | undefined {
  const target = normalizePath(path);
  return workspaces.find((w) => (w.projects ?? []).some((p) => normalizePath(p.path) === target));
}

let busy = false;

export async function openFromDisk(): Promise<void> {
  if (busy) return;
  busy = true;
  try {
    const path = await pickDirectory();
    if (!path) return;
    const existing = findWorkspaceByPath(get(workspaces), path);
    if (existing) await openWorkspace(existing.id);
    else await newWorkspaceFromFolder(path);
  } catch (e) {
    showToast(errMessage(e), 'error');
  } finally {
    busy = false;
  }
}

// Tidak membuka workspace; UI membukanya setelah input nama inline.
export async function createNew(): Promise<Workspace | null> {
  try {
    return await createWorkspace(defaultWorkspaceName(get(workspaces)));
  } catch (e) {
    showToast(errMessage(e), 'error');
    return null;
  }
}

export async function openRecent(id: string): Promise<void> {
  try {
    await openWorkspace(id);
  } catch (e) {
    showToast(errMessage(e), 'error');
  }
}
