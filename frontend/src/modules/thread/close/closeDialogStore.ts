import { writable, type Readable } from 'svelte/store';

// Id thread yang dialog tutupnya sedang terbuka (null = tertutup).
const current = writable<string | null>(null);

export const closeDialogFor: Readable<string | null> = { subscribe: current.subscribe };

export function openCloseDialog(threadId: string): void {
  current.set(threadId);
}

export function closeCloseDialog(): void {
  current.set(null);
}
