import { derived, get, writable, type Readable } from 'svelte/store';
import { onEvent } from '../../lib/ipc';
import { activeWorkspaceId } from '../../lib/stores/workspaces';
import { listRepos, parseGitError, type Provider, type Repo } from './client';

export type GitState = { status: 'loading' | 'ready' | 'error'; code?: string; message?: string };

export const gitRepos = writable<Repo[]>([]);
export const gitState = writable<GitState>({ status: 'loading' });

export const repoKey = (r: Repo): string => r.host + '/' + r.path;

const gitlabRepos = derived(gitRepos, ($r) => $r.filter((r) => r.provider === 'gitlab'));
const githubRepos = derived(gitRepos, ($r) => $r.filter((r) => r.provider === 'github'));
export function reposFor(provider: Provider): Readable<Repo[]> {
  return provider === 'gitlab' ? gitlabRepos : githubRepos;
}

// Peta di memori (bertahan selama app hidup). true = terlipat.
export const collapsed = writable<Record<string, boolean>>({});
export const selectedState = writable<Record<string, 'open' | 'merged' | 'closed'>>({});

export function isSectionOpen(map: Record<string, boolean>, key: string, index: number): boolean {
  if (key in map) return !map[key];
  return index === 0;
}

export function toggleSection(key: string, currentlyOpen: boolean): void {
  collapsed.update((m) => ({ ...m, [key]: currentlyOpen }));
}

let seq = 0;

export async function refreshRepos(): Promise<void> {
  const wsId = get(activeWorkspaceId);
  if (wsId === null) {
    seq++;
    gitRepos.set([]);
    gitState.set({ status: 'ready' });
    return;
  }
  const my = ++seq;
  try {
    const rs = await listRepos(wsId);
    if (my !== seq || get(activeWorkspaceId) !== wsId) return;
    gitRepos.set(rs);
    gitState.set({ status: 'ready' });
  } catch (x) {
    if (my !== seq || get(activeWorkspaceId) !== wsId) return;
    const e = parseGitError(x);
    gitRepos.set([]);
    gitState.set({ status: 'error', code: e.code, message: e.message });
  }
}

let lastWs: string | null | undefined = undefined;
activeWorkspaceId.subscribe((id) => {
  if (id === lastWs) return;
  lastWs = id;
  gitRepos.set([]);
  gitState.set({ status: 'loading' });
  void refreshRepos();
});

const RELEVANT = new Set(['projectAdded', 'projectRemoved', 'deleted', 'configChanged']);
onEvent('workspace.changed', (_t, payload) => {
  if (!payload || typeof payload !== 'object') return;
  const p = payload as { kind?: unknown; workspaceId?: unknown };
  if (typeof p.kind !== 'string' || !RELEVANT.has(p.kind)) return;
  if (typeof p.workspaceId === 'string' && p.workspaceId !== '' && p.workspaceId !== get(activeWorkspaceId)) return;
  void refreshRepos();
});
