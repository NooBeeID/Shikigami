import Bell from './Bell.svelte';
import { registerTopbarItem } from '../shell';
export { Bell };
export * from './store';
registerTopbarItem({ id: 'notifications', component: Bell, align: 'right', order: 10 });
