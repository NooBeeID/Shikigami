import { writable, type Readable } from 'svelte/store';
import { call, onEvent } from '../../lib/ipc';
import { layout } from '../../lib/stores/layout';
import { sessions } from '../../lib/stores/sessions';
import type { Session } from '../../lib/stores/types';
import { findLeaf, type LayoutDoc } from '../layout/model';
import type { ReviewFile, ReviewFilesResult, ReviewStatus } from './types';

const EMPTY: ReviewFilesResult = { supported: true, files: [] };
const filesStore = writable<ReviewFilesResult>(EMPTY);
export const reviewFiles: Readable<ReviewFilesResult> = { subscribe: filesStore.subscribe };

const countsStore = writable<Record<string, number>>({});
export const reviewCounts: Readable<Record<string, number>> = { subscribe: countsStore.subscribe };

let activeSessionId: string | null = null;
let loadSeq = 0;

const STATUSES: ReviewStatus[] = ['modified', 'added', 'deleted'];

function reviewIsFile(v: unknown): v is ReviewFile {
  const f = v as Partial<ReviewFile> | null;
  return !!f && typeof f === 'object' && typeof f.path === 'string'
    && STATUSES.includes(f.status as ReviewStatus);
}

function reviewNormalize(res: unknown): ReviewFilesResult {
  const r = res as { supported?: unknown; files?: unknown } | null;
  if (!r || typeof r !== 'object' || !Array.isArray(r.files)) return { supported: true, files: [] };
  const supported = r.supported !== false;
  if (!supported) return { supported: false, files: [] };
  const files = r.files.filter(reviewIsFile).map((f): ReviewFile => ({
    path: f.path,
    status: f.status,
    additions: typeof f.additions === 'number' ? f.additions : 0,
    deletions: typeof f.deletions === 'number' ? f.deletions : 0,
    reviewed: f.reviewed === true,
    external: f.external === true,
    binary: f.binary === true,
    tooLarge: f.tooLarge === true,
  }));
  return { supported, files };
}

function reviewUnreviewed(files: ReviewFile[]): number {
  return files.filter((f) => f.reviewed === false).length;
}

function reviewSetCount(sessionId: string, n: number) {
  countsStore.update((m) => {
    if (n <= 0) {
      if (!(sessionId in m)) return m;
      const next = { ...m };
      delete next[sessionId];
      return next;
    }
    if (m[sessionId] === n) return m;
    return { ...m, [sessionId]: n };
  });
}

// undefined = tidak berubah (pertahankan sesi sebelumnya); null = tidak ada sesi aktif.
function reviewActiveFromLayout(doc: LayoutDoc): string | null | undefined {
  if (doc.root === null) return null;
  const leaf = doc.focusId ? findLeaf(doc, doc.focusId) : null;
  if (leaf && leaf.kind === 'terminal' && leaf.sessionId) return leaf.sessionId;
  return undefined;
}

export async function refreshReview(sessionId: string): Promise<void> {
  if (!sessionId) return;
  const isActive = sessionId === activeSessionId;
  const seq = isActive ? ++loadSeq : loadSeq;
  let res: ReviewFilesResult;
  try {
    res = reviewNormalize(await call<unknown>('review.files', { sessionId }));
  } catch (e) {
    console.warn('review.files gagal:', e);
    res = { supported: true, files: [] };
  }
  reviewSetCount(sessionId, res.supported ? reviewUnreviewed(res.files) : 0);
  if (sessionId === activeSessionId && (!isActive || seq === loadSeq)) filesStore.set(res);
}

function reviewHandleChanged(_topic: string, payload: unknown) {
  const p = payload as { sessionId?: unknown; count?: unknown } | null;
  if (!p || typeof p !== 'object' || typeof p.sessionId !== 'string' || typeof p.count !== 'number') return;
  reviewSetCount(p.sessionId, p.count);
  if (p.sessionId === activeSessionId) void refreshReview(p.sessionId);
}

function reviewSetActive(next: string | null) {
  if (next === activeSessionId) return;
  activeSessionId = next;
  loadSeq++;
  filesStore.set(EMPTY);
  if (next !== null) void refreshReview(next);
}

const reviewSeen = new Set<string>();

function reviewSeed(list: Session[]) {
  const ids = new Set(list.map((s) => s.id));
  for (const s of list) {
    if (s.status === 'exited' || reviewSeen.has(s.id)) continue;
    reviewSeen.add(s.id);
    void refreshReview(s.id);
  }
  for (const id of [...reviewSeen]) {
    if (ids.has(id)) continue;
    reviewSeen.delete(id);
    reviewSetCount(id, 0);
  }
}

let unsub: (() => void) | null = null;
export function startReviewSync(): () => void {
  const stop = () => { if (unsub) { unsub(); unsub = null; loadSeq++; } };
  if (unsub) return stop;
  // Berlangganan event dulu agar tidak ada yang hilang, baru muat awal.
  const off = onEvent('review.changed', reviewHandleChanged);
  const offLayout = layout.subscribe((doc) => {
    const next = reviewActiveFromLayout(doc);
    if (next !== undefined) reviewSetActive(next);
  });
  const offSessions = sessions.subscribe(reviewSeed);
  unsub = () => { off(); offLayout(); offSessions(); };
  return stop;
}
