import { call } from '../../../lib/ipc';
import { THREAD_METHODS, type Thread } from '../types';

export interface CloseUncommitted { path: string; status: string }
export interface CloseCommit { sha: string; subject: string }
export interface CloseWorktreeEntry {
  participant: { sessionId: string; name: string; projectId: string };
  path: string; branch: string; baseBranch: string;
  uncommitted: CloseUncommitted[]; unmergedCommits: CloseCommit[]; error?: string;
}
export interface ClosePreview { threadId: string; worktrees: CloseWorktreeEntry[] }

function asArray<T>(v: unknown): T[] {
  return Array.isArray(v) ? (v as T[]) : [];
}

function normEntry(e: Partial<CloseWorktreeEntry> | null | undefined): CloseWorktreeEntry {
  const x = e ?? {};
  return {
    participant: { sessionId: '', name: '', projectId: '', ...(x.participant ?? {}) },
    path: x.path ?? '',
    branch: x.branch ?? '',
    baseBranch: x.baseBranch ?? '',
    uncommitted: asArray<CloseUncommitted>(x.uncommitted),
    unmergedCommits: asArray<CloseCommit>(x.unmergedCommits),
    error: x.error,
  };
}

// Pratinjau pekerjaan yang akan hilang saat thread ditutup (read-only di daemon).
export async function fetchClosePreview(threadId: string): Promise<ClosePreview> {
  const res = await call<Partial<ClosePreview>>(THREAD_METHODS.closePreview, { id: threadId });
  return { threadId: res?.threadId ?? threadId, worktrees: asArray<CloseWorktreeEntry>(res?.worktrees).map(normEntry) };
}

export async function closeThread(threadId: string): Promise<Thread> {
  const t = await call<Thread>(THREAD_METHODS.close, { id: threadId });
  return { ...t, participants: Array.isArray(t?.participants) ? t.participants : [] };
}
