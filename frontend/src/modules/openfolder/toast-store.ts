import { writable } from 'svelte/store';

export interface ToastMessage { id: number; kind: 'info' | 'error'; text: string }

export const toast = writable<ToastMessage | null>(null);

let timer: ReturnType<typeof setTimeout> | null = null;
let counter = 0;

export function clearToast(): void {
  if (timer) {
    clearTimeout(timer);
    timer = null;
  }
  toast.set(null);
}

export function showToast(text: string, kind: 'info' | 'error' = 'info', ms = 4000): void {
  if (timer) clearTimeout(timer);
  counter += 1;
  toast.set({ id: counter, kind, text });
  timer = setTimeout(clearToast, ms);
}
