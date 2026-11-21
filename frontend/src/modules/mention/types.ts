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
