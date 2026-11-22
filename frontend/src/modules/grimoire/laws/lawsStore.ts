import { writable, readonly, type Writable, type Readable } from 'svelte/store';
import { call, onEvent } from '../../../lib/ipc';
import { grimoireStatus } from '../client';

export interface LawsInjectionInfo { exists: boolean; indexBytes: number; estTokens: number }

export const MAX_LAWS_BYTES = 16 * 1024;

export function utf8Bytes(s: string): number {
  return new TextEncoder().encode(s).length;
}

// true saat LawsEditor tampil inline menggantikan daftar buku.
export const lawsViewOpen: Writable<boolean> = writable(false);

const info = writable<LawsInjectionInfo | null>(null);
export const injectionInfo: Readable<LawsInjectionInfo | null> = readonly(info);

let generation = 0;

export async function getLaws(projectId: string): Promise<string> {
  const r = await call<{ content?: string }>('grimoire.laws.get', { projectId });
  return r?.content ?? '';
}

export async function setLaws(projectId: string, content: string): Promise<number> {
  const r = await call<{ lawsBytes?: number }>('grimoire.laws.set', { projectId, content });
  return r?.lawsBytes ?? utf8Bytes(content);
}

export function startLawsStore(projectId: string): () => void {
  const gen = ++generation;
  let stopped = false;
  info.set(null);

  async function refresh() {
    try {
      const s = await grimoireStatus(projectId);
      if (stopped || gen !== generation) return;
      info.set(s.exists ? { exists: true, indexBytes: s.indexBytes, estTokens: s.estTokens } : null);
    } catch {
      if (!stopped && gen === generation) info.set(null);
    }
  }

  const off = onEvent('grimoire.changed', (_t, payload) => {
    const id = (payload as { projectId?: string } | null)?.projectId;
    if (id === projectId) void refresh();
  });
  void refresh();

  return () => {
    if (stopped) return;
    stopped = true;
    off();
    if (gen === generation) info.set(null);
  };
}
