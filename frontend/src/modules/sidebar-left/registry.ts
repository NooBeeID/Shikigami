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

// Keputusan (tiket main-rail-ticket-02-threads-rail): Threads kini item rail
// (registerRailItem, id 'threads'), bukan section di panel Agen. Mekanisme
// registerSidebarSection/sidebarSections sengaja DIBIARKAN sebagai titik ekstensi
// untuk section opsional di dalam panel Agen; saat ini TIDAK ADA pendaftar.
// Pemakai di frontend/src (hasil grep): AgentSidebar.svelte (merender generik) dan
// ekspor ulang di index.ts; satu-satunya pemanggil register lama
// (modules/thread/index.ts, 'thread-sidebar') dihapus oleh wiring threads-rail-item.
// Jangan ubah perilaku mekanisme ini tanpa memeriksa ulang pemakai (grep).
export interface SidebarSectionSpec { id: string; component: Component<any>; order?: number }
const sectionStore = writable<Record<string, SidebarSectionSpec>>({});
export function registerSidebarSection(spec: SidebarSectionSpec): void { sectionStore.update((m) => ({ ...m, [spec.id]: spec })); }
export function sidebarSections(): Readable<SidebarSectionSpec[]> { return derived(sectionStore, (m) => Object.values(m).sort(byOrder)); }

export interface SessionFilterSpec { id: string; predicate: Readable<(s: Session) => boolean> } // true = tampil
const filterStore = writable<Record<string, SessionFilterSpec>>({});
export function registerSessionFilter(spec: SessionFilterSpec): void { filterStore.update((m) => ({ ...m, [spec.id]: spec })); }
export const sessionFilter: Readable<(s: Session) => boolean> = derived(filterStore, ($m, set: (v: (s: Session) => boolean) => void) => {
  const stores = Object.values($m).map((f) => f.predicate);
  if (stores.length === 0) { set(() => true); return; }
  return derived(stores, (fns) => (s: Session) => fns.every((f) => f(s))).subscribe(set);
});
