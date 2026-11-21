import { writable, get, type Writable } from 'svelte/store';
import type { Tab, TabType } from './types';

export const tabs: Writable<Tab[]> = writable<Tab[]>([]);
export const activeTabId: Writable<string | null> = writable<string | null>(null);

// FINAL(foundation)
export function openTab(t: { type: TabType; refId: string; title?: string }): Tab {
  const id = `${t.type}:${t.refId}`;
  const existing = get(tabs).find((x) => x.id === id);
  if (existing) {
    activeTabId.set(id);
    return existing;
  }
  const tab: Tab = { id, type: t.type, refId: t.refId, title: t.title ?? t.refId };
  tabs.update((l) => [...l, tab]);
  activeTabId.set(id);
  return tab;
}

// FINAL(foundation)
export function activateTab(id: string): void {
  if (get(tabs).some((x) => x.id === id)) activeTabId.set(id);
}

// FINAL(foundation)
export function closeTab(id: string): void {
  const list = get(tabs);
  const idx = list.findIndex((x) => x.id === id);
  if (idx < 0) return;
  const next = list.filter((x) => x.id !== id);
  tabs.set(next);
  if (get(activeTabId) === id) {
    const neighbor = next[idx] ?? next[idx - 1] ?? null;
    activeTabId.set(neighbor ? neighbor.id : null);
  }
}
