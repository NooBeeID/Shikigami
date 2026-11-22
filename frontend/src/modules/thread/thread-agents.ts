// Cache nama kategori agent untuk UI thread. Sengaja tidak mengimpor modules/settings/agents.
import { writable, type Readable } from 'svelte/store';
import { inWails } from '../../lib/ipc';
import { listAgentCategories, onAgentsChanged } from './thread-client';
import type { Participant, Thread } from './types';

const names = writable<Record<string, string>>({});
export const agentCategories: Readable<Record<string, string>> = { subscribe: names.subscribe };

let loadGen = 0;

export async function loadAgentCategories(): Promise<void> {
  if (!inWails) return;
  const gen = ++loadGen;
  try {
    const list = await listAgentCategories();
    if (gen !== loadGen) return;
    const map: Record<string, string> = {};
    for (const c of list) map[c.id] = c.name;
    names.set(map);
  } catch {
    // Gagal memuat: pertahankan cache terakhir (atau kosong); UI jatuh ke id mentah.
  }
}

export function categoryLabel(map: Record<string, string>, categoryId: string): string {
  if (!categoryId) return '';
  const name = map[categoryId];
  if (!name) return categoryId;
  return name.toLowerCase() === categoryId.toLowerCase() ? name : `${categoryId} (${name})`;
}

export function masterOf(thread: Thread): Participant | undefined {
  return thread.participants.find((p) => p.role === 'master');
}

export function participantSub(p: Participant, projectName: string, categoryNames: Record<string, string>): string {
  if (!p.agentName) return [p.role, projectName].filter(Boolean).join(' · ');
  return [p.role, projectName, p.agentName, categoryLabel(categoryNames, p.category ?? ''), p.model].filter(Boolean).join(' · ');
}

let cleanup: (() => void) | null = null;

export function subscribeAgentEvents(): () => void {
  if (cleanup) return cleanup;
  void loadAgentCategories();
  const off = onAgentsChanged(() => void loadAgentCategories());
  cleanup = () => { off(); cleanup = null; };
  return cleanup;
}
