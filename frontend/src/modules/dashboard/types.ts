export type AgentStatus = 'waiting' | 'running' | 'idle' | 'done';

export interface AgentRow {
  sessionId: string;
  title: string;
  providerId: string;
  projectName: string;
  status: AgentStatus;
  since: number; // ms epoch
  reason?: string;
  lastActivity?: string;
}

export const STATUS_ORDER: AgentStatus[] = ['waiting', 'running', 'idle', 'done'];

export const STATUS_LABEL: Record<AgentStatus, string> = {
  waiting: 'Menunggu',
  running: 'Berjalan',
  idle: 'Idle',
  done: 'Selesai',
};
