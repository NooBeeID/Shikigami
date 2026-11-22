import { fsRead, classifyError, FsBinaryError, FsNotFoundError, basename } from '../shared/fsClient';
import { CHUNK_SIZE, type ReadResultRaw } from '../shared/types';
import { call } from '../../../lib/ipc';

export type FileKind = 'markdown' | 'text';

export function fileKind(path: string): FileKind {
  const name = basename(path);
  const i = name.lastIndexOf('.');
  const ext = i < 0 ? '' : name.slice(i).toLowerCase();
  return ext === '.md' || ext === '.markdown' || ext === '.txt' ? 'markdown' : 'text';
}

export interface Chunk { content: string; size: number; mtime: number; eof: boolean; offset: number; length: number }

export class BinaryFileError extends Error {
  constructor(message?: string) { super(message); this.name = 'BinaryFileError'; }
}
export class FileNotFoundError extends Error {
  constructor(message?: string) { super(message); this.name = 'FileNotFoundError'; }
}

export const MARKDOWN_MAX_BYTES = 2 * 1024 * 1024;
export function isLargeForMarkdown(size: number): boolean {
  return size > MARKDOWN_MAX_BYTES;
}

function mapError(e: unknown): never {
  if (e instanceof FsBinaryError) throw new BinaryFileError(e.message);
  if (e instanceof FsNotFoundError) throw new FileNotFoundError(e.message);
  throw e;
}

function relUnderRoot(root: string, path: string): string {
  const r = root.length > 1 ? root.replace(/\/+$/, '') : root;
  const prefix = r.endsWith('/') ? r : r + '/';
  if (path.startsWith(prefix)) return path.slice(prefix.length);
  throw new Error('path di luar root: ' + path);
}

export async function readChunk(path: string, offset: number, length: number, root?: string): Promise<Chunk> {
  try {
    let r: ReadResultRaw;
    if (root) {
      const rel = relUnderRoot(root, path);
      try {
        r = await call<ReadResultRaw>('fs.read', { root, path: rel, offset, limit: length });
      } catch (e) {
        throw classifyError(e);
      }
    } else {
      r = await fsRead(path, offset, length);
    }
    return { content: r.content, size: r.size, mtime: r.mtime, eof: r.eof, offset: r.offset, length: r.length };
  } catch (e) {
    mapError(e);
  }
}

async function readAllOnce(path: string, root?: string): Promise<Chunk | null> {
  const parts: string[] = [];
  let offset = 0;
  let total = 0;
  let firstMtime: number | null = null;
  for (;;) {
    const c = await readChunk(path, offset, CHUNK_SIZE, root);
    if (firstMtime === null) firstMtime = c.mtime;
    else if (c.mtime !== firstMtime) return null;
    parts.push(c.content);
    total += c.length;
    offset = c.offset + c.length;
    if (c.eof) {
      return { content: parts.join(''), size: c.size, mtime: c.mtime, eof: true, offset: 0, length: total };
    }
    if (c.length === 0) throw new Error('Pembacaan file tidak maju');
  }
}

export async function readAll(path: string, root?: string): Promise<Chunk> {
  const first = await readAllOnce(path, root);
  if (first) return first;
  const second = await readAllOnce(path, root);
  if (second) return second;
  throw new Error('File berubah saat dibaca, coba lagi');
}
