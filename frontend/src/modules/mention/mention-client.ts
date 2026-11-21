// Pembungkus IPC mention dan helper murni.
import { call, onEvent } from '../../lib/ipc';
import type { Message, NameInfo, QueueEvent } from './types';

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
  if (/ERR_NOT_FOUND|not found|tidak ditemukan/i.test(msg)) return 'Sesi tidak ditemukan';
  if (/ERR_CONFLICT|penuh/i.test(msg)) return 'Antrean penuh (maks 50)';
  if (msg.includes('user_only')) return 'Mode user_only: pesan dari agent ditolak';
  if (/ERR_INVALID|tidak valid|workspace/i.test(msg)) return 'Pesan tidak valid (kosong atau beda workspace)';
  return msg.slice(0, 120);
}

export async function listNames(workspaceId: string): Promise<NameInfo[]> {
  return asList<NameInfo>(await call('mention.names', { workspaceId }), 'names');
}

export function sendMention(workspaceId: string, to: string, text: string): Promise<Message> {
  return call<Message>('mention.send', { workspaceId, to, text });
}

export async function listQueue(): Promise<Message[]> {
  return asList<Message>(await call('mention.queue.list', {}), 'messages');
}

export async function cancelMessage(id: string): Promise<void> {
  await call('mention.queue.cancel', { id });
}

export async function confirmMessage(id: string): Promise<void> {
  await call('mention.confirm', { id });
}

export function onQueueEvent(fn: (e: QueueEvent) => void): () => void {
  return onEvent('mention.queue', (_t, p) => {
    if (!p || typeof p !== 'object') return;
    const o = p as { sessionId?: unknown; queue?: unknown };
    if (typeof o.sessionId !== 'string') return;
    const queue = Array.isArray(o.queue) ? (o.queue as Message[]) : [];
    fn({ sessionId: o.sessionId, queue });
  });
}

export function onSessionExit(fn: () => void): () => void {
  return onEvent('session.exit', () => fn());
}

export function stripAnsi(s: string): string {
  return s
    .replace(/\x1b\[[0-?]*[ -\/]*[@-~]/g, '')
    .replace(/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, '')
    .replace(/\x1b[@-Z\\-_]/g, '')
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g, '');
}

export function decodeBase64(b64: string): string {
  try {
    const bin = atob(b64);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    return new TextDecoder('utf-8', { fatal: false }).decode(bytes);
  } catch {
    return '';
  }
}

export async function lastOutput(sessionId: string, lines = 30): Promise<string> {
  const res = await call<{ scrollback?: string }>('session.attach', { id: sessionId });
  const text = stripAnsi(decodeBase64(res?.scrollback ?? ''));
  const arr = text.split('\n');
  while (arr.length && arr[arr.length - 1].trim() === '') arr.pop();
  let out = arr.slice(-lines).join('\n').trimEnd();
  if (out.length > 4000) out = out.slice(-4000);
  return out;
}

export function parseMention(input: string): { name: string; text: string } | null {
  const m = /^@([a-z0-9-]+)(?:\s+([\s\S]*))?$/i.exec(input.trimStart());
  if (!m) return null;
  return { name: m[1].toLowerCase(), text: (m[2] ?? '').trim() };
}

export function mentionQuery(input: string, caret: number): { start: number; query: string } | null {
  const head = input.slice(0, caret);
  const m = /(^|\s)@([a-z0-9-]*)$/i.exec(head);
  if (!m) return null;
  return { start: head.length - m[2].length - 1, query: m[2].toLowerCase() };
}
