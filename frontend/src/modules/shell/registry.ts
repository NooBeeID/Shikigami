import type { Component } from 'svelte';
import { derived, writable, type Readable } from 'svelte/store';

export interface RightTabSpec { id: string; title: string; component: Component<any>; order?: number }
export interface TopbarItemSpec { id: string; component: Component<any>; align: 'left' | 'center' | 'right'; order?: number }

export interface RailItemSpec {
  id: string;
  title: string;
  icon: Component;
  component: Component<any>;
  order?: number;
  visible?: Readable<boolean>;
  badge?: Readable<number | boolean>; // titik tampil bila > 0 atau true
}

export interface LayoutHeaderSpec { id: string; component: Component<any>; order?: number }

const rightTabStore = writable<Record<string, RightTabSpec>>({});
const railStore = writable<Record<string, RailItemSpec>>({});
const topbarStore = writable<Record<string, TopbarItemSpec>>({});
const layoutHeaderStore = writable<Record<string, LayoutHeaderSpec>>({});

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

export function registerLayoutHeader(spec: LayoutHeaderSpec): void {
  layoutHeaderStore.update((m) => ({ ...m, [spec.id]: spec }));
}

export function layoutHeaders(): Readable<LayoutHeaderSpec[]> {
  return derived(layoutHeaderStore, (m) => Object.values(m).sort(byOrder));
}

export function registerRailItem(spec: RailItemSpec): void {
  railStore.update((m) => ({ ...m, [spec.id]: spec }));
}

export function railItems(): Readable<RailItemSpec[]> {
  const sorted = derived(railStore, (m) => Object.values(m).sort(byOrder));
  return derived(
    sorted,
    ($items, set) => {
      if ($items.length === 0) {
        set([]);
        return;
      }
      const flags: boolean[] = $items.map(() => true);
      let ready = false;
      const emit = () => {
        if (ready) set($items.filter((_, i) => flags[i]));
      };
      const unsubs = $items.map((item, i) =>
        item.visible
          ? item.visible.subscribe((v) => {
              flags[i] = v;
              emit();
            })
          : () => {},
      );
      ready = true;
      emit();
      return () => unsubs.forEach((u) => u());
    },
    [] as RailItemSpec[],
  );
}
