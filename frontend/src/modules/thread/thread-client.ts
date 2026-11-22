// Pembungkus IPC thread dan helper murni.
import { call, onEvent } from '../../lib/ipc';
import {
  AGENTS_EVENTS, AGENTS_METHODS, THREAD_EVENTS, THREAD_METHODS,
  type AgentCategoryInfo, type Participant, type Thread, type ThreadMessage,
} from './types';

export function asList<T>(res: unknown, key?: string): T[] {
  if (Array.isArray(res)) return res as T[];
  if (res && typeof res === 'object' && key) {
    const v = (res as Record<string, unknown>)[key];
    if (Array.isArray(v)) return v as T[];
  }
  return [];
}

export function errorText(e: unknown): string {
  let msg = '';
  try {
    msg = e instanceof Error ? e.message : typeof e === 'string' ? e : String((e as { message?: unknown })?.message ?? '');
  } catch {
    msg = '';
  }
  if (!msg) return 'Gagal menghubungi daemon';
  if (msg.includes('bukan di dalam Wails')) return 'Hanya tersedia di aplikasi desktop';
  if (/ERR_INVALID|tidak valid/i.test(msg)) {
    return msg.includes('ditutup') ? 'Thread sudah ditutup' : 'Masukan tidak valid (goal/pesan kosong atau workspace tidak dikenal)';
  }
  if (/ERR_NOT_FOUND|tidak ditemukan/i.test(msg)) return 'Thread tidak ditemukan';
  return msg.slice(0, 160);
}

function normParticipant(p: Participant): Participant {
  return {
    ...p,
    agentId: typeof p.agentId === 'string' ? p.agentId : undefined,
    agentName: typeof p.agentName === 'string' ? p.agentName : undefined,
    category: typeof p.category === 'string' ? p.category : undefined,
    providerId: typeof p.providerId === 'string' ? p.providerId : undefined,
    model: typeof p.model === 'string' ? p.model : undefined,
  };
}

function normThread(t: Thread): Thread {
  const parts = Array.isArray(t.participants) ? t.participants.filter((p) => p && typeof p === 'object').map(normParticipant) : [];
  return { ...t, participants: parts, noOrchestrator: t.noOrchestrator === true };
}

export async function listAgentCategories(): Promise<AgentCategoryInfo[]> {
  const res = await call<unknown>(AGENTS_METHODS.list, {});
  return asList<AgentCategoryInfo>(res, 'categories').filter((c) => c && typeof c.id === 'string' && typeof c.name === 'string');
}

export function onAgentsChanged(fn: () => void): () => void {
  try {
    return onEvent(AGENTS_EVENTS.changed, () => fn());
  } catch {
    return () => {};
  }
}

export async function listThreads(workspaceId: string): Promise<Thread[]> {
  return asList<Thread>(await call(THREAD_METHODS.list, { workspaceId })).map(normThread);
}

export async function getThread(id: string): Promise<Thread> {
  return normThread(await call<Thread>(THREAD_METHODS.get, { id }));
}

export async function createThreadRpc(workspaceId: string, goal: string, title?: string): Promise<Thread> {
  const params: Record<string, string> = { workspaceId, goal };
  if (title) params.title = title;
  return normThread(await call<Thread>(THREAD_METHODS.create, params));
}

export async function listMessages(id: string, sinceTs?: string): Promise<ThreadMessage[]> {
  const params: Record<string, string> = { id };
  if (sinceTs) params.sinceTs = sinceTs;
  return asList<ThreadMessage>(await call(THREAD_METHODS.messages, params));
}

export function sendThread(id: string, text: string): Promise<ThreadMessage> {
  return call<ThreadMessage>(THREAD_METHODS.send, { id, text });
}

export async function ownedSessions(): Promise<string[]> {
  return asList<string>(await call(THREAD_METHODS.ownedSessions, {}), 'sessionIds');
}

export function onThreadUpdated(fn: (t: Thread) => void): () => void {
  return onEvent(THREAD_EVENTS.updated, (_t, p) => {
    if (!p || typeof p !== 'object') return;
    if (typeof (p as { id?: unknown }).id !== 'string') return;
    fn(normThread(p as Thread));
  });
}

export function onThreadMessage(fn: (m: ThreadMessage) => void): () => void {
  return onEvent(THREAD_EVENTS.message, (_t, p) => {
    if (!p || typeof p !== 'object') return;
    const o = p as { id?: unknown; threadId?: unknown };
    if (typeof o.id !== 'string' || typeof o.threadId !== 'string') return;
    fn(p as ThreadMessage);
  });
}
