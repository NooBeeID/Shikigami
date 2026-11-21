import { get } from 'svelte/store';
import { layout } from '../../lib/stores/layout';
import { findLeafBy, type Leaf } from '../layout/model';
import {
  sessions,
  providers,
  activeWorkspaceId,
  selectProject,
  createSession,
  type Session,
} from '../../lib/stores';
import { emitMenuEvent } from '../appmenu';

export function findSessionPane(sessionId: string): Leaf | null {
  return findLeafBy(get(layout), (l) => l.kind === 'terminal' && l.sessionId === sessionId);
}

export function findFilePane(projectId: string, filePath: string): Leaf | null {
  return findLeafBy(
    get(layout),
    (l) => l.kind === 'file' && !!l.filePath && l.filePath === filePath && l.projectId === projectId,
  );
}

export function basename(path: string): string {
  const p = path.replace(/[\\/]+$/, '');
  const i = Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'));
  return i >= 0 ? p.slice(i + 1) : p;
}

export function sessionTitle(sessionId: string): string {
  const pane = findSessionPane(sessionId);
  if (pane) return pane.title;
  const s = get(sessions).find((x) => x.id === sessionId);
  if (!s) return sessionId.slice(0, 6);
  return get(providers).find((p) => p.id === s.providerId)?.name ?? s.providerId;
}

export function focusOrOpenSession(sessionId: string): boolean {
  const s = get(sessions).find((x) => x.id === sessionId);
  if (!s) return false;
  const pane = findSessionPane(sessionId);
  if (pane) {
    layout.setFocus(pane.id);
    return true;
  }
  layout.openPane({
    kind: 'terminal',
    sessionId,
    projectId: s.projectId,
    provider: s.providerId,
    title: sessionTitle(sessionId),
  });
  return true;
}

export function openFilePane(a: { projectId: string; filePath: string; workspaceId?: string; title?: string }): string {
  const existing = findFilePane(a.projectId, a.filePath);
  if (existing) {
    layout.setFocus(existing.id);
    return existing.id;
  }
  return layout.openPane({
    kind: 'file',
    projectId: a.projectId,
    filePath: a.filePath,
    workspaceId: a.workspaceId ?? get(activeWorkspaceId) ?? undefined,
    title: a.title ?? basename(a.filePath),
  });
}

export async function createAndOpenSession(providerId: string, projectId?: string): Promise<Session> {
  if (projectId) selectProject(projectId);
  const s = await createSession(providerId);
  focusOrOpenSession(s.id);
  return s;
}

export function openFolder(): void {
  emitMenuEvent('menu:open-folder');
}

export function zoom(action: 'in' | 'out' | 'reset'): void {
  emitMenuEvent('menu:zoom', { action });
}

export function tidyGrid(): void {
  layout.tidyGrid();
}
