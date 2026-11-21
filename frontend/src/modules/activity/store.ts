import { writable, type Readable } from 'svelte/store';
import { call, onEvent } from '../../lib/ipc';
import type { ActivityEntry } from './types';

const MAX_ENTRIES = 300;
const INITIAL_LIMIT = 100;

const entries = writable<ActivityEntry[]>([]);
export const activityEntries: Readable<ActivityEntry[]> = { subscribe: entries.subscribe };

let started = false;
let current: ActivityEntry[] = [];
entries.subscribe((v) => { current = v; });
let unsub: (() => void) | null = null;

function keyOf(e: ActivityEntry): string {
  return `${e.ts}|${e.sessionId}|${e.tool}|${e.summary}`;
}

function isEntry(x: unknown): x is ActivityEntry {
  if (!x || typeof x !== 'object') return false;
  const o = x as Record<string, unknown>;
  return typeof o.ts === 'number' && typeof o.sessionId === 'string' &&
    typeof o.tool === 'string' && typeof o.summary === 'string';
}

function pushLive(e: ActivityEntry): void {
  const k = keyOf(e);
  if (current.some((x) => keyOf(x) === k)) return;
  entries.set([e, ...current].slice(0, MAX_ENTRIES));
}

function merge(list: unknown): void {
  if (!Array.isArray(list)) return;
  const seen = new Set(current.map(keyOf));
  const hist = list.filter(isEntry).filter((e) => !seen.has(keyOf(e)));
  const all = [...current, ...hist];
  // Array.prototype.sort is stable.
  all.sort((a, b) => b.ts - a.ts);
  entries.set(all.slice(0, MAX_ENTRIES));
}

export function ensureActivityStore(): void {
  if (started) return;
  started = true;
  unsub = onEvent('activity.event', (_t, payload) => {
    if (isEntry(payload)) pushLive(payload);
  });
  call<ActivityEntry[]>('activity.list', { limit: INITIAL_LIMIT }).then(merge).catch(() => {});
}
