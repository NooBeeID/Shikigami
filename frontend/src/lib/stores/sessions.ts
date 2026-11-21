import { writable, get, type Writable } from 'svelte/store';
import { call } from '../ipc';
import type { Session } from './types';
import { activeWorkspaceId, activeProject } from './workspaces';
import { normalizeList } from './util';

export const sessions: Writable<Session[]> = writable<Session[]>([]);

// FINAL(shell-layout): hanya bergantung pada workspace aktif, bukan project.
export async function loadSessions(): Promise<void> {
  const workspaceId = get(activeWorkspaceId);
  if (!workspaceId) {
    sessions.set([]);
    return;
  }
  const res = await call('session.list', { workspaceId });
  const list = normalizeList<Session>(res, 'sessions').filter((s) => s.workspaceId === workspaceId);
  // abaikan balasan basi bila workspace aktif sudah berganti
  if (get(activeWorkspaceId) !== workspaceId) return;
  sessions.set(list);
}

// FINAL(shell-layout)
export async function createSession(providerId: string): Promise<Session> {
  const workspaceId = get(activeWorkspaceId);
  const project = get(activeProject);
  if (!workspaceId || !project) throw new Error('Tambah project dulu');
  const s = await call<Session>('session.create', {
    workspaceId,
    projectId: project.id,
    cwd: project.path,
    providerId,
  });
  sessions.update((l) => (l.some((x) => x.id === s.id) ? l : [...l, s]));
  return s;
}

// FINAL(shell-layout)
export async function killSession(id: string): Promise<void> {
  await call('session.kill', { id });
  sessions.update((l) => l.map((s) => (s.id === id ? { ...s, status: 'exited' } : s)));
}
