import { readable, type Readable } from 'svelte/store';
import { call, inWails, appPid } from '../../lib/ipc';

export type ProcKind = 'daemon' | 'app' | 'session';
export interface Proc {
  label: string; kind: ProcKind; sessionId: string; pid: number; count: number; rssBytes: number; cpuPercent: number;
}
export interface Snapshot { at: number; totalRssBytes: number; totalCpuPercent: number; procs: Proc[] }
export interface ResourceState { snapshot: Snapshot | null; error: string }

const INTERVAL_MS = 5000;
let state: ResourceState = { snapshot: null, error: '' };
const subs = new Set<(s: ResourceState) => void>();

function set(next: ResourceState) {
  state = next;
  subs.forEach((fn) => fn(state));
}

export const resourceStore: Readable<ResourceState> = {
  subscribe(fn) {
    fn(state);
    subs.add(fn);
    return () => { subs.delete(fn); };
  },
};

let pidPromise: Promise<number> | null = null;
let stopFn: (() => void) | null = null;

export function startResourceSync(): () => void {
  if (!inWails) return () => {};
  if (stopFn) return stopFn;

  let timer: ReturnType<typeof setInterval> | null = null;
  let inFlight = false;

  async function tick() {
    if (inFlight) return;
    inFlight = true;
    try {
      pidPromise ??= appPid().catch(() => 0);
      const pid = await pidPromise;
      const snap = await call<Snapshot>('resource.snapshot', { appPid: pid });
      set({ snapshot: snap, error: '' });
    } catch (e) {
      set({ snapshot: state.snapshot, error: e instanceof Error ? e.message : String(e) });
    } finally {
      inFlight = false;
    }
  }

  function clear() {
    if (timer !== null) { clearInterval(timer); timer = null; }
  }

  function sync() {
    clear();
    if (document.visibilityState !== 'visible') return;
    void tick();
    timer = setInterval(() => void tick(), INTERVAL_MS);
  }

  document.addEventListener('visibilitychange', sync);
  sync();

  stopFn = () => {
    clear();
    document.removeEventListener('visibilitychange', sync);
    stopFn = null;
  };
  return stopFn;
}
