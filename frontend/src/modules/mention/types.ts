// Tipe lokal modul mention (bentuk mengikuti kontrak IPC).
export type MessageState = 'queued' | 'awaiting_confirm' | 'delivered' | 'failed';

export interface Message {
  id: string;
  from: string;
  fromName: string;
  to: string;
  text: string;
  createdAt: string;
  state: MessageState;
  source: 'agent' | 'user';
  error?: string;
}

export interface NameInfo {
  sessionId: string;
  name: string;
  projectName: string;
  providerId: string;
  status: 'running' | 'waiting' | 'done' | 'idle';
}

export interface QueueEvent {
  sessionId: string;
  queue: Message[];
}

// Agent global (dari IPC agents.list).
export interface AgentInfo {
  id: string;
  name: string;
  categoryId: string;
  providerId: string;
  model: string;
  isDefault: boolean;
}

// Kategori agent beserta brief peran.
export interface CategoryInfo {
  id: string;
  name: string;
  role: string;
  builtin: boolean;
}

// Satu baris saran autocomplete @: sesi, agent, atau kategori ber-default.
export type MentionSuggestion =
  | ({ kind: 'session' } & NameInfo)
  | { kind: 'agent'; name: string; categoryName: string; providerId: string; model: string; isDefault: boolean; hasSession: boolean }
  | { kind: 'category'; name: string; defaultAgent: string; hasSession: boolean };

// Hasil mention.send; spawned/spawnedSession opsional (sesi baru dibuka dari agent).
export interface MentionSendResult extends Message {
  spawned?: boolean;
  spawnedSession?: string;
}
