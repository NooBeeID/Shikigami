import { startRunsSync } from './store';
import RunsMenu from './RunsMenu.svelte';
import { registerTopbarItem } from '../shell';

startRunsSync();

registerTopbarItem({ id: 'runs', component: RunsMenu, align: 'right', order: 9 });

export { runs, runsCount, startRunsSync, refreshDetail } from './store';
export type { Run, Progress, AgentLine } from './types';
export { RunsMenu };
