import { registerRailItem } from '../shell/registry';
import GrimoirePanel from './GrimoirePanel.svelte';
import GrimoireRailIcon from './GrimoireRailIcon.svelte';

// Slot order rail: agents=0, lark=10, git-github=20, git-gitlab=21, threads=30, grimoire=40.
// Panel lama (registerPanel area 'sidebar') tidak dirender sejak shell v2, jadi dipindah ke rail.
export function registerGrimoirePanel(): void {
  registerRailItem({ id: 'grimoire', title: 'Grimoire', icon: GrimoireRailIcon, component: GrimoirePanel, order: 40 });
}
registerGrimoirePanel();
