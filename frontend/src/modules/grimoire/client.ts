import { call } from '../../lib/ipc';

export interface GrimoireChapter { name: string; path: string; stale: boolean }
export interface GrimoireBook { name: string; tags: string[]; chapters: GrimoireChapter[] }
export interface GrimoireStatus {
  exists: boolean; books: number; pages: number; staleCount: number; building: boolean;
  indexBytes: number; estTokens: number; lawsBytes: number; llmEnabled: boolean; lastBuildCost: number; dir: string;
}
export interface GrimoireSummary { books: number; pages: number; indexBytes: number }

export function grimoireBuild(projectId: string): Promise<GrimoireSummary> {
  return call<GrimoireSummary>('grimoire.build', { projectId });
}

export function grimoireStatus(projectId: string): Promise<GrimoireStatus> {
  return call<GrimoireStatus>('grimoire.status', { projectId });
}

export async function grimoireList(projectId: string): Promise<GrimoireBook[]> {
  const r = await call<GrimoireBook[] | null>('grimoire.list', { projectId });
  return r ?? []; // defensif bila server mengembalikan null
}

export function grimoireRead(projectId: string, path: string): Promise<{ content: string; size: number }> {
  return call<{ content: string; size: number }>('grimoire.read', { projectId, path });
}
