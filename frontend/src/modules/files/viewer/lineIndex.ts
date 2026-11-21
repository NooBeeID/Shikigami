// Pembaca berjenjang untuk PlainTextView: indeks baris berurutan + cache LRU potongan.
import { CHUNK_SIZE } from '../shared/types';
import { readChunk } from './fileApi';

export interface ChunkMeta { index: number; offset: number; byteLen: number; firstLine: number; lineCount: number }

const DEFAULT_MAX_ROWS = 500_000;
const DEFAULT_MAX_CACHED = 6;
const MAX_LINE_CHARS = 10_000;

function splitLines(text: string): string[] {
  const parts = text.split('\n');
  if (parts.length > 0 && parts[parts.length - 1] === '') parts.pop();
  return parts;
}

function displayLine(raw: string): string {
  let l = raw;
  if (l.length > 0 && l.charCodeAt(l.length - 1) === 13) l = l.slice(0, -1);
  if (l.length > MAX_LINE_CHARS) l = l.slice(0, MAX_LINE_CHARS) + '…';
  return l;
}

const encoder = new TextEncoder();

export class LineReader {
  readonly chunks: ChunkMeta[] = [];
  stale = false;
  truncated = false;

  private readonly path: string;
  private readonly size: number;
  private readonly maxRows: number;
  private readonly maxCached: number;
  private _indexed = 0;
  private _complete = false;
  private _maxCols = 0;
  private _mtime: number | null = null;
  private nextOffset = 0;
  private bytesIndexed = 0;
  private cache = new Map<number, string[]>();
  private inflight = new Map<number, Promise<string[]>>();
  private lock: Promise<void> = Promise.resolve();

  constructor(path: string, size: number, opts?: { maxCached?: number; maxRows?: number }) {
    this.path = path;
    this.size = size;
    this.maxRows = opts?.maxRows ?? DEFAULT_MAX_ROWS;
    this.maxCached = Math.max(2, opts?.maxCached ?? DEFAULT_MAX_CACHED);
  }

  get indexedLines(): number { return this._indexed; }
  get complete(): boolean { return this._complete; }
  get maxCols(): number { return this._maxCols; }
  get mtime(): number | null { return this._mtime; }

  get totalLines(): number {
    if (this._complete) return this._indexed;
    if (this._indexed === 0) return Math.min(this.maxRows, Math.max(1, Math.ceil(this.size / 40)));
    const avg = Math.max(1, this.bytesIndexed / this._indexed);
    return Math.min(this.maxRows, Math.max(this._indexed, Math.ceil(this.size / avg)));
  }

  private remember(index: number, lines: string[]) {
    this.cache.delete(index);
    this.cache.set(index, lines);
    while (this.cache.size > this.maxCached) {
      const k = this.cache.keys().next().value as number;
      this.cache.delete(k);
    }
  }

  private async readNext(): Promise<void> {
    const offset = this.nextOffset;
    const c = await readChunk(this.path, offset, CHUNK_SIZE);
    if (this._mtime === null) this._mtime = c.mtime;
    else if (c.mtime !== this._mtime) { this.stale = true; return; }
    if (c.length === 0 && !c.eof) throw new Error('Pembacaan file tidak maju');

    let rawLines: string[];
    let byteLen: number;
    let allConsumed = false;
    if (c.eof) {
      rawLines = splitLines(c.content);
      byteLen = c.length;
      allConsumed = true;
    } else {
      const idx = c.content.lastIndexOf('\n');
      if (idx < 0) {
        rawLines = [c.content]; // baris raksasa: konsumsi seluruh potongan
        byteLen = c.length;
        allConsumed = true;
      } else {
        const consumed = c.content.slice(0, idx + 1);
        rawLines = splitLines(consumed);
        byteLen = encoder.encode(consumed).length;
      }
    }

    const room = this.maxRows - this._indexed;
    if (rawLines.length > room) {
      rawLines = rawLines.slice(0, Math.max(0, room));
      byteLen = encoder.encode(rawLines.length ? rawLines.join('\n') + '\n' : '').length;
      this.truncated = true;
      allConsumed = false;
    }

    const lines = rawLines.map(displayLine);
    for (const l of lines) if (l.length > this._maxCols) this._maxCols = l.length;

    if (lines.length > 0) {
      const meta: ChunkMeta = { index: this.chunks.length, offset, byteLen, firstLine: this._indexed, lineCount: lines.length };
      this.chunks.push(meta);
      this.remember(meta.index, lines);
      this._indexed += lines.length;
      this.bytesIndexed += byteLen;
    }
    this.nextOffset = offset + byteLen;
    if (c.eof && !this.truncated) this._complete = true;
    else if (byteLen === 0 && !this.truncated && !allConsumed) throw new Error('Pembacaan file tidak maju');
  }

  ensureIndexed(row: number): Promise<void> {
    const run = this.lock.then(async () => {
      while (this._indexed <= row && !this._complete && !this.truncated && !this.stale) {
        await this.readNext();
      }
    });
    this.lock = run.catch(() => {});
    return run;
  }

  private findChunk(line: number): number {
    let lo = 0;
    let hi = this.chunks.length - 1;
    while (lo <= hi) {
      const mid = (lo + hi) >> 1;
      const m = this.chunks[mid];
      if (line < m.firstLine) hi = mid - 1;
      else if (line >= m.firstLine + m.lineCount) lo = mid + 1;
      else return mid;
    }
    return -1;
  }

  private loadChunk(index: number): Promise<string[]> {
    const hit = this.cache.get(index);
    if (hit) {
      this.remember(index, hit);
      return Promise.resolve(hit);
    }
    const pending = this.inflight.get(index);
    if (pending) return pending;
    const meta = this.chunks[index];
    const p = (async () => {
      const c = await readChunk(this.path, meta.offset, meta.byteLen);
      if (this._mtime !== null && c.mtime !== this._mtime) this.stale = true;
      const lines = splitLines(c.content).map(displayLine);
      this.remember(index, lines);
      return lines;
    })();
    this.inflight.set(index, p);
    const done = () => { this.inflight.delete(index); };
    p.then(done, done);
    return p;
  }

  async getLines(from: number, to: number): Promise<string[] | null> {
    const lo = Math.max(0, from);
    const hi = Math.min(to, this._indexed);
    const out: string[] = [];
    let line = lo;
    while (line < hi) {
      const ci = this.findChunk(line);
      if (ci < 0) return null;
      const meta = this.chunks[ci];
      const lines = await this.loadChunk(ci);
      const end = Math.min(hi, meta.firstLine + meta.lineCount);
      for (let l = line; l < end; l++) out.push(lines[l - meta.firstLine] ?? '');
      line = end;
    }
    return out;
  }

  peekLines(from: number, to: number): (string | undefined)[] {
    const out: (string | undefined)[] = [];
    const hi = Math.min(to, this._indexed);
    for (let line = Math.max(0, from); line < hi; line++) {
      const ci = this.findChunk(line);
      const lines = ci < 0 ? undefined : this.cache.get(ci);
      out.push(lines ? lines[line - this.chunks[ci].firstLine] : undefined);
    }
    return out;
  }
}
