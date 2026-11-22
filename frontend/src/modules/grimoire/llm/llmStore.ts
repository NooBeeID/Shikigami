import { writable, type Readable } from 'svelte/store';
import { call, onEvent } from '../../../lib/ipc';

export interface LlmCost { modules: number; estTokens: number; durationMs: number; failed: number; at: string; skipped?: number; note?: string }
export interface LlmProgress { current: string; done: number; failed: number }
export interface LlmEstimate { modules: number; estTokens: number; overLimit?: number }
export interface LlmState {
  projectId: string | null; loaded: boolean;
  llm: boolean; maxModules: number | null;
  building: boolean; progress: LlmProgress | null; lastBuildCost: LlmCost | null;
  estimate: LlmEstimate | null; // pratinjau konfirmasi; non-null = menunggu konfirmasi
  busy: boolean; error: string | null;
}

// Tipe longgar: field yang belum dikirim daemon diperlakukan mati/kosong.
type RawSettings = { llm?: unknown; maxModules?: unknown };
type RawStatus = { building?: unknown; llmEnabled?: unknown; lastBuildCost?: unknown; llmProgress?: unknown };
type RawEstimate = { modules?: unknown; estTokens?: unknown; overLimit?: unknown };

const initial = (projectId: string | null): LlmState => ({
  projectId,
  loaded: false,
  llm: false,
  maxModules: null,
  building: false,
  progress: null,
  lastBuildCost: null,
  estimate: null,
  busy: false,
  error: null,
});

const state = writable<LlmState>(initial(null));
export const llmState: Readable<LlmState> = { subscribe: state.subscribe };

let currentId: string | null = null;
let seq = 0;

function msg(e: unknown): string {
  const m = e instanceof Error ? e.message : String(e);
  return m.length > 200 ? m.slice(0, 200) + '...' : m;
}

const isObj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null;
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);

function asMaxModules(v: unknown): number | null {
  return typeof v === 'number' && Number.isInteger(v) && v >= 1 ? v : null;
}

function asCost(v: unknown): LlmCost | null {
  if (!isObj(v)) return null;
  const cost: LlmCost = {
    modules: num(v.modules),
    estTokens: num(v.estTokens),
    durationMs: num(v.durationMs),
    failed: num(v.failed),
    at: typeof v.at === 'string' ? v.at : '',
  };
  if (typeof v.skipped === 'number' && v.skipped > 0) cost.skipped = v.skipped;
  if (typeof v.note === 'string' && v.note !== '') cost.note = v.note;
  return cost;
}

function asProgress(v: unknown): LlmProgress | null {
  if (!isObj(v)) return null;
  return {
    current: typeof v.current === 'string' ? v.current : '',
    done: num(v.done),
    failed: num(v.failed),
  };
}

function asEstimate(v: unknown): LlmEstimate {
  const r: RawEstimate = isObj(v) ? v : {};
  const est: LlmEstimate = { modules: num(r.modules), estTokens: num(r.estTokens) };
  if (typeof r.overLimit === 'number' && r.overLimit > 0) est.overLimit = r.overLimit;
  return est;
}

async function load(id: string, withSettings: boolean): Promise<void> {
  const my = ++seq;
  const [st, se] = await Promise.allSettled([
    call<RawStatus | null>('grimoire.status', { projectId: id }),
    withSettings
      ? call<RawSettings | null>('grimoire.settings.get', { projectId: id })
      : Promise.resolve(undefined),
  ]);
  if (my !== seq || currentId !== id) return;

  const status = st.status === 'fulfilled' ? st.value : undefined;
  const statusOk = st.status === 'fulfilled';
  const settingsOk = se.status === 'fulfilled' && withSettings;
  const settings = se.status === 'fulfilled' ? se.value : undefined;

  let error: string | null = null;
  if (st.status === 'rejected' && se.status === 'rejected') error = msg(st.reason);
  else if (st.status === 'rejected') error = msg(st.reason);
  else if (se.status === 'rejected') error = msg(se.reason);

  state.update((s) => {
    const next: LlmState = { ...s, loaded: true, error };
    if (statusOk) {
      next.building = status?.building === true;
      next.progress = asProgress(status?.llmProgress);
      next.lastBuildCost = asCost(status?.lastBuildCost);
    }
    if (settingsOk) {
      next.llm = settings?.llm === true;
      next.maxModules = asMaxModules(settings?.maxModules);
    } else if (statusOk && typeof status?.llmEnabled === 'boolean') {
      next.llm = status.llmEnabled;
    }
    return next;
  });
}

