import type { Component, Snippet } from 'svelte';
import { derived, writable, type Readable } from 'svelte/store';
import type { Session } from '../../lib/stores/types';

export interface SessionListWrapperProps {
  onerror: (msg: string) => void;
  list: Snippet<[filter: (s: Session) => boolean]>;
}
export interface SessionListWrapperSpec { id: string; component: Component<SessionListWrapperProps> }

export interface SessionMenuItemProps { sessionId: string; onDone?: () => void }
export interface SessionMenuItemSpec { id: string; component: Component<SessionMenuItemProps>; order?: number }

const wrapperStore = writable<SessionListWrapperSpec | null>(null);
const menuItemStore = writable<Record<string, SessionMenuItemSpec>>({});

const byOrder = (a: { id: string; order?: number }, b: { id: string; order?: number }) =>
  (a.order ?? 0) - (b.order ?? 0) || a.id.localeCompare(b.id);

export function registerSessionListWrapper(spec: SessionListWrapperSpec): void {
  wrapperStore.set(spec);
}

export const sessionListWrapper: Readable<SessionListWrapperSpec | null> = { subscribe: wrapperStore.subscribe };

export function registerSessionMenuItem(spec: SessionMenuItemSpec): void {
  menuItemStore.update((m) => ({ ...m, [spec.id]: spec }));
}

export function sessionMenuItems(): Readable<SessionMenuItemSpec[]> {
  return derived(menuItemStore, (m) => Object.values(m).sort(byOrder));
}

export interface SidebarSectionSpec { id: string; component: Component<any>; order?: number }
const sectionStore = writable<Record<string, SidebarSectionSpec>>({});
export function registerSidebarSection(spec: SidebarSectionSpec): void { sectionStore.update((m) => ({ ...m, [spec.id]: spec })); }
export function sidebarSections(): Readable<SidebarSectionSpec[]> { return derived(sectionStore, (m) => Object.values(m).sort(byOrder)); }
