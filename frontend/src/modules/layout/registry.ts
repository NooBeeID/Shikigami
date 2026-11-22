import type { Component } from 'svelte';
import { writable, type Readable } from 'svelte/store';
import type { Tab } from './model';

export interface PaneKindSpec {
  kind: string;
  component: Component<{ pane: Tab; paneId: string; visible: boolean }>;
  titleFor?: (tab: Tab) => string;
  onEndSession?: (tab: Tab) => Promise<void>;
  badgeFor?: (tab: Tab) => string | null;
}

const kindStore = writable<Record<string, PaneKindSpec>>({});

export function registerPaneKind(spec: PaneKindSpec): void {
  kindStore.update((m) => ({ ...m, [spec.kind]: spec }));
}

export function paneKinds(): Readable<Record<string, PaneKindSpec>> {
  return { subscribe: kindStore.subscribe };
}

export function getPaneKind(kind: string): PaneKindSpec | undefined {
  let found: PaneKindSpec | undefined;
  const unsub = kindStore.subscribe((m) => {
    found = m[kind];
  });
  unsub();
  return found;
}
