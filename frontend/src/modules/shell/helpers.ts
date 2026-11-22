import { get } from 'svelte/store';
import { layout } from '../../lib/stores/layout';
import { activeTab, findLeaf, findTab } from '../layout/model';
import {
  sessions,
  providers,
  activeWorkspaceId,
  selectProject,
  createSession,
  type Session,
} from '../../lib/stores';
import { emitMenuEvent } from '../appmenu';

export interface TabRef {
  paneId: string;
  tabId: string;
}

export function findSessionPane(sessionId: string): TabRef | null {
  const r = findTab(get(layout), (t) => t.kind === 'terminal' && t.sessionId === sessionId);
  return r ? { paneId: r.pane.id, tabId: r.tab.id } : null;
}

export function findFilePane(projectId: string, filePath: string): TabRef | null {
  const r = findTab(
    get(layout),
    (t) => t.kind === 'file' && !!t.filePath && t.filePath === filePath && t.projectId === projectId,
  );
  return r ? { paneId: r.pane.id, tabId: r.tab.id } : null;
}

function refOfPane(paneId: string): TabRef | null {
  const p = findLeaf(get(layout), paneId);
  return p ? { paneId: p.id, tabId: activeTab(p).id } : null;
}

export function basename(path: string): string {
  const p = path.replace(/[\\/]+$/, '');
  const i = Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'));
  return i >= 0 ? p.slice(i + 1) : p;
}

export function sessionTitle(sessionId: string): string {
  const r = findTab(get(layout), (t) => t.kind === 'terminal' && t.sessionId === sessionId);
  if (r) return r.tab.title;
  const s = get(sessions).find((x) => x.id === sessionId);
  if (!s) return sessionId.slice(0, 6);
  return get(providers).find((p) => p.id === s.providerId)?.name ?? s.providerId;
}

export function focusOrOpenSession(sessionId: string): TabRef | null {
  const s = get(sessions).find((x) => x.id === sessionId);
  if (!s) return null;
  const ref = findSessionPane(sessionId);
  if (ref) {
    layout.activateTab(ref.paneId, ref.tabId);
    return ref;
  }
  const paneId = layout.openPane({
    kind: 'terminal',
    sessionId,
    projectId: s.projectId,
    provider: s.providerId,
    title: sessionTitle(sessionId),
  });
  return refOfPane(paneId);
}

export function openFilePane(a: { projectId: string; filePath: string; workspaceId?: string; title?: string }): TabRef {
  const ref = findFilePane(a.projectId, a.filePath);
  if (ref) {
    layout.activateTab(ref.paneId, ref.tabId);
    return ref;
  }
  const paneId = layout.openPane({
    kind: 'file',
    projectId: a.projectId,
    filePath: a.filePath,
    workspaceId: a.workspaceId ?? get(activeWorkspaceId) ?? undefined,
    title: a.title ?? basename(a.filePath),
  });
  return refOfPane(paneId) ?? { paneId, tabId: '' };
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
