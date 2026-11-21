export type MenuEventName =
  | 'menu:zoom'
  | 'menu:open-folder'
  | 'menu:new-workspace-from-folder'
  | 'menu:toggle-sidebar-left'
  | 'menu:toggle-sidebar-right'
  | 'files:dropped';

export interface MenuPayloads {
  'menu:zoom': { action: 'in' | 'out' | 'reset' };
  'files:dropped': { paths: string[] };
}

export interface AppConfig {
  zoom: number;
  window?: { x: number; y: number; w: number; h: number };
}
