import { registerRightTab } from '../shell';
import ActivityTab from './ActivityTab.svelte';

registerRightTab({ id: 'activity', title: 'Activity', order: 20, component: ActivityTab });

export { ActivityTab };
export type { ActivityEntry } from './types';
