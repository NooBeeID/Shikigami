import { derived, get, writable, type Readable } from 'svelte/store';
import { call, onEvent } from '../../lib/ipc';
import { activeWorkspaceId } from '../../lib/stores/workspaces';
import { sessions } from '../../lib/stores/sessions';
import type { Run } from './types';

export type { Run, Progress, AgentLine } from './types';

export type RunsBySession = Record<string, Run[]>;

// Penjaga tipe sempit: abaikan run yang bentuknya tidak valid.
export function isRun(v: unknown): v is Run {
  const r = v as Partial<Run> | null;
  return !!r && typeof r === 'object'
    && typeof r.id === 'string' && typeof r.sessionId === 'string'
    && (r.kind === 'workflow' || r.kind === 'subagent')
    && typeof r.startedAt === 'number';
}

// Ganti entri satu sesi; daftar kosong berarti sesi itu tidak punya run.
export function replaceSession(m: RunsBySession, sessionId: string, list: Run[]): RunsBySession {
  const next = { ...m };
  if (list.length === 0) delete next[sessionId];
  else next[sessionId] = list;
  return next;
}

// Kelompokkan daftar datar run per sesi.
export function groupBySession(list: Run[]): RunsBySession {
  const out: RunsBySession = {};
  for (const r of list) (out[r.sessionId] ??= []).push(r);
  return out;
}

// Urut startedAt naik, seri diurutkan id.
export function sortRuns(list: Run[]): Run[] {
  return [...list].sort((a, b) => a.startedAt - b.startedAt || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
}

// Hanya run milik sesi workspace aktif, sudah terurut.
export function visibleRuns(m: RunsBySession, sessionIds: Iterable<string>): Run[] {
  const out: Run[] = [];
  for (const id of sessionIds) {
    const l = m[id];
    if (l) out.push(...l);
  }
  return sortRuns(out);
}

const store = writable<RunsBySession>({});

export const runs: Readable<Run[]> = derived([store, sessions], ([$m, $s]) =>
  visibleRuns($m, $s.map((s) => s.id)));
export const runsCount: Readable<number> = derived(runs, ($r) => $r.length);

// Sesi yang diubah event selama panggilan muat berjalan: nilai event lebih baru
// daripada balasan, jadi dipertahankan.
let touched = new Set<string>();

// Nomor muat terbaru: hanya balasan dari muat terbaru yang boleh menyentuh store.
let loadSeq = 0;
export function isLatestLoad(seq: number, current: number): boolean {
  return seq === current;
}

function handler(_topic: string, payload: unknown): void {
  const p = payload as { sessionId?: unknown; runs?: unknown } | null;
  if (!p || typeof p.sessionId !== 'string' || !Array.isArray(p.runs)) return;
  const id = p.sessionId;
  const list = p.runs.filter(isRun).filter((r) => r.sessionId === id);
  touched.add(id);
  store.update((m) => replaceSession(m, id, list));
}

// Muat terbaru yang gagal dan berjenis runs.detail memicu satu muat runs.list
// sebagai cadangan; runs.list yang gagal tidak mengulang (tanpa loop).
export function shouldFallbackToList(method: string, seq: number, current: number): boolean {
  return method === 'runs.detail' && isLatestLoad(seq, current);
}

// Ganti isi store dengan hasil panggilan workspace `wid`, kecuali sesi yang
// baru diubah event, dan abaikan balasan basi bila workspace sudah berganti.
function apply(seq: number, wid: string, res: unknown, mark: Set<string>): void {
  if (!isLatestLoad(seq, loadSeq) || get(activeWorkspaceId) !== wid || !Array.isArray(res)) return;
  const fresh = groupBySession(res.filter(isRun));
  store.update((m) => {
    const next = { ...fresh };
    for (const id of mark) {
      if (m[id]) next[id] = m[id];
      else delete next[id];
    }
    return next;
  });
}

async function load(method: 'runs.list' | 'runs.detail'): Promise<void> {
  const seq = ++loadSeq;
  const wid = get(activeWorkspaceId);
  if (!wid) { store.set({}); return; }
  touched = new Set();
  const mark = touched;
  try {
    apply(seq, wid, await call<Run[]>(method, { workspaceId: wid }), mark);
  } catch (e) {
    console.warn(`${method} gagal:`, e);
    // seq masih terbaru berarti belum ada stop() atau muat lain; cek workspace aktif.
    if (shouldFallbackToList(method, seq, loadSeq) && get(activeWorkspaceId) === wid) {
      void load('runs.list');
    }
  }
}

// Panggil runs.detail (rekonsiliasi dari berkas Claude) untuk workspace aktif.
// Gagal tidak mengosongkan daftar.
export async function refreshDetail(): Promise<void> {
  if (!get(activeWorkspaceId)) return;
  await load('runs.detail');
}

let unsub: (() => void) | null = null;

// Idempoten: panggilan berikutnya mengembalikan pembersih yang sama.
export function startRunsSync(): () => void {
  const stop = () => {
    if (unsub) { unsub(); unsub = null; loadSeq++; }
  };
  if (unsub) return stop;
  // Berlangganan dahulu agar tidak ada event yang hilang, baru muat awal.
  const off = onEvent('runs.changed', handler);
  // subscribe memanggil callback langsung: itu sekaligus muat awal.
  const offWs = activeWorkspaceId.subscribe(() => { void load('runs.list'); });
  unsub = () => { off(); offWs(); };
  return stop;
}
