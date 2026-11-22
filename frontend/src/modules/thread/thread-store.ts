// Store thread: daftar thread workspace aktif, pesan per thread, dan langganan event real-time.
import { derived, get, writable, type Readable, type Writable } from 'svelte/store';
import { inWails } from '../../lib/ipc';
import { activeWorkspaceId } from '../../lib/stores';
import { layout } from '../../lib/stores/layout';
import {
  errorText, createThreadRpc, listMessages, listThreads, getThread, sendThread,
  onThreadMessage, onThreadUpdated,
} from './thread-client';
import { subscribeAgentEvents } from './thread-agents';
import type { Thread, ThreadMessage } from './types';

export const threads = writable<Thread[]>([]);
export const messagesByThread = writable<Record<string, ThreadMessage[]>>({});
export const threadError = writable('');

// State panel Threads yang harus bertahan saat panel di-unmount (item rail lain aktif).
export interface ThreadPanelState { scrollTop: number }
export const threadPanelState: Writable<ThreadPanelState> = writable<ThreadPanelState>({ scrollTop: 0 });

export function sortThreads(list: Thread[]): Thread[] {
  return [...list].sort((a, b) => (a.createdAt < b.createdAt ? 1 : a.createdAt > b.createdAt ? -1 : 0));
}

export function mergeMessages(list: ThreadMessage[], incoming: ThreadMessage[]): ThreadMessage[] {
  const map = new Map<string, ThreadMessage>();
  for (const m of list) map.set(m.id, m);
  for (const m of incoming) map.set(m.id, m);
  return [...map.values()].sort((a, b) => {
    const ta = new Date(a.ts).getTime();
    const tb = new Date(b.ts).getTime();
    if (ta !== tb) return ta - tb;
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
  });
}

function upsertThread(t: Thread): void {
  threads.update((list) => {
    const i = list.findIndex((x) => x.id === t.id);
    if (i < 0) return sortThreads([...list, t]);
    const next = list.slice();
    next[i] = t;
    return sortThreads(next);
  });
}

// Dipakai ThreadTab untuk thread yang dimuat lewat getThread.
export function upsertThreadIfActiveWorkspace(t: Thread): void {
  if (t.workspaceId === get(activeWorkspaceId)) upsertThread(t);
}

function putMessages(threadId: string, incoming: ThreadMessage[]): void {
  messagesByThread.update((m) => ({ ...m, [threadId]: mergeMessages(m[threadId] ?? [], incoming) }));
}

let loadGen = 0;

export async function loadThreads(workspaceId: string | null): Promise<void> {
  const gen = ++loadGen;
  if (!workspaceId) {
    threads.set([]);
    return;
  }
  if (!inWails) {
    threadError.set('Hanya tersedia di aplikasi desktop');
    return;
  }
  try {
    const list = await listThreads(workspaceId);
    if (gen !== loadGen) return;
    threads.set(sortThreads(list));
    threadError.set('');
  } catch (e) {
    if (gen !== loadGen) return;
    threadError.set(errorText(e));
  }
}

export async function loadMessages(threadId: string): Promise<void> {
  try {
    putMessages(threadId, await listMessages(threadId));
  } catch (e) {
    threadError.set(errorText(e));
  }
}

export function openThreadPane(t: Thread): void {
  layout.openPane({ kind: 'thread', threadId: t.id, workspaceId: t.workspaceId, title: t.title });
}

export async function createThread(goal: string): Promise<Thread> {
  const g = goal.trim();
  if (!g) throw new Error('Goal tidak boleh kosong');
  const ws = get(activeWorkspaceId);
  if (!ws) throw new Error('Pilih workspace terlebih dahulu');
  const t = await createThreadRpc(ws, g);
  upsertThread(t);
  openThreadPane(t);
  return t;
}

export async function sendMessage(threadId: string, text: string): Promise<void> {
  const v = text.trim();
  if (!v) return;
  const m = await sendThread(threadId, v);
  if (m && typeof m.id === 'string') putMessages(threadId, [{ ...m, threadId: m.threadId || threadId }]);
}

export function threadById(id: string): Readable<Thread | undefined> {
  return derived(threads, (list) => list.find((t) => t.id === id));
}

export function participantName(thread: Thread | undefined, id: string): string {
  if (id === 'user') return 'user';
  if (id === 'system') return 'sistem';
  if (id === 'all') return 'semua';
  const p = thread?.participants.find((x) => x.sessionId === id || x.previousSessionIds?.includes(id));
  if (p) {
    if (p.name) return p.name;
    if (p.role === 'master') return 'master';
  }
  return id.slice(0, 8);
}

let cleanup: (() => void) | null = null;

// Idempoten: panggilan kedua mengembalikan pembersih yang sama.
export function subscribeThreadEvents(): () => void {
  if (cleanup) return cleanup;
  const offUpdated = onThreadUpdated((t) => {
    if (t.workspaceId === get(activeWorkspaceId)) upsertThread(t);
  });
  const offMessage = onThreadMessage((m) => putMessages(m.threadId, [m]));
  const offAgents = subscribeAgentEvents();
  let first = true;
  const offWs = activeWorkspaceId.subscribe((id) => {
    // Langganan sudah aktif sebelum pemuatan awal; subscribe() langsung memanggil sekali.
    void first;
    first = false;
    void loadThreads(id);
  });
  const stop = () => {
    offUpdated();
    offMessage();
    offAgents();
    offWs();
    cleanup = null;
  };
  cleanup = stop;
  return stop;
}

export { getThread };
