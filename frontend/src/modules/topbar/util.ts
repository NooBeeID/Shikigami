import type { Session, Workspace } from '../../lib/stores';
import type { Status } from '../status/store';

export function providerBadge(providerId: string): string {
  if (providerId === 'claude') return 'CL';
  if (providerId === 'codex') return 'CX';
  if (providerId === 'shell') return 'SH';
  if (!providerId) return '??';
  return providerId.slice(0, 2).toUpperCase();
}

export function defaultWorkspaceName(list: Workspace[]): string {
  const used = new Set(list.map((w) => w.name));
  let n = list.length + 1;
  while (used.has(`Workspace ${n}`)) n++;
  return `Workspace ${n}`;
}

export function errText(e: unknown): string {
  const m = e instanceof Error ? e.message : typeof e === 'string' ? e : '';
  const t = m.trim();
  if (!t) return 'terjadi kesalahan';
  return t.length > 120 ? t.slice(0, 120) : t;
}

export function countByStatus(
  workspaceSessions: Session[],
  map: Record<string, { status: Status }>,
): Record<Status, number> {
  const out: Record<Status, number> = { running: 0, waiting: 0, done: 0, idle: 0 };
  for (const s of workspaceSessions) {
    if (s.status !== 'running') continue;
    out[map[s.id]?.status ?? 'idle']++;
  }
  return out;
}

export function sessionsOfActive(all: Session[], wsId: string | null): Session[] {
  return all.filter((s) => s.workspaceId === wsId);
}
