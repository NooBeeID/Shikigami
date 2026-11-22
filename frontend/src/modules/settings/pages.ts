import type { Component } from 'svelte';
import { derived, writable, type Readable } from 'svelte/store';

export interface SettingsPageSpec {
  id: string;
  title: string;
  order?: number;
  component: Component<any>;
}

const pageStore = writable<Record<string, SettingsPageSpec>>({});

export function registerSettingsPage(spec: SettingsPageSpec): void {
  pageStore.update((m) => ({ ...m, [spec.id]: spec }));
}

export const settingsPages: Readable<SettingsPageSpec[]> = derived(pageStore, (m) =>
  Object.values(m).sort((a, b) => (a.order ?? 0) - (b.order ?? 0) || a.id.localeCompare(b.id)),
);
