import { writable } from 'svelte/store';

// Popup pilihan jenis sesi baru (⌘T): shell / claude / codex.
export const newSessionPicker = writable(false);

export function openNewSessionPicker(): void {
  newSessionPicker.set(true);
}

export function closeNewSessionPicker(): void {
  newSessionPicker.set(false);
}
