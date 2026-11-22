import { writable, type Writable } from 'svelte/store';

export interface SidebarState { visible: boolean; width: number }
export interface RightSidebarState extends SidebarState { activeTab: string }

export const LEFT_MIN = 240, LEFT_MAX = 480, RIGHT_MIN = 220, RIGHT_MAX = 520;
export const LEFT_DEFAULT = 300, RIGHT_DEFAULT = 280;

export const leftSidebar: Writable<SidebarState> = writable<SidebarState>({ visible: true, width: LEFT_DEFAULT });
export const rightSidebar: Writable<RightSidebarState> = writable<RightSidebarState>({
  visible: true,
  width: RIGHT_DEFAULT,
  activeTab: 'files',
});

function clamp(n: number, min: number, max: number, def: number): number {
  if (!Number.isFinite(n)) return def;
  return Math.round(Math.min(max, Math.max(min, n)));
}

export function toggleLeft(): void {
  leftSidebar.update((s) => ({ ...s, visible: !s.visible }));
}
export function toggleRight(): void {
  rightSidebar.update((s) => ({ ...s, visible: !s.visible }));
}
export function setLeftWidth(px: number): void {
  const width = clamp(px, LEFT_MIN, LEFT_MAX, LEFT_DEFAULT);
  leftSidebar.update((s) => ({ ...s, width }));
}
export function setRightWidth(px: number): void {
  const width = clamp(px, RIGHT_MIN, RIGHT_MAX, RIGHT_DEFAULT);
  rightSidebar.update((s) => ({ ...s, width }));
}
export function setRightTab(id: string): void {
  rightSidebar.update((s) => ({ ...s, activeTab: id }));
}
