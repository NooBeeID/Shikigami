// Pembungkus IPC mention dan helper murni.
import { call, onEvent } from '../../lib/ipc';
import type { AgentInfo, CategoryInfo, Message, MentionSendResult, MentionSuggestion, NameInfo, QueueEvent } from './types';

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
  // galat Indonesia dari backend (project konteks / agent / gagal spawn) tampil apa adanya
  if (/project konteks|Gagal membuka sesi|agent/i.test(msg)) return msg.slice(0, 160);
  if (/ERR_NOT_FOUND|not found|tidak ditemukan/i.test(msg)) return 'Sesi tidak ditemukan';
  if (/ERR_CONFLICT|penuh/i.test(msg)) return 'Antrean penuh (maks 50)';
  if (msg.includes('user_only')) return 'Mode user_only: pesan dari agent ditolak';
  if (/ERR_INVALID|tidak valid|workspace/i.test(msg)) return 'Pesan tidak valid (kosong atau beda workspace)';
  return msg.slice(0, 120);
}

export async function listNames(workspaceId: string): Promise<NameInfo[]> {
  return asList<NameInfo>(await call('mention.names', { workspaceId }), 'names');
}

export function sendMention(workspaceId: string, to: string, text: string, projectId?: string): Promise<MentionSendResult> {
  const params: Record<string, string> = { workspaceId, to, text };
  if (typeof projectId === 'string' && projectId) params.projectId = projectId;
  return call<MentionSendResult>('mention.send', params);
}

function slug(s: string): string {
  return s.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
}

// Daftar agent dan kategori global.
export async function listAgents(): Promise<{ categories: CategoryInfo[]; agents: AgentInfo[] }> {
  const res = await call<{ categories?: unknown; agents?: unknown }>('agents.list', {});
  return { categories: asList<CategoryInfo>(res, 'categories'), agents: asList<AgentInfo>(res, 'agents') };
}

// Susun saran: sesi, lalu agent, lalu kategori ber-default; tanpa duplikasi nama.
export function buildSuggestions(names: NameInfo[], agents: AgentInfo[], categories: CategoryInfo[]): MentionSuggestion[] {
  const out: MentionSuggestion[] = names.map((n) => ({ kind: 'session' as const, ...n }));
  const taken = new Set(names.map((n) => n.name));
  const hasSession = (base: string) => names.some((n) => n.name.startsWith(base + '-'));
  for (const a of agents) {
    const name = slug(a.name);
    if (!name || taken.has(name)) continue;
    out.push({
      kind: 'agent',
      name,
      categoryName: categories.find((c) => c.id === a.categoryId)?.name ?? a.categoryId,
      providerId: a.providerId,
      model: a.model,
      isDefault: a.isDefault,
      hasSession: hasSession(name),
    });
    taken.add(name);
  }
  for (const c of categories) {
    const def = agents.find((a) => a.categoryId === c.id && a.isDefault);
    if (!def) continue;
    const name = slug(c.name);
    if (!name || taken.has(name)) continue;
    const defaultAgent = slug(def.name);
    out.push({ kind: 'category', name, defaultAgent, hasSession: hasSession(name) || hasSession(defaultAgent) });
    taken.add(name);
  }
  return out;
}

// Apakah nama merujuk agent, atau kategori yang punya agent default.
export function matchAgentRef(name: string, agents: AgentInfo[], categories: CategoryInfo[]): boolean {
  const n = slug(name);
  if (agents.some((a) => slug(a.name) === n)) return true;
  return categories.some(
    (c) => (slug(c.name) === n || c.id.toLowerCase() === n) && agents.some((a) => a.categoryId === c.id && a.isDefault),
  );
}

export function onAgentsChanged(fn: () => void): () => void {
  return onEvent('agents.changed', () => fn());
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
