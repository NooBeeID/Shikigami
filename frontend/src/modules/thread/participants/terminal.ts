import { get } from 'svelte/store';
import { layout } from '../../../lib/stores/layout';
import { sessions, loadSessions } from '../../../lib/stores/sessions';
import { providers } from '../../../lib/stores/providers';
import type { Participant, Thread } from '../types';

function focusExisting(sessionId: string): boolean {
  const l = get(layout.leaves).find((x) => x.kind === 'terminal' && x.sessionId === sessionId);
  if (!l) return false;
  layout.setFocus(l.id);
  return true;
}

export function openParticipantTerminal(p: Participant, thread: Thread): void {
  if (!p.sessionId || focusExisting(p.sessionId)) return;
  void (async () => {
    try {
      let s = get(sessions).find((x) => x.id === p.sessionId);
      if (!s) {
        await loadSessions();
        s = get(sessions).find((x) => x.id === p.sessionId);
      }
      if (!s) {
        console.warn('sesi peserta tidak ditemukan', p.sessionId);
        return;
      }
      if (focusExisting(p.sessionId)) return;
      const providerName = get(providers).find((x) => x.id === s!.providerId)?.name ?? s.providerId;
      layout.openThreadAgent({
        kind: 'terminal',
        sessionId: s.id,
        projectId: s.projectId,
        provider: s.providerId,
        workspaceId: thread.workspaceId,
        title: p.name || providerName,
      });
    } catch (e) {
      console.warn('gagal membuka terminal peserta:', e);
    }
  })();
}
