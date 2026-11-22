// Registri tab file: jembatan antara FileTab (pemilik state dirty) dan
// penutup tab (PaneHeader/PaneMenu) yang berada di luar komponen.
import { writable, get, type Readable } from 'svelte/store';

export interface FileTabHandle {
  isDirty(): boolean;
  // true bila berhasil atau memang tidak ada yang perlu disimpan; false bila gagal
  save(): Promise<boolean>;
}

const handles = new Map<string, FileTabHandle>();
const dirtySet = writable<ReadonlySet<string>>(new Set());

// Himpunan id tab yang sedang kotor (read-only bagi pemanggil).
export const dirtyTabs: Readable<ReadonlySet<string>> = { subscribe: dirtySet.subscribe };

// Mendaftarkan handle tab; mengembalikan fungsi cabut yang aman terhadap remount cepat.
export function registerFileTab(tabId: string, handle: FileTabHandle): () => void {
  handles.set(tabId, handle);
  return () => {
    if (handles.get(tabId) === handle) unregisterFileTab(tabId);
  };
}

export function unregisterFileTab(tabId: string): void {
  handles.delete(tabId);
  setFileTabDirty(tabId, false);
}

export function getFileTab(tabId: string): FileTabHandle | undefined {
  return handles.get(tabId);
}

// Set baru tiap perubahan (imutabel) agar reaktif; no-op bila status sama.
export function setFileTabDirty(tabId: string, dirty: boolean): void {
  if (get(dirtySet).has(tabId) === dirty) return;
  dirtySet.update((old) => {
    const next = new Set(old);
    if (dirty) next.add(tabId);
    else next.delete(tabId);
    return next;
  });
}

export function isTabDirty(tabId: string): boolean {
  return get(dirtySet).has(tabId);
}
