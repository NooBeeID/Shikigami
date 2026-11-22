import { get, writable } from 'svelte/store';
import { onEvent } from '../../../lib/ipc';
import { listAgents, type Agent, type Category } from './client';

export const categories = writable<Category[]>([]);
export const agents = writable<Agent[]>([]);
export const agentsLoading = writable<boolean>(false);
export const agentsError = writable<string | null>(null);
export const selectedCategoryId = writable<string | null>(null);

let loadSeq = 0;

export async function loadAgents(): Promise<void> {
  const seq = ++loadSeq;
  agentsLoading.set(true);
  agentsError.set(null);
  try {
    const snap = await listAgents();
    if (seq !== loadSeq) return;
    categories.set(snap.categories);
    agents.set(snap.agents);
    const sel = get(selectedCategoryId);
    if (sel === null || !snap.categories.some((c) => c.id === sel)) {
      selectedCategoryId.set(snap.categories[0]?.id ?? null);
    }
  } catch (e) {
    if (seq === loadSeq) agentsError.set(e instanceof Error ? e.message : String(e));
  } finally {
    if (seq === loadSeq) agentsLoading.set(false);
  }
}

export function selectCategory(id: string): void {
  selectedCategoryId.set(id);
}

// Berlangganan event agents.changed; kembalikan fungsi unsubscribe.
export function startAgentsSync(): () => void {
  return onEvent('agents.changed', () => {
    void loadAgents();
  });
}
