import { get, writable } from 'svelte/store';
import { onEvent } from '../../lib/ipc';
import { activeWorkspaceId } from '../../lib/stores/workspaces';
import { larkSourcesGet, parseLarkError, type LarkConfig } from './client';

export type LarkState = { status: 'loading' | 'ready' | 'error'; code?: string; message?: string };

const empty = (): LarkConfig => ({ tickets: [], wiki: [] });

export const larkSources = writable<LarkConfig>(empty());
export const larkState = writable<LarkState>({ status: 'loading' });
export const sectionOpen = writable<{ tickets: boolean; wiki: boolean }>({ tickets: true, wiki: true });

let seq = 0;

export async function refreshSources(): Promise<void> {
  const wsId = get(activeWorkspaceId);
  if (wsId === null) {
    seq++;
    larkSources.set(empty());
    larkState.set({ status: 'ready' });
    return;
  }
  const my = ++seq;
  try {
    const cfg = await larkSourcesGet(wsId);
    if (my !== seq || get(activeWorkspaceId) !== wsId) return;
    larkSources.set(cfg);
    larkState.set({ status: 'ready' });
  } catch (x) {
    if (my !== seq || get(activeWorkspaceId) !== wsId) return;
    const e = parseLarkError(x);
    larkState.set({ status: 'error', code: e.code, message: e.message });
  }
}

let lastWs: string | null | undefined = undefined;
activeWorkspaceId.subscribe((id) => {
  if (id === lastWs) return;
  lastWs = id;
  larkSources.set(empty());
  larkState.set({ status: 'loading' });
  void refreshSources();
});

onEvent('workspace.changed', (_t, payload) => {
  if (!payload || typeof payload !== 'object') return;
  const p = payload as { kind?: unknown; workspaceId?: unknown };
  if (p.kind !== 'configChanged') return;
  if (typeof p.workspaceId === 'string' && p.workspaceId !== '' && p.workspaceId !== get(activeWorkspaceId)) return;
  void refreshSources();
});
