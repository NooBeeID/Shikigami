// Cermin tipe Go runs.Run, runs.Progress, runs.AgentLine (internal/runs/runs.go).
export interface Usage { input: number; cacheRead: number; output: number; }

export interface AgentLine { label: string; phase: string; done: boolean; model?: string; usage?: Usage; }

export interface Progress {
  phaseTitle?: string;
  done?: number;
  total?: number;
  agents?: AgentLine[];
  model?: string;
  usage?: Usage;
}

export interface Run {
  id: string;
  sessionId: string;
  kind: 'workflow' | 'subagent';
  name: string;
  identity: string;
  taskId?: string;
  runId?: string;
  agentId?: string;
  startedAt: number;
  confirmed: boolean;
  progress?: Progress;
}
