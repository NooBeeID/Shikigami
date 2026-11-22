import { registerPanel } from '../../lib/registry';
import GrimoirePanel from './GrimoirePanel.svelte';

export function registerGrimoirePanel(): void {
  registerPanel({ id: 'grimoire', area: 'sidebar', component: GrimoirePanel, order: 30 });
}
registerGrimoirePanel();
