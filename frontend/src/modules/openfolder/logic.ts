import { get } from 'svelte/store';
import { pickDirectory } from '../appmenu';
import {
  activeWorkspace,
  addProject,
  createWorkspace,
  openWorkspace,
} from '../../lib/stores';
import { layout } from '../../lib/stores/layout';
import { findLeaf } from '../layout/model';
import { pasteToSession, formatDroppedPaths } from '../terminal/paste';
import { showToast } from './toast-store';

export function basename(path: string): string {
  let p = path ?? '';
  while (p.endsWith('/') || p.endsWith('\\')) p = p.slice(0, -1);
  const i = Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'));
  const seg = i >= 0 ? p.slice(i + 1) : p;
  return seg || 'root';
}

export function errMessage(e: unknown): string {
  let m: string;
  if (e instanceof Error) m = e.message;
  else if (typeof e === 'string') m = e;
  else m = 'terjadi kesalahan';
  if (!m) m = 'terjadi kesalahan';
  return m.length > 120 ? m.slice(0, 117) + '...' : m;
}

function normalize(p: string): string {
  let s = p;
  while (s.length > 1 && s.endsWith('/')) s = s.slice(0, -1);
  return s;
}

export function isDuplicate(path: string): boolean {
  const ws = get(activeWorkspace);
  if (!ws) return false;
  const n = normalize(path);
  return ws.projects.some((p) => normalize(p.path) === n);
}

export async function newWorkspaceFromFolder(path: string): Promise<void> {
  const ws = await createWorkspace(basename(path));
  await openWorkspace(ws.id);
  await addProject(path);
}

export async function addFolders(paths: string[]): Promise<void> {
  let ok = 0;
  let dupes = 0;
  const failed: string[] = [];
  let rest = paths;

  if (get(activeWorkspace) === null && paths.length > 0) {
    const first = paths[0];
    rest = paths.slice(1);
    try {
      await newWorkspaceFromFolder(first);
      ok++;
    } catch (e) {
      failed.push(`${basename(first)}: ${errMessage(e)}`);
    }
  }

  for (const path of rest) {
    if (isDuplicate(path)) {
      dupes++;
      continue;
    }
    try {
      await addProject(path);
      ok++;
    } catch (e) {
      failed.push(`${basename(path)}: ${errMessage(e)}`);
    }
  }

  if (failed.length > 0) {
    const prefix = ok > 0 ? `${ok} folder ditambahkan. ` : '';
    showToast(`${prefix}Gagal menambahkan: ${failed.join('; ')}`, 'error');
  } else if (ok > 0 && dupes === 0) {
    showToast(`${ok} folder ditambahkan`);
  } else if (ok > 0) {
    showToast(`${ok} folder ditambahkan, ${dupes} sudah menjadi project`);
  } else if (dupes > 0) {
    showToast(dupes === 1 ? 'Folder sudah menjadi project' : `${dupes} folder sudah menjadi project`);
  }
}

let busy = false;

export async function handleOpenFolder(): Promise<void> {
  if (busy) return;
  busy = true;
  try {
    const p = await pickDirectory();
    if (!p) return;
    await addFolders([p]);
  } catch (e) {
    showToast(errMessage(e), 'error');
  } finally {
    busy = false;
  }
}

export async function handleNewWorkspaceFromFolder(): Promise<void> {
  if (busy) return;
  busy = true;
  try {
    const p = await pickDirectory();
    if (!p) return;
    await newWorkspaceFromFolder(p);
    showToast(`Workspace "${basename(p)}" dibuat`);
  } catch (e) {
    showToast(errMessage(e), 'error');
  } finally {
    busy = false;
  }
}

// Pane terminal di titik jatuh (piksel CSS dari event drop Wails), bila ada.
function terminalSessionAt(x: unknown, y: unknown): string | null {
  if (typeof x !== 'number' || typeof y !== 'number') return null;
  const el = document.elementFromPoint(x, y);
  const paneId = el?.closest('[data-pane-id]')?.getAttribute('data-pane-id');
  if (!paneId) return null;
  const leaf = findLeaf(get(layout), paneId);
  return leaf && leaf.kind === 'terminal' && leaf.sessionId ? leaf.sessionId : null;
}

export async function handleDropped(p: { paths: string[]; all?: string[]; x?: number; y?: number }): Promise<void> {
  // Jatuh di atas pane terminal: ketik path-nya ke terminal (gambar/file untuk Claude Code).
  const all = p?.all ?? [];
  if (all.length > 0) {
    const sid = terminalSessionAt(p.x, p.y);
    if (sid && pasteToSession(sid, formatDroppedPaths(all))) return;
  }
  if (busy) return;
  const paths = p?.paths ?? [];
  if (paths.length === 0) {
    showToast('Hanya folder yang bisa ditambahkan', 'error');
    return;
  }
  busy = true;
  try {
    await addFolders(paths);
  } catch (e) {
    showToast(errMessage(e), 'error');
  } finally {
    busy = false;
  }
}
