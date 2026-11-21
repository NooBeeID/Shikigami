import { get } from 'svelte/store';
import { call } from '../../lib/ipc';
import { layout, type LayoutStore } from '../../lib/stores/layout';
import { sessions, createSession } from '../../lib/stores/sessions';
import { activeWorkspace } from '../../lib/stores/workspaces';
import type { Session } from '../../lib/stores/types';
import { findLeaf, type NewLeaf } from './model';

// Mencegah Cmd+D beruntun membuat banyak sesi.
let inflight = false;

export async function splitPaneWithNewSession(
  paneId: string,
  dir: 'h' | 'v',
  store: LayoutStore = layout,
): Promise<void> {
  if (inflight) return;
  const src = findLeaf(get(store), paneId);
  if (!src || src.kind !== 'terminal') return;
  const providerId =
    src.provider ?? get(sessions).find((s) => s.id === src.sessionId)?.providerId;
  if (!providerId) {
    console.error('[layout] provider pane tidak diketahui');
    return;
  }
  inflight = true;
  try {
    let s: Session;
    const ws = get(activeWorkspace);
    const project = ws?.projects?.find((p) => p.id === src.projectId);
    if (ws && project) {
      s = await call<Session>('session.create', {
        workspaceId: ws.id,
        projectId: project.id,
        cwd: project.path,
        providerId,
      });
      sessions.update((l) => (l.some((x) => x.id === s.id) ? l : [...l, s]));
    } else {
      s = await createSession(providerId);
    }
    const nl: NewLeaf = {
      kind: 'terminal',
      sessionId: s.id,
      projectId: s.projectId,
      provider: s.providerId,
    };
    if (get(store).maximizedId) store.restore();
    if (findLeaf(get(store), paneId)) store.splitPane(paneId, dir, nl, 'after');
    else store.openPane(nl, { dir });
  } catch (err) {
    console.error('[layout] gagal membuat sesi split:', err);
  } finally {
    inflight = false;
  }
}

export async function splitFocused(dir: 'h' | 'v', store: LayoutStore = layout): Promise<void> {
  const id = get(store).focusId;
  if (!id) return;
  await splitPaneWithNewSession(id, dir, store);
}

export function installLayoutShortcuts(store: LayoutStore = layout): () => void {
  function onKeyDown(e: KeyboardEvent) {
    if (!(e.metaKey && !e.ctrlKey && !e.altKey && e.code === 'KeyD')) return;
    const t = e.target as HTMLElement | null;
    if (
      t &&
      !t.closest?.('.xterm-helper-textarea') &&
      (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)
    )
      return;
    e.preventDefault();
    e.stopPropagation();
    if (e.repeat) return;
    void splitFocused(e.shiftKey ? 'v' : 'h', store);
  }
  window.addEventListener('keydown', onKeyDown, true);
  return () => window.removeEventListener('keydown', onKeyDown, true);
}
