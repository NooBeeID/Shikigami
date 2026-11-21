import { registerPanel } from '../../lib/registry';
import WorkspaceSidebar from './WorkspaceSidebar.svelte';

registerPanel({ id: 'workspace-sidebar', area: 'sidebar', component: WorkspaceSidebar, order: 10 });
