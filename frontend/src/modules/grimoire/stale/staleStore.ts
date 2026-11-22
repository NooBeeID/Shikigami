import { writable, type Readable } from 'svelte/store';
import { call, onEvent } from '../../../lib/ipc';

export interface StaleState {
  building: boolean;
  staleCount: number;
  staleChapters: Set<string>; // path bab stale
  staleBooks: Set<string>; // nama buku yang memuat >=1 bab stale
  error: string | null;
}

// Tipe longgar: field yang belum dikirim backend diperlakukan false/0.
type RawChapter = { path?: unknown; stale?: unknown };
type RawBook = { name?: unknown; chapters?: unknown };
type RawStatus = { exists?: unknown; staleCount?: unknown; building?: unknown };

const initial = (): StaleState => ({
  building: false,
  staleCount: 0,
  staleChapters: new Set(),
  staleBooks: new Set(),
  error: null,
});

const state = writable<StaleState>(initial());
export const staleState: Readable<StaleState> = { subscribe: state.subscribe };

let currentId: string | null = null;
let seq = 0;

function staleMsg(e: unknown): string {
  const m = e instanceof Error ? e.message : String(e);
  return m.length > 200 ? m.slice(0, 200) + '...' : m;
}

async function load(id: string): Promise<void> {
  const my = ++seq;
  try {
    const st = await call<RawStatus | null>('grimoire.status', { projectId: id });
    if (my !== seq || currentId !== id) return;
    const building = st?.building === true;
    const staleCount = typeof st?.staleCount === 'number' ? st.staleCount : 0;
    if (st?.exists !== true) {
      state.set({ ...initial(), building });
      return;
    }
    const books = await call<RawBook[] | null>('grimoire.list', { projectId: id });
    if (my !== seq || currentId !== id) return;
    const staleChapters = new Set<string>();
    const staleBooks = new Set<string>();
    for (const b of books ?? []) {
      const chapters = Array.isArray(b.chapters) ? (b.chapters as RawChapter[]) : [];
      let any = false;
      for (const c of chapters) {
        if (c && c.stale === true && typeof c.path === 'string') {
          staleChapters.add(c.path);
          any = true;
        }
      }
      if (any && typeof b.name === 'string') staleBooks.add(b.name);
    }
    state.set({ building, staleCount, staleChapters, staleBooks, error: null });
  } catch (e) {
    if (my !== seq || currentId !== id) return;
    state.update((s) => ({ ...s, error: staleMsg(e) }));
  }
}

export function startStaleStore(projectId: string): () => void {
  currentId = projectId;
  seq++;
  state.set(initial());
  void load(projectId);
  const off = onEvent('grimoire.changed', (_t, payload) => {
    if (typeof payload !== 'object' || !payload || !('projectId' in payload)) return;
    const pid = (payload as { projectId?: unknown }).projectId;
    if (typeof pid !== 'string' || pid !== currentId) return;
    void load(pid);
  });
  return () => {
    off();
    if (currentId === projectId) {
      currentId = null;
      seq++;
      state.set(initial());
    }
  };
}

export async function runUpdate(projectId: string): Promise<void> {
  if (currentId === projectId) state.update((s) => ({ ...s, error: null, building: true }));
  try {
    // Build inkremental: tanpa field full. Respons {building:true} bukan galat.
    await call('grimoire.build', { projectId });
  } catch (e) {
    if (currentId === projectId) state.update((s) => ({ ...s, error: staleMsg(e) }));
  } finally {
    // Status server menjadi sumber kebenaran, menimpa building optimistis.
    if (currentId === projectId) void load(projectId);
  }
}

export const isChapterStale = (s: StaleState, path: string): boolean => s.staleChapters.has(path);
export const isBookStale = (s: StaleState, book: string): boolean => s.staleBooks.has(book);