export function startLlmStore(projectId: string): () => void {
  currentId = projectId;
  seq++;
  state.set(initial(projectId));
  void load(projectId, true);
  const off = onEvent('grimoire.changed', (_t, payload) => {
    if (typeof payload !== 'object' || !payload || !('projectId' in payload)) return;
    const pid = (payload as { projectId?: unknown }).projectId;
    if (typeof pid !== 'string' || pid !== currentId) return;
    void load(pid, true);
  });
  return () => {
    off();
    if (currentId === projectId) {
      currentId = null;
      seq++;
      state.set(initial(null));
    }
  };
}

let busyNow = false;
let llmNow = false;
let estimateNow: LlmEstimate | null = null;
state.subscribe((s) => {
  busyNow = s.busy;
  llmNow = s.llm;
  estimateNow = s.estimate;
});

export async function requestEnable(): Promise<void> {
  const id = currentId;
  if (!id || busyNow || llmNow) return;
  state.update((s) => ({ ...s, busy: true, error: null }));
  try {
    const raw = await call<RawEstimate | null>('grimoire.estimate', { projectId: id });
    if (currentId !== id) return;
    state.update((s) => ({ ...s, estimate: asEstimate(raw), busy: false }));
  } catch (e) {
    if (currentId !== id) return;
    state.update((s) => ({ ...s, error: msg(e), busy: false, estimate: null }));
  }
}

export async function confirmEnable(): Promise<void> {
  const id = currentId;
  if (!id || busyNow || !estimateNow) return;
  state.update((s) => ({ ...s, busy: true }));
  try {
    const res = await call<RawSettings | null>('grimoire.settings.set', { projectId: id, llm: true });
    if (currentId !== id) return;
    state.update((s) => ({
      ...s,
      llm: isObj(res) && typeof res.llm === 'boolean' ? res.llm : true,
      maxModules: isObj(res) && 'maxModules' in res ? asMaxModules(res.maxModules) : s.maxModules,
      estimate: null,
      busy: false,
      error: null,
    }));
  } catch (e) {
    if (currentId !== id) return;
    state.update((s) => ({ ...s, error: msg(e), busy: false }));
  }
}

export function cancelEnable(): void {
  state.update((s) => ({ ...s, estimate: null }));
}

export async function disableLlm(): Promise<void> {
  const id = currentId;
  if (!id) return;
  state.update((s) => ({ ...s, busy: true }));
  try {
    await call('grimoire.settings.set', { projectId: id, llm: false });
    if (currentId !== id) return;
    state.update((s) => ({ ...s, llm: false, estimate: null, busy: false, error: null }));
  } catch (e) {
    if (currentId !== id) return;
    state.update((s) => ({ ...s, error: msg(e), busy: false }));
  }
}

export async function setMaxModules(v: number | null): Promise<void> {
  const id = currentId;
  if (!id) return;
  if (v !== null && (!Number.isInteger(v) || v < 1)) {
    state.update((s) => ({ ...s, error: 'maxModules harus integer >= 1' }));
    return;
  }
  try {
    const res = await call<RawSettings | null>('grimoire.settings.set', { projectId: id, maxModules: v });
    if (currentId !== id) return;
    state.update((s) => ({
      ...s,
      maxModules: isObj(res) && 'maxModules' in res ? asMaxModules(res.maxModules) : v,
      error: null,
    }));
  } catch (e) {
    if (currentId !== id) return;
    state.update((s) => ({ ...s, error: msg(e) }));
  }
}

export function formatTokens(n: number): string {
  if (!Number.isFinite(n) || n < 0) return '0';
  if (n < 1000) return String(Math.round(n));
  if (n < 1_000_000) return `${(n / 1000).toFixed(n < 10_000 ? 1 : 0).replace(/\.0$/, '')}k`;
  return `${(n / 1e6).toFixed(1)}M`;
}

export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return '0 ms';
  if (ms < 1000) return `${Math.round(ms)} ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} dtk`;
  const total = Math.round(ms / 1000);
  return `${Math.floor(total / 60)} mnt ${total % 60} dtk`;
}
