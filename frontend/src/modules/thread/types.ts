// Bentuk dicerminkan dari internal/thread/types.go; tiket lain hanya menambah field opsional.
export type ThreadState = 'active' | 'closed';
export type ThreadRole = 'master' | 'peer';
export type ThreadKind = 'message' | 'system' | 'activity';
// dead/previousSessionIds ditambah epic thread-ticket-05
export interface Participant {
  sessionId: string; projectId: string; role: ThreadRole; name: string; dead?: boolean; previousSessionIds?: string[];
  // agentId.. ditambah epic global-agents-ticket-03 (semua opsional; thread lama tanpa field ini).
  agentId?: string; agentName?: string; category?: string; providerId?: string; model?: string;
}
export interface Thread {
  id: string; workspaceId: string; title: string; goal: string; state: ThreadState;
  createdAt: string; closedAt?: string; participants: Participant[]; readyToClose: boolean;
  closeSummary?: string;
  // Ditambah epic global-agents-ticket-03.
  noOrchestrator?: boolean;
}
export interface ThreadMessage {
  id: string; threadId: string; from: string; to: string; text: string; ts: string; kind: ThreadKind;
  // Ditambah epic thread-ticket-03.
  count?: number;   // hanya kind 'activity'
  items?: string[]; // hanya kind 'activity'
}
export const THREAD_METHODS = {
  create: 'thread.create', list: 'thread.list', get: 'thread.get',
  messages: 'thread.messages', send: 'thread.send', ownedSessions: 'thread.ownedSessions',
  closePreview: 'thread.closePreview', close: 'thread.close',
} as const;
export const THREAD_EVENTS = { updated: 'thread.updated', message: 'thread.message' } as const;
export const AGENTS_METHODS = { list: 'agents.list' } as const;
export const AGENTS_EVENTS = { changed: 'agents.changed' } as const;
export interface AgentCategoryInfo { id: string; name: string }
export const FROM_USER = 'user';
export const FROM_SYSTEM = 'system';
export const TO_USER = 'user';
export const TO_ALL = 'all';

export const RECOVERY_DEAD_NOTICE = 'daemon dimulai ulang: agent mati, akan dihidupkan saat pesan berikutnya';
export const RECOVERY_RESPAWN_PREFIX = 'agent dihidupkan ulang: ';
export const RECOVERY_FAILED_PREFIX = 'gagal menghidupkan ulang agent: ';
