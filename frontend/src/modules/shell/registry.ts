import type { Component } from 'svelte';
import { derived, writable, type Readable } from 'svelte/store';

export interface RightTabSpec { id: string; title: string; component: Component<any>; order?: number }
export interface TopbarItemSpec { id: string; component: Component<any>; align: 'left' | 'center' | 'right'; order?: number }

const rightTabStore = writable<Record<string, RightTabSpec>>({});
const topbarStore = writable<Record<string, TopbarItemSpec>>({});

const byOrder = (a: { id: string; order?: number }, b: { id: string; order?: number }) =>
  (a.order ?? 0) - (b.order ?? 0) || a.id.localeCompare(b.id);

export function registerRightTab(spec: RightTabSpec): void {
  rightTabStore.update((m) => ({ ...m, [spec.id]: spec }));
}

export function registerTopbarItem(spec: TopbarItemSpec): void {
  topbarStore.update((m) => ({ ...m, [spec.id]: spec }));
}

export function rightTabs(): Readable<RightTabSpec[]> {
  return derived(rightTabStore, (m) => Object.values(m).sort(byOrder));
}

export function topbarItems(align: 'left' | 'center' | 'right'): Readable<TopbarItemSpec[]> {
  return derived(topbarStore, (m) =>
    Object.values(m)
      .filter((i) => i.align === align)
      .sort(byOrder),
  );
}
