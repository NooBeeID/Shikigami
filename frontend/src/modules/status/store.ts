import { derived, writable, type Readable } from 'svelte/store';
import { call, onEvent } from '../../lib/ipc';

export type Status = 'running' | 'waiting' | 'done' | 'idle';
export interface SessionStatus { status: Status; since: number; reason?: string; providerId?: string; }

const store = writable<Record<string, SessionStatus>>({});
export const statusStore: Readable<Record<string, SessionStatus>> = { subscribe: store.subscribe };

// Sesi yang tidak ada di store dianggap idle.
export function statusOf(id: string): Readable<Status> {
  return derived(store, (m) => m[id]?.status ?? 'idle');
}

const known: Status[] = ['running', 'waiting', 'done', 'idle'];
const isStatus = (v: unknown): v is Status => known.includes(v as Status);

function handler(_topic: string, payload: unknown): void {
  const p = payload as { sessionId?: unknown; status?: unknown; source?: unknown; reason?: unknown; at?: unknown } | null;
  if (!p || typeof p.sessionId !== 'string' || !isStatus(p.status)) return;
  // Event exit ditampilkan sebagai done sampai shell menghapus sesi.
  const status: Status = p.source === 'exit' ? 'done' : p.status;
  const since = typeof p.at === 'number' ? p.at : Date.now();
  const reason = p.source === 'exit' ? 'exit' : (typeof p.reason === 'string' && p.reason ? p.reason : undefined);
  const id = p.sessionId;
  store.update((m) => ({ ...m, [id]: { status, since, reason, providerId: m[id]?.providerId } }));
}

let unsub: (() => void) | null = null;

// Idempoten: panggilan berikutnya mengembalikan pembersih yang sama.
export function startStatusSync(): () => void {
  const stop = () => {
    if (unsub) { unsub(); unsub = null; }
  };
  if (unsub) return stop;
  // Berlangganan dahulu agar tidak ada event yang hilang, baru muat awal.
  unsub = onEvent('session.status', handler);
  call<Array<{ sessionId: string; status: Status; since: number; reason?: string; providerId?: string }>>('status.list')
    .then((list) => {
      if (!Array.isArray(list)) return;
      store.update((m) => {
        const next = { ...m };
        for (const it of list) {
          if (!it || typeof it.sessionId !== 'string' || !isStatus(it.status)) continue;
          const cur = next[it.sessionId];
          // Pertahankan entri dari event yang lebih baru.
          const providerId = typeof it.providerId === 'string' && it.providerId ? it.providerId : undefined;
          if (cur && cur.since >= it.since) {
            if (!cur.providerId && providerId) next[it.sessionId] = { ...cur, providerId };
            continue;
          }
          next[it.sessionId] = {
            status: it.status,
            since: it.since,
            reason: typeof it.reason === 'string' && it.reason ? it.reason : undefined,
            providerId,
          };
        }
        return next;
      });
    })
    .catch((e) => console.warn('status.list gagal:', e));
  return stop;
}
