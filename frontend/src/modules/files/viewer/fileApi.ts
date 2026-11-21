import { fsRead, FsBinaryError, FsNotFoundError, basename } from '../shared/fsClient';
import { CHUNK_SIZE } from '../shared/types';

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

export async function readChunk(path: string, offset: number, length: number): Promise<Chunk> {
  try {
    const r = await fsRead(path, offset, length);
    return { content: r.content, size: r.size, mtime: r.mtime, eof: r.eof, offset: r.offset, length: r.length };
  } catch (e) {
    mapError(e);
  }
}

async function readAllOnce(path: string): Promise<Chunk | null> {
  const parts: string[] = [];
  let offset = 0;
  let total = 0;
  let firstMtime: number | null = null;
  for (;;) {
    const c = await readChunk(path, offset, CHUNK_SIZE);
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

export async function readAll(path: string): Promise<Chunk> {
  const first = await readAllOnce(path);
  if (first) return first;
  const second = await readAllOnce(path);
  if (second) return second;
  throw new Error('File berubah saat dibaca, coba lagi');
}
