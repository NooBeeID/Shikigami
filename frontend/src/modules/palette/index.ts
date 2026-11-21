import { registerBuiltinActions } from './actions';
import { registerBuiltinSources } from './sources';

registerBuiltinActions();
registerBuiltinSources();

export { default as Palette } from './Palette.svelte';
export { paletteOpen, openPalette, closePalette } from './state';
export { registerPaletteAction, registerPaletteSource } from './registry';
export type { PaletteAction, PaletteItem, PaletteSource, PaletteSourceKind } from './registry';
