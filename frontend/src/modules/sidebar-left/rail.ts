import { registerRailItem } from '../shell/registry';
import AgentSidebar from './AgentSidebar.svelte';
import AgentRailIcon from './AgentRailIcon.svelte';

registerRailItem({ id: 'agents', title: 'Agen', icon: AgentRailIcon, component: AgentSidebar, order: 0 });
