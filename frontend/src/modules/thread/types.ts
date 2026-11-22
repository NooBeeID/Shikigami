// Bentuk dicerminkan dari internal/thread/types.go; tiket lain hanya menambah field opsional.
export type ThreadState = 'active' | 'closed';
export type ThreadRole = 'master' | 'peer';
export type ThreadKind = 'message' | 'system' | 'activity';
export interface Participant { sessionId: string; projectId: string; role: ThreadRole; name: string }
export interface Thread {
  id: string; workspaceId: string; title: string; goal: string; state: ThreadState;
  createdAt: string; closedAt?: string; participants: Participant[]; readyToClose: boolean;
  closeSummary?: string;
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
export const FROM_USER = 'user';
export const FROM_SYSTEM = 'system';
export const TO_USER = 'user';
export const TO_ALL = 'all';
