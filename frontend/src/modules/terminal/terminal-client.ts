import { call, onEvent } from '../../lib/ipc';

export interface SessionInfo { id: string; workspaceId: string; projectId: string; cwd: string; providerId: string; status: 'running' | 'exited'; createdAt: string; exitCode?: number }
export interface OutputEvent { data: string; offset: number }
export interface AttachResult { session: SessionInfo; scrollback: Uint8Array; offset: number }

export function bytesToBase64(b: Uint8Array): string {
  const chunk = 0x8000;
  let s = '';
  for (let i = 0; i < b.length; i += chunk) {
    s += String.fromCharCode.apply(null, Array.from(b.subarray(i, i + chunk)));
  }
  return btoa(s);
}

export function base64ToBytes(s: string): Uint8Array {
  if (!s) return new Uint8Array(0);
  const bin = atob(s);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

export function stringToBase64(s: string): string {
  return bytesToBase64(new TextEncoder().encode(s));
}

export async function listSessions(): Promise<SessionInfo[]> {
  const r = await call<{ sessions: SessionInfo[] | null }>('session.list', {});
  return r?.sessions ?? [];
}

export function createSession(): Promise<SessionInfo> {
  return call<SessionInfo>('session.create', {});
}

export async function killSession(id: string): Promise<void> {
  await call('session.kill', { id });
}

export async function writeSession(id: string, data: string): Promise<void> {
  await call('session.write', { id, data: stringToBase64(data) });
}

export async function resizeSession(id: string, cols: number, rows: number): Promise<void> {
  await call('session.resize', { id, cols, rows });
}

export function subscribeOutput(id: string, fn: (ev: OutputEvent) => void): () => void {
  return onEvent('session.output.' + id, (_t, p) => {
    const ev = p as OutputEvent;
    if (ev && typeof ev.data === 'string' && typeof ev.offset === 'number') fn(ev);
  });
}

export function subscribeExit(id: string, fn: (exitCode: number) => void): () => void {
  return onEvent('session.exit', (_t, p) => {
    const e = p as { id?: string; exitCode?: number };
    if (e && e.id === id) fn(e.exitCode ?? 0);
  });
}

export async function attachSession(id: string): Promise<AttachResult> {
  const r = await call<{ session: SessionInfo; scrollback: string; offset: number }>('session.attach', { id });
  return { session: r.session, scrollback: base64ToBytes(r.scrollback ?? ''), offset: r.offset ?? 0 };
}
