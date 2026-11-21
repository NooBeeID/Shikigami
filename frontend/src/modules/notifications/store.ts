import { writable, derived, type Readable } from 'svelte/store';
import { statusStore, type Status } from '../status/store';
import { sessionTitle } from '../shell';

export type NotificationKind = 'waiting' | 'done';
export interface AppNotification {
  id: string;
  sessionId: string;
  kind: NotificationKind;
  title: string;
  at: number;
  read: boolean;
}
export const MAX_NOTIFICATIONS = 50;

const list = writable<AppNotification[]>([]);
export const notifications: Readable<AppNotification[]> = { subscribe: list.subscribe };
export const unreadCount = derived(list, (l) => l.filter((n) => !n.read).length);

export function markRead(id: string): void {
  list.update((l) => (l.some((n) => n.id === id && !n.read) ? l.map((n) => (n.id === id ? { ...n, read: true } : n)) : l));
}
export function markAllRead(): void {
  list.update((l) => (l.some((n) => !n.read) ? l.map((n) => (n.read ? n : { ...n, read: true })) : l));
}
export function clear(): void {
  list.set([]);
}

const prev = new Map<string, Status>();
let counter = 0;

function ingest(map: Record<string, { status: Status; since: number }>): void {
  const fresh: AppNotification[] = [];
  for (const [sessionId, entry] of Object.entries(map)) {
    const status = entry.status;
    if (!prev.has(sessionId)) {
      prev.set(sessionId, status);
      continue;
    }
    if (prev.get(sessionId) === status) continue;
    prev.set(sessionId, status);
    if (status !== 'waiting' && status !== 'done') continue;
    const at = Number.isFinite(entry.since) ? entry.since : Date.now();
    fresh.unshift({
      id: `${sessionId}:${status}:${at}:${counter++}`,
      sessionId,
      kind: status,
      title: sessionTitle(sessionId),
      at,
      read: false,
    });
  }
  for (const id of [...prev.keys()]) if (!(id in map)) prev.delete(id);
  if (fresh.length > 0) list.update((l) => [...fresh, ...l].slice(0, MAX_NOTIFICATIONS));
}

let started = false;
if (!started) {
  started = true;
  statusStore.subscribe(ingest);
}
