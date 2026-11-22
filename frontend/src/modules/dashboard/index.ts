import { registerRailItem } from '../shell/registry';
import DashboardPanel from './DashboardPanel.svelte';
import DashboardRailIcon from './DashboardRailIcon.svelte';
import { waitingCount } from './store';

// Slot order rail: agents=0, lark=10, git-github=20, git-gitlab=21, threads=30, grimoire=40, dashboard=50.
// Badge titik pada ikon rail menyala saat ada sesi agent berstatus waiting.
export function registerDashboardPanel(): void {
  registerRailItem({
    id: 'dashboard',
    title: 'Dashboard',
    icon: DashboardRailIcon,
    component: DashboardPanel,
    order: 50,
    badge: waitingCount,
  });
}
registerDashboardPanel();
