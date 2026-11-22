// Cermin tipe Go runs.Run, runs.Progress, runs.AgentLine (internal/runs/runs.go).
export interface AgentLine { label: string; phase: string; done: boolean; }

export interface Progress {
  phaseTitle?: string;
  done?: number;
  total?: number;
  agents?: AgentLine[];
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
