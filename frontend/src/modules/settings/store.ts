import { get, writable, type Writable } from 'svelte/store';
import { settingsPages } from './pages';

export const settingsOpen: Writable<boolean> = writable(false);
export const activePage: Writable<string> = writable('connectors');

export function openSettings(pageId?: string): void {
  const pages = get(settingsPages);
  if (pageId !== undefined && pages.some((p) => p.id === pageId)) activePage.set(pageId);
  else if (pages.length > 0) activePage.set(pages[0].id);
  // daftar kosong: biarkan activePage apa adanya
  settingsOpen.set(true);
}

export function closeSettings(): void {
  settingsOpen.set(false);
}
