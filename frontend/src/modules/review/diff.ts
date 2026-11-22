// Klien diff review: tipe kontrak, pengambilan lewat IPC review.diff, dan normalisasi defensif.
import { call } from '../../lib/ipc';

export type DiffLineKind = 'ctx' | 'add' | 'del';
export interface DiffLine { kind: DiffLineKind; text: string; oldNo: number; newNo: number }
export interface Hunk { id: string; oldStart: number; oldLines: number; newStart: number; newLines: number; lines: DiffLine[] }
export interface DiffResult { hunks: Hunk[]; currentHash: string; binary: boolean; tooLarge: boolean }

const KINDS: DiffLineKind[] = ['ctx', 'add', 'del'];

function diffNum(v: unknown): number {
  return typeof v === 'number' ? v : 0;
}

function diffNormalizeHunk(raw: unknown, index: number): Hunk {
  const h = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const lines: DiffLine[] = [];
  if (Array.isArray(h.lines)) {
    for (const l of h.lines) {
      const o = (l && typeof l === 'object' ? l : {}) as Record<string, unknown>;
      if (!KINDS.includes(o.kind as DiffLineKind)) continue;
      lines.push({
        kind: o.kind as DiffLineKind,
        text: typeof o.text === 'string' ? o.text : '',
        oldNo: diffNum(o.oldNo),
        newNo: diffNum(o.newNo),
      });
    }
  }
  return {
    id: typeof h.id === 'string' ? h.id : String(index),
    oldStart: diffNum(h.oldStart),
    oldLines: diffNum(h.oldLines),
    newStart: diffNum(h.newStart),
    newLines: diffNum(h.newLines),
    lines,
  };
}

function diffNormalize(raw: unknown): DiffResult {
  if (!raw || typeof raw !== 'object') return { hunks: [], currentHash: '', binary: false, tooLarge: false };
  const r = raw as Record<string, unknown>;
  return {
    hunks: Array.isArray(r.hunks) ? r.hunks.map(diffNormalizeHunk) : [],
    currentHash: typeof r.currentHash === 'string' ? r.currentHash : '',
    binary: r.binary === true,
    tooLarge: r.tooLarge === true,
  };
}

export async function fetchDiff(sessionId: string, path: string): Promise<DiffResult> {
  const raw = await call<unknown>('review.diff', { sessionId, path });
  return diffNormalize(raw);
}

export function hunkSummary(hunk: Hunk): string {
  let add = 0;
  let del = 0;
  for (const l of hunk.lines) {
    if (l.kind === 'add') add++;
    else if (l.kind === 'del') del++;
  }
  return `+${add} -${del}`;
}
