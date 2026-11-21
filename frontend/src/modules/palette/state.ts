import { writable, get, type Readable } from 'svelte/store';

const open = writable(false);

export const paletteOpen: Readable<boolean> = { subscribe: open.subscribe };
export function openPalette(): void { open.set(true); }
export function closePalette(): void { open.set(false); }
export function togglePalette(): void { open.set(!get(open)); }
