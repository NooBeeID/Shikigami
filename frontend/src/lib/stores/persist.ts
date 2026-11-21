import { writable, type Readable } from 'svelte/store';

export type PersistStatus = 'idle' | 'restoring' | 'ready' | 'error';

const status = writable<PersistStatus>('idle');
const saved = writable<number | null>(null);

export const persistStatus: Readable<PersistStatus> = { subscribe: status.subscribe };
export const lastSavedAt: Readable<number | null> = { subscribe: saved.subscribe };

let handler: (() => Promise<void>) | null = null;

export function setPersistStatus(s: PersistStatus): void {
  status.set(s);
}
export function markSaved(ts: number = Date.now()): void {
  saved.set(ts);
}
export function setFlushHandler(fn: (() => Promise<void>) | null): void {
  handler = fn;
}
export async function flushNow(): Promise<void> {
  try {
    await handler?.();
  } catch (e) {
    console.warn('[persist] flush gagal', e);
  }
}
