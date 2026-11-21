// State antrean bersama (tanpa timer; pembaruan lewat event mention.queue).
import { writable, derived } from 'svelte/store';
import { inWails } from '../../lib/ipc';
import { cancelMessage, confirmMessage, errorText, listQueue, onQueueEvent } from './mention-client';
import type { Message, QueueEvent } from './types';

const bySession = writable<Record<string, Message[]>>({});
export const queueError = writable('');

export const pendingMessages = derived(bySession, (m) =>
  Object.values(m)
    .flat()
    .sort((a, b) => String(a.createdAt).localeCompare(String(b.createdAt))),
);
export const pendingCount = derived(pendingMessages, (l) => l.length);

const seen = new Set<string>();
let refs = 0;
let unsub: (() => void) | null = null;

export function applyQueueEvent(e: QueueEvent): void {
  bySession.update((m) => {
    const n = { ...m };
    if (e.queue.length === 0) delete n[e.sessionId];
    else n[e.sessionId] = e.queue;
    return n;
  });
}

export async function loadInitial(): Promise<void> {
  if (!inWails) {
    queueError.set('Hanya tersedia di aplikasi desktop');
    return;
  }
  try {
    const list = await listQueue();
    queueError.set('');
    const grouped: Record<string, Message[]> = {};
    for (const msg of list) (grouped[msg.to] ??= []).push(msg);
    bySession.update((cur) => {
      const n = { ...cur };
      for (const [sid, q] of Object.entries(grouped)) {
        if (!seen.has(sid)) n[sid] = q;
      }
      return n;
    });
  } catch (e) {
    queueError.set(errorText(e));
  }
}

export function startQueueSync(): () => void {
  refs++;
  if (refs === 1) {
    unsub = onQueueEvent((e) => {
      seen.add(e.sessionId);
      applyQueueEvent(e);
    });
    void loadInitial();
  }
  let done = false;
  return () => {
    if (done) return;
    done = true;
    refs--;
    if (refs === 0 && unsub) {
      unsub();
      unsub = null;
      seen.clear();
    }
  };
}

export async function doCancel(id: string): Promise<string> {
  try {
    await cancelMessage(id);
    return '';
  } catch (e) {
    return errorText(e);
  }
}

export async function doConfirm(id: string): Promise<string> {
  try {
    await confirmMessage(id);
    return '';
  } catch (e) {
    return errorText(e);
  }
}
