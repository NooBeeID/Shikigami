import { writable, derived, type Readable } from 'svelte/store';

export interface PaletteAction { id: string; title: string; hint?: string; keywords?: string[]; run: () => void | Promise<void> }
export type PaletteSourceKind = 'project' | 'session' | 'file' | 'action';
export interface PaletteItem { id: string; kind: PaletteSourceKind; title: string; subtitle?: string; run: () => void | Promise<void> }
export interface PaletteSource { id: string; kind: PaletteSourceKind; list: (query: string) => PaletteItem[] | Promise<PaletteItem[]> }

const actions = writable<Record<string, PaletteAction>>({});
const sources = writable<Record<string, PaletteSource>>({});

export function registerPaletteAction(a: PaletteAction): void {
  actions.update((m) => ({ ...m, [a.id]: a }));
}

export function registerPaletteSource(s: PaletteSource): void {
  sources.update((m) => ({ ...m, [s.id]: s }));
}

function sorted<T extends { id: string }>(m: Record<string, T>): T[] {
  return Object.values(m).sort((a, b) => a.id.localeCompare(b.id));
}

export const paletteActions: Readable<PaletteAction[]> = derived(actions, sorted);
export const paletteSources: Readable<PaletteSource[]> = derived(sources, sorted);
