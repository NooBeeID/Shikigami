// Satu-satunya tempat memanggil call('fs.*') dan memegang bentuk pesan fs.*.
import { get } from 'svelte/store';
import { call, onEvent } from '../../../lib/ipc';
import type { FsEntry, ReadResultRaw, FileChangedEvent } from './types';
import { activeWorkspace } from './shellBridge';

export class FsBinaryError extends Error {}
export class FsConflictError extends Error { currentMtime?: number }
export class FsNotFoundError extends Error {}
export class FsInvalidError extends Error {}

// Bentuk error nyata di JS: Error(string) dari Wails, string = ipc.Error.Error() =
// "<pesan> (kode -32001)", mis. "file changed on disk (disk mtime 1700000000000000000) (kode -32001)"
// atau "binary file not supported: a.bin (kode -32000)" (Wrap: "<sentinel>: <detail>").
export function classifyError(e: unknown): unknown {
  const any = e as { code?: unknown; message?: unknown; data?: any } | null;
  const msg = typeof any?.message === 'string' ? any.message : String(e);
  let code: number | undefined = typeof any?.code === 'number' ? any.code : undefined;
  if (code === undefined) {
    const m = /-3200[0-3]/.exec(msg);
    if (m) code = Number(m[0]);
  }
  if (code === undefined) {
    if (msg.includes('binary file not supported')) code = -32000;
    else if (msg.includes('file changed on disk')) code = -32001;
    else if (msg.includes('not found')) code = -32002;
    else if (msg.includes('invalid argument')) code = -32003;
  }
  switch (code) {
    case -32000: return new FsBinaryError(msg);
    case -32001: {
      const err = new FsConflictError(msg);
      const d = any?.data?.currentMtime;
      if (typeof d === 'number') err.currentMtime = d;
      else {
        const m = /disk mtime (\d+)/.exec(msg);
        if (m) err.currentMtime = Number(m[1]);
      }
      return err;
    }
    case -32002: return new FsNotFoundError(msg);
    case -32003: return new FsInvalidError(msg);
    default: return e;
  }
}

// mtime = token opaque; satu-satunya tempat penyesuaian bila presisi ns terbukti hilang.
export function normalizeMtime(n: number): number { return n; }

export function splitPath(absPath: string): { root: string; rel: string } {
  const ws = get(activeWorkspace);
  let best = '';
  for (const p of ws?.projects ?? []) {
    const root = p.path.length > 1 ? p.path.replace(/\/+$/, '') : p.path;
    if ((absPath === root || absPath.startsWith(root + '/')) && root.length > best.length) best = root;
  }
  if (!best) throw new FsInvalidError('path di luar project: ' + absPath);
  return { root: best, rel: absPath.slice(best.length).replace(/^\/+/, '') };
}

export function joinPath(dirAbs: string, name: string): string {
  return dirAbs.replace(/\/+$/, '') + '/' + name;
}

export function basename(path: string): string {
  const i = path.lastIndexOf('/');
  return i < 0 ? path : path.slice(i + 1);
}

export async function fsList(absDir: string, root?: string): Promise<FsEntry[]> {
  try {
    let r = root;
    let relDir: string;
    if (r) {
      relDir = absDir === r ? '' : absDir.slice(r.length).replace(/^\/+/, '');
    } else {
      const s = splitPath(absDir);
      r = s.root;
      relDir = s.rel;
    }
    const res = await call<{ entries: FsEntry[] | null }>('fs.list', { root: r, relDir });
    return (res?.entries ?? []).map((e) => ({ ...e, mtime: normalizeMtime(e.mtime) }));
  } catch (e) {
    throw classifyError(e);
  }
}

export async function fsRead(absPath: string, offset = 0, limit = 0): Promise<ReadResultRaw> {
  try {
    const { root, rel } = splitPath(absPath);
    const res = await call<ReadResultRaw>('fs.read', { root, path: rel, offset, limit });
    return { ...res, mtime: normalizeMtime(res.mtime) };
  } catch (e) {
    throw classifyError(e);
  }
}

export async function fsWrite(absPath: string, content: string, expectedMtime: number): Promise<{ mtime: number; size: number }> {
  try {
    const { root, rel } = splitPath(absPath);
    const res = await call<{ mtime: number; size: number }>('fs.write', {
      root, path: rel, content, expectedMtime: normalizeMtime(expectedMtime),
    });
    return { ...res, mtime: normalizeMtime(res.mtime) };
  } catch (e) {
    throw classifyError(e);
  }
}

export function watchFile(absPath: string, onChange: (ev: FileChangedEvent) => void): () => void {
  const topicWanted = 'fs.changed.' + absPath;
  let active = true;
  let split: { root: string; rel: string };
  try {
    split = splitPath(absPath);
  } catch (e) {
    throw classifyError(e);
  }
  const off = onEvent(topicWanted, (topic, payload) => {
    if (active && topic === topicWanted) onChange(payload as FileChangedEvent);
  });
  call('fs.watch', { root: split.root, path: split.rel }).catch(() => {});
  return () => {
    if (!active) return;
    active = false;
    off();
    call('fs.unwatch', { root: split.root, path: split.rel }).catch(() => {});
  };
}
