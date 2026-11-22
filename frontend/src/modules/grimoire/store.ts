import { writable, get } from 'svelte/store';
import { onEvent } from '../../lib/ipc';
import { activeProjectId } from '../../lib/stores';
import { grimoireBuild, grimoireStatus, grimoireList, type GrimoireBook } from './client';

export interface GrimoireState {
  projectId: string | null;
  exists: boolean;
  building: boolean;
  books: GrimoireBook[];
  error: string | null;
}

const empty = (projectId: string | null): GrimoireState => ({
  projectId, exists: false, building: false, books: [], error: null,
});

const state = writable<GrimoireState>(empty(null));
// Hanya subscribe yang diekspos; perubahan lewat aksi di berkas ini.
export const grimoire = { subscribe: state.subscribe };

let unsubEvent: (() => void) | null = null;
let unsubProject: (() => void) | null = null;
let loadSeq = 0;

function msg(e: unknown): string {
  const m = e instanceof Error ? e.message : String(e);
  return m.length > 200 ? m.slice(0, 200) + '...' : m;
}

// Mengembalikan true bila refresh berhasil diterapkan atau dibuang karena usang.
async function refresh(projectId: string): Promise<boolean> {
  const seq = ++loadSeq;
  try {
    const st = await grimoireStatus(projectId);
    if (seq !== loadSeq || get(activeProjectId) !== projectId) return true;
    const books = st.exists ? await grimoireList(projectId) : [];
    if (seq !== loadSeq || get(activeProjectId) !== projectId) return true;
    state.update((s) => ({ ...s, projectId, exists: st.exists, building: st.building, books }));
    return true;
  } catch (e) {
    if (seq !== loadSeq || get(activeProjectId) !== projectId) return true;
    state.update((s) => ({ ...s, error: msg(e) }));
    return false;
  }
}

export function startGrimoireStore(): void {
  if (unsubEvent) return;
  let first = true;
  let last: string | null = null;
  unsubProject = activeProjectId.subscribe((id) => {
    if (!first && id === last) return;
    first = false;
    last = id;
    loadSeq++; // batalkan refresh yang sedang berjalan untuk project lama
    state.set(empty(id));
    if (id) void refresh(id);
  });
  unsubEvent = onEvent('grimoire.changed', (_t, payload) => {
    if (typeof payload !== 'object' || !payload || !('projectId' in payload)) return;
    const pid = (payload as { projectId: unknown }).projectId;
    if (typeof pid !== 'string' || pid !== get(activeProjectId)) return;
    void refresh(pid);
  });
}

export function stopGrimoireStore(): void {
  unsubProject?.();
  unsubEvent?.();
  unsubProject = null;
  unsubEvent = null;
}

export async function generate(): Promise<void> {
  const projectId = get(activeProjectId);
  if (!projectId || get(state).building) return;
  state.update((s) => ({ ...s, building: true, error: null }));
  try {
    await grimoireBuild(projectId);
  } catch (e) {
    if (get(activeProjectId) === projectId) state.update((s) => ({ ...s, error: msg(e) }));
  } finally {
    const ok = await refresh(projectId);
    // status server adalah sumber kebenaran; jangan biarkan building tertinggal true
    if (!ok && get(activeProjectId) === projectId) state.update((s) => ({ ...s, building: false }));
  }
}
