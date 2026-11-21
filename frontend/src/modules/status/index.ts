import { registerPanel } from '../../lib/registry';
import SidebarBadges from './SidebarBadges.svelte';
import StatusBar from './StatusBar.svelte';
import { startStatusSync } from './store';

registerPanel({ id: 'status-badges', area: 'sidebar', order: 90, component: SidebarBadges });
registerPanel({ id: 'status-summary', area: 'statusbar', order: 50, component: StatusBar });
startStatusSync();

// Diekspor agar shell dapat mengimpor StatusBadge kelak.
export { default as StatusBadge } from './StatusBadge.svelte';
