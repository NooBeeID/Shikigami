import { startRunsSync } from './store';

startRunsSync();

export { runs, runsCount, startRunsSync, refreshDetail } from './store';
export type { Run, Progress, AgentLine } from './types';
