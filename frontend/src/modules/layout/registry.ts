import type { Component } from 'svelte';
import { writable, type Readable } from 'svelte/store';
import type { Leaf } from './model';

export interface PaneKindSpec {
  kind: string;
  component: Component<{ pane: Leaf; visible: boolean }>;
  titleFor?: (leaf: Leaf) => string;
  onEndSession?: (leaf: Leaf) => Promise<void>;
  badgeFor?: (leaf: Leaf) => string | null;
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
