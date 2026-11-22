// Penyaringan sesi milik thread dari sidebar kiri: set id dari thread.ownedSessions + thread.updated,
// ditambah pola cwd /threads/<ULID>/ agar sesi tidak sempat tampil sebelum event tiba.
import { derived, get, writable, type Readable } from 'svelte/store';
import type { Session } from '../../../lib/stores/types';
import { ownedSessions, onThreadUpdated } from '../thread-client';
import { threads } from '../thread-store';
import type { Thread } from '../types';

const THREAD_CWD = /[\\/]threads[\\/][0-9A-HJKMNP-TV-Z]{26}(?:[\\/]|$)/;

const idsStore = writable<Set<string>>(new Set());
export const threadSessionIds: Readable<Set<string>> = { subscribe: idsStore.subscribe };

function addIds(ids: string[]): void {
  const cur = get(idsStore);
  const fresh = ids.filter((id) => id && !cur.has(id));
  if (fresh.length === 0) return;
  idsStore.update((s) => new Set([...s, ...fresh]));
}

function participantIds(t: Thread): string[] {
  return (t.participants ?? []).map((p) => p.sessionId).filter(Boolean);
}

export function isThreadSession(s: { id: string; cwd?: string }): boolean {
  return get(idsStore).has(s.id) || (!!s.cwd && THREAD_CWD.test(s.cwd));
}

// true = bukan sesi thread (tampil di sidebar).
export const threadSessionPredicate: Readable<(s: Session) => boolean> = derived(
  idsStore,
  ($ids) => (s: Session) => !($ids.has(s.id) || (!!s.cwd && THREAD_CWD.test(s.cwd))),
);

let unsub: (() => void) | null = null;

// Idempoten: panggilan berikutnya mengembalikan pembersih yang sama.
export function startSessionSync(): () => void {
  const stop = () => {
    if (unsub) { unsub(); unsub = null; }
  };
  if (unsub) return stop;
  unsub = onThreadUpdated((t) => addIds(participantIds(t)));
  ownedSessions().then(addIds).catch(() => {});
  addIds(get(threads).flatMap(participantIds));
  return stop;
}
