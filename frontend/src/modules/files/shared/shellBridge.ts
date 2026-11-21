// Satu-satunya tempat mengimpor store ui-shell untuk modul files.
import { get, derived, type Readable } from 'svelte/store';
import { activeWorkspace as shellActiveWorkspace, openTab, tabs, activateTab } from '../../../lib/stores';
import type { FileTabData } from './types';

export interface ShellProject { id: string; path: string; name: string }
export interface ShellWorkspace { id: string; projects: ShellProject[] }

export const activeWorkspace: Readable<ShellWorkspace | null> = derived(shellActiveWorkspace, ($w) =>
  $w ? { id: $w.id, projects: $w.projects ?? [] } : null,
);

function baseName(path: string): string {
  const i = path.lastIndexOf('/');
  return i < 0 ? path : path.slice(i + 1);
}

// Skema data tab (satu-satunya): TabContent hanya meneruskan `refId` (string) ke komponen,
// jadi FileTabData disandikan sebagai JSON pada `refId` dengan urutan kunci tetap
// (id tab = `file:${refId}` sehingga path yang sama menghasilkan tab yang sama).
function encode(data: FileTabData): string {
  return JSON.stringify({ path: data.path, projectId: data.projectId, workspaceId: data.workspaceId });
}

export function openFileTab(data: FileTabData): void {
  const refId = encode(data);
  const id = `file:${refId}`;
  if (get(tabs).some((t) => t.id === id)) {
    activateTab(id);
    return;
  }
  openTab({ type: 'file', refId, title: baseName(data.path) });
}

export function decodeFileTab(props: Record<string, unknown>): FileTabData {
  const raw = props.refId;
  if (typeof raw !== 'string') throw new Error('refId tab file tidak valid');
  const o = JSON.parse(raw) as Partial<FileTabData>;
  if (typeof o.path !== 'string') throw new Error('data tab file tidak valid');
  return { path: o.path, projectId: String(o.projectId ?? ''), workspaceId: String(o.workspaceId ?? '') };
}

// ui-shell belum punya API tab kotor; kirim CustomEvent.
export function setTabDirty(path: string, dirty: boolean): void {
  window.dispatchEvent(new CustomEvent('shikigami:tab-dirty', { detail: { path, dirty } }));
}
