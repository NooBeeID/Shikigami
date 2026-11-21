import { SvelteMap, SvelteSet } from 'svelte/reactivity';
import { fsList, joinPath, FsNotFoundError, FsInvalidError } from '../shared/fsClient';
import type { FsEntry } from '../shared/types';
import type { ShellProject } from '../shared/shellBridge';

export type NodeStatus = 'idle' | 'loading' | 'error';

export interface TreeRow {
  key: string;
  name: string;
  path: string;
  root: string;
  depth: number;
  kind: 'project' | 'dir' | 'file' | 'symlink';
  expanded: boolean;
  status: NodeStatus;
  error?: string;
  placeholder?: 'loading' | 'empty' | 'error';
}

function errorMessage(e: unknown): string {
  if (e instanceof FsNotFoundError) return 'Folder tidak ditemukan.';
  if (e instanceof FsInvalidError) return 'Path tidak valid.';
  if (e instanceof Error && e.message) return e.message;
  return 'Gagal memuat folder.';
}

export class TreeModel {
  projects = $state<ShellProject[]>([]);

  private cache = new SvelteMap<string, FsEntry[]>();
  private expanded = new SvelteSet<string>();
  private status = new SvelteMap<string, { state: NodeStatus; error?: string }>();
  private rootOf = new Map<string, string>();
  private tokens = new Map<string, number>();
  private generation = 0;
  private counter = 0;

  rows: TreeRow[] = $derived.by(() => {
    const out: TreeRow[] = [];
    for (const p of this.projects) {
      this.appendDir(out, p.path, p.path, p.name, 0, 'project');
    }
    return out;
  });

  private appendDir(
    out: TreeRow[],
    path: string,
    root: string,
    name: string,
    depth: number,
    kind: 'project' | 'dir',
  ): void {
    const isOpen = this.expanded.has(path);
    const st = this.status.get(path);
    out.push({
      key: path,
      name,
      path,
      root,
      depth,
      kind,
      expanded: isOpen,
      status: st?.state ?? 'idle',
      error: st?.error,
    });
    if (!isOpen) return;
    const entries = this.cache.get(path);
    if (!entries) {
      if (st?.state === 'error') {
        out.push(this.placeholder(path, root, depth + 1, 'error', st.error));
      } else {
        out.push(this.placeholder(path, root, depth + 1, 'loading'));
      }
      return;
    }
    if (entries.length === 0) {
      out.push(this.placeholder(path, root, depth + 1, 'empty'));
      return;
    }
    for (const e of entries) {
      const childPath = joinPath(path, e.name);
      if (e.type === 'dir') {
        this.appendDir(out, childPath, root, e.name, depth + 1, 'dir');
      } else {
        out.push({
          key: childPath,
          name: e.name,
          path: childPath,
          root,
          depth: depth + 1,
          kind: e.type === 'symlink' ? 'symlink' : 'file',
          expanded: false,
          status: 'idle',
        });
      }
    }
  }

  private placeholder(
    parent: string,
    root: string,
    depth: number,
    kind: 'loading' | 'empty' | 'error',
    error?: string,
  ): TreeRow {
    return {
      key: parent,
      name: '',
      path: parent,
      root,
      depth,
      kind: 'file',
      expanded: false,
      status: 'idle',
      error,
      placeholder: kind,
    };
  }

  setProjects(projects: ShellProject[]): void {
    this.generation++;
    this.cache.clear();
    this.expanded.clear();
    this.status.clear();
    this.rootOf.clear();
    this.tokens.clear();
    for (const p of projects) this.rootOf.set(p.path, p.path);
    this.projects = projects;
  }

  toggle(row: TreeRow): void {
    if (row.kind !== 'project' && row.kind !== 'dir') return;
    if (this.expanded.has(row.path)) {
      this.expanded.delete(row.path);
      return;
    }
    this.expanded.add(row.path);
    this.rootOf.set(row.path, row.root);
    void this.loadDir(row.path, row.root);
  }

  retry(row: TreeRow): void {
    if (this.status.get(row.path)?.state !== 'error') return;
    this.status.delete(row.path);
    this.tokens.delete(row.path);
    void this.loadDir(row.path, row.root);
  }

  refreshAll(): void {
    this.generation++;
    this.cache.clear();
    this.status.clear();
    this.tokens.clear();
    for (const path of [...this.expanded]) {
      const root = this.rootOf.get(path);
      if (root) void this.loadDir(path, root);
    }
  }

  refreshDir(row: TreeRow): void {
    const prefix = row.path + '/';
    const affected = (p: string) => p === row.path || p.startsWith(prefix);
    for (const p of [...this.cache.keys()]) if (affected(p)) this.cache.delete(p);
    for (const p of [...this.status.keys()]) if (affected(p)) this.status.delete(p);
    for (const p of [...this.tokens.keys()]) if (affected(p)) this.tokens.delete(p);
    for (const p of [...this.expanded]) {
      if (!affected(p)) continue;
      const root = this.rootOf.get(p);
      if (root) void this.loadDir(p, root);
    }
  }

  private async loadDir(absDir: string, root: string): Promise<void> {
    if (this.cache.has(absDir)) return;
    if (this.status.get(absDir)?.state === 'loading') return;
    this.status.set(absDir, { state: 'loading' });
    const gen = this.generation;
    const token = ++this.counter;
    this.tokens.set(absDir, token);
    try {
      const entries = await fsList(absDir, root);
      if (gen !== this.generation || this.tokens.get(absDir) !== token) return;
      for (const e of entries) {
        if (e.type === 'dir') this.rootOf.set(joinPath(absDir, e.name), root);
      }
      this.cache.set(absDir, entries);
      this.status.set(absDir, { state: 'idle' });
    } catch (e) {
      if (gen !== this.generation || this.tokens.get(absDir) !== token) return;
      console.warn('fsList gagal', absDir, e);
      this.status.set(absDir, { state: 'error', error: errorMessage(e) });
    }
  }
}
