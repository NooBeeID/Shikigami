import type { Component } from 'svelte';
import { derived, writable, type Readable } from 'svelte/store';

export type PanelArea = 'sidebar' | 'main-tab' | 'statusbar';
export interface PanelSpec { id: string; area: PanelArea; component: Component<any>; order?: number; }
export interface TabTypeSpec { type: 'session' | 'file' | 'thread'; component: Component<any>; }

const panelStore = writable<Record<string, PanelSpec>>({});
const tabTypeStore = writable<Record<string, TabTypeSpec>>({});

export function registerPanel(spec: PanelSpec): void {
  panelStore.update((m) => ({ ...m, [spec.id]: spec }));
}

export function registerTabType(spec: TabTypeSpec): void {
  tabTypeStore.update((m) => ({ ...m, [spec.type]: spec }));
}

export function panels(area: PanelArea): Readable<PanelSpec[]> {
  return derived(panelStore, (m) =>
    Object.values(m)
      .filter((p) => p.area === area)
      .sort((a, b) => (a.order ?? 0) - (b.order ?? 0) || a.id.localeCompare(b.id)),
  );
}

export function tabTypes(): Readable<Record<string, TabTypeSpec>> {
  return { subscribe: tabTypeStore.subscribe };
}
