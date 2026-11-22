export interface Project { id: string; path: string; name: string }
export interface WorkspaceConfig { grimoireUrl?: string; grimoireKey?: string }
export interface WorkspaceGroup { id: string; name: string; order: number; main?: boolean; layout?: unknown | null }
export interface Workspace {
  id: string; name: string; projects: Project[]; config: WorkspaceConfig; lastOpenedAt?: number;
  groups?: WorkspaceGroup[]; activeGroupId?: string; membership?: Record<string, string>;
}

export type SessionStatus = 'running' | 'exited';
export interface Session {
  id: string; workspaceId: string; projectId: string; cwd: string;
  providerId: string; status: SessionStatus; createdAt: string; exitCode?: number;
}

export interface Provider {
  id: string; name: string;
  capabilities?: { hooks?: boolean; mcp?: boolean; idleEvent?: boolean };
  available?: boolean; path?: string; error?: string;
}

export type TabType = 'session' | 'file';
export interface Tab { id: string; type: TabType; title: string; refId: string }
