// Model pohon pane: tipe dan fungsi murni (tanpa IPC, tanpa mutasi masukan).

export type PaneKind = 'terminal' | 'file' | 'thread' | 'lark-board' | 'lark-doc'; // string terbuka lewat registerPaneKind
export interface Leaf {
  type: 'pane';
  id: string;
  kind: PaneKind;
  sessionId?: string;
  projectId?: string;
  provider?: string;
  filePath?: string;
  threadId?: string;
  title: string;
  workspaceId?: string; // tambahan: untuk pane file
  root?: string; // root eksplisit untuk pane file read-only (mis. direktori grimoire)
  readOnly?: boolean;
  // Satu-satunya pane terminal untuk agent thread; membuka agent lain mengganti sesinya.
  threadAgent?: boolean;
  sourceId?: string;
  tableId?: string;
  viewId?: string;
  nodeToken?: string;
}
export interface Split {
  type: 'split';
  dir: 'h' | 'v'; // h: a kiri/b kanan; v: a atas/b bawah
  ratio: number;
  a: Node;
  b: Node;
}
export type Node = Leaf | Split;
export interface LayoutDoc {
  version: 1;
  root: Node | null;
  focusId?: string;
  maximizedId?: string | null;
}
export type DropZone = 'top' | 'bottom' | 'left' | 'right' | 'center';
export type SplitPath = string;
export interface NewLeaf {
  kind: PaneKind;
  threadAgent?: boolean;
  sessionId?: string;
  projectId?: string;
  provider?: string;
  filePath?: string;
  threadId?: string;
  workspaceId?: string;
  root?: string;
  readOnly?: boolean;
  title?: string;
  sourceId?: string;
  tableId?: string;
  viewId?: string;
  nodeToken?: string;
}

export const emptyDoc = (): LayoutDoc => ({ version: 1, root: null });

export const clampRatio = (r: number): number => {
  if (typeof r !== 'number' || Number.isNaN(r)) return 0.5;
  return Math.min(0.9, Math.max(0.1, r));
};

let counter = 0;
function newLeafId(): string {
  let hex = '';
  try {
    hex = crypto.randomUUID().replace(/-/g, '').slice(0, 10);
  } catch {
    counter += 1;
    hex = (Date.now().toString(16) + counter.toString(16).padStart(4, '0')).slice(-10).padStart(8, '0');
  }
  return `p_${hex}`;
}

const isDoc = (n: Node | LayoutDoc | null): n is LayoutDoc =>
  !!n && (n as LayoutDoc).version === 1 && 'root' in (n as LayoutDoc);

export const leaves = (n: Node | LayoutDoc | null): Leaf[] => {
  const node = isDoc(n) ? n.root : (n as Node | null);
  const out: Leaf[] = [];
  const walk = (x: Node | null) => {
    if (!x) return;
    if (x.type === 'pane') out.push(x);
    else {
      walk(x.a);
      walk(x.b);
    }
  };
  walk(node);
  return out;
};

export const findLeafBy = (doc: LayoutDoc, pred: (l: Leaf) => boolean): Leaf | null =>
  leaves(doc).find(pred) ?? null;

export const findLeaf = (doc: LayoutDoc, id: string): Leaf | null => findLeafBy(doc, (l) => l.id === id);

export const pathOf = (doc: LayoutDoc, id: string): SplitPath | null => {
  const walk = (n: Node | null, path: string[]): string[] | null => {
    if (!n) return null;
    if (n.type === 'pane') return n.id === id ? path : null;
    return walk(n.a, [...path, 'a']) ?? walk(n.b, [...path, 'b']);
  };
  const r = walk(doc.root, []);
  return r ? r.join('.') : null;
};

const segs = (path: SplitPath): string[] => (path === '' ? [] : path.split('.'));

function getAt(doc: LayoutDoc, path: SplitPath): Node | null {
  let cur: Node | null = doc.root;
  for (const s of segs(path)) {
    if (!cur || cur.type !== 'split') return null;
    if (s !== 'a' && s !== 'b') return null;
    cur = cur[s];
  }
  return cur;
}

function replaceAt(root: Node | null, path: SplitPath, node: Node | null): Node | null {
  const parts = segs(path);
  const rec = (cur: Node | null, i: number): Node | null => {
    if (i === parts.length) return node;
    if (!cur || cur.type !== 'split') return cur;
    const k = parts[i] as 'a' | 'b';
    if (k !== 'a' && k !== 'b') return cur;
    return { ...cur, [k]: rec(cur[k], i + 1) } as Split;
  };
  return rec(root, 0);
}

const basename = (p?: string): string | undefined => {
  if (!p) return undefined;
  const parts = p.split(/[\\/]/).filter(Boolean);
  return parts.length ? parts[parts.length - 1] : undefined;
};

function makeLeaf(nl: NewLeaf): Leaf {
  const leaf: Leaf = {
    type: 'pane',
    id: newLeafId(),
    kind: nl.kind,
    title: nl.title ?? nl.provider ?? basename(nl.filePath) ?? 'Pane',
  };
  if (nl.threadAgent) leaf.threadAgent = true;
  if (nl.sessionId !== undefined) leaf.sessionId = nl.sessionId;
  if (nl.projectId !== undefined) leaf.projectId = nl.projectId;
  if (nl.provider !== undefined) leaf.provider = nl.provider;
  if (nl.filePath !== undefined) leaf.filePath = nl.filePath;
  if (nl.threadId !== undefined) leaf.threadId = nl.threadId;
  if (nl.workspaceId !== undefined) leaf.workspaceId = nl.workspaceId;
  if (nl.root !== undefined) leaf.root = nl.root;
  if (nl.readOnly !== undefined) leaf.readOnly = nl.readOnly;
  if (nl.sourceId !== undefined) leaf.sourceId = nl.sourceId;
  if (nl.tableId !== undefined) leaf.tableId = nl.tableId;
  if (nl.viewId !== undefined) leaf.viewId = nl.viewId;
  if (nl.nodeToken !== undefined) leaf.nodeToken = nl.nodeToken;
  return leaf;
}

// Pastikan tidak ada Split dengan anak null.
function normalizeNode(n: Node | null): Node | null {
  if (!n) return null;
  if (n.type === 'pane') return n;
  const a = normalizeNode(n.a);
  const b = normalizeNode(n.b);
  if (!a) return b;
  if (!b) return a;
  return { ...n, a, b };
}

function normalize(doc: LayoutDoc): LayoutDoc {
  const root = normalizeNode(doc.root);
  const ids = new Set(leaves(root).map((l) => l.id));
  const out: LayoutDoc = { version: 1, root };
  if (doc.focusId && ids.has(doc.focusId)) out.focusId = doc.focusId;
  if (doc.maximizedId && ids.has(doc.maximizedId)) out.maximizedId = doc.maximizedId;
  else if (doc.maximizedId === null) out.maximizedId = null;
  return out;
}

export function setFocus(doc: LayoutDoc, paneId: string): LayoutDoc {
  if (!findLeaf(doc, paneId)) return doc;
  return { ...doc, focusId: paneId };
}

export function splitPane(
  doc: LayoutDoc,
  paneId: string,
  dir: 'h' | 'v',
  newLeaf: NewLeaf | Leaf,
  side: 'after' | 'before' = 'after',
): { doc: LayoutDoc; id: string } {
  const target = pathOf(doc, paneId);
  const targetNode = findLeaf(doc, paneId);
  const leaf: Leaf = (newLeaf as Leaf).type === 'pane' && (newLeaf as Leaf).id ? (newLeaf as Leaf) : makeLeaf(newLeaf as NewLeaf);
  if (target === null || !targetNode) {
    return { doc, id: '' };
  }
  const split: Split = {
    type: 'split',
    dir,
    ratio: 0.5,
    a: side === 'after' ? targetNode : leaf,
    b: side === 'after' ? leaf : targetNode,
  };
  const root = replaceAt(doc.root, target, split);
  return { doc: { ...doc, root, focusId: leaf.id }, id: leaf.id };
}

export function openPane(
  doc: LayoutDoc,
  nl: NewLeaf,
  opts?: { dir?: 'h' | 'v' },
): { doc: LayoutDoc; id: string } {
  const existing = findLeafBy(doc, (l) => {
    if (nl.sessionId !== undefined && l.sessionId === nl.sessionId) return true;
    if (nl.filePath !== undefined && l.filePath === nl.filePath && l.projectId === nl.projectId) return true;
    if (nl.threadId !== undefined && l.threadId === nl.threadId) return true;
    if (nl.kind === 'lark-board' && l.kind === 'lark-board' && nl.sourceId !== undefined && l.sourceId === nl.sourceId) return true;
    if (nl.kind === 'lark-doc' && l.kind === 'lark-doc' && nl.sourceId !== undefined && l.sourceId === nl.sourceId && (l.nodeToken ?? '') === (nl.nodeToken ?? '')) return true;
    return false;
  });
  if (existing) return { doc: setFocus(doc, existing.id), id: existing.id };
  if (!doc.root) {
    const leaf = makeLeaf(nl);
    return { doc: { ...doc, root: leaf, focusId: leaf.id }, id: leaf.id };
  }
  const all = leaves(doc);
  const focus = doc.focusId && findLeaf(doc, doc.focusId) ? doc.focusId : all[all.length - 1].id;
  return splitPane(doc, focus, opts?.dir ?? 'h', nl, 'after');
}

// Cabut leaf dari pohon tanpa mengubah fokus; kembalikan akar baru.
function removeLeaf(doc: LayoutDoc, paneId: string): Node | null {
  const path = pathOf(doc, paneId);
  if (path === null) return doc.root;
  if (path === '') return null;
  const parts = segs(path);
  const parentPath = parts.slice(0, -1).join('.');
  const side = parts[parts.length - 1] as 'a' | 'b';
  const parent = getAt(doc, parentPath);
  if (!parent || parent.type !== 'split') return doc.root;
  const sibling = parent[side === 'a' ? 'b' : 'a'];
  return replaceAt(doc.root, parentPath, sibling);
}

export function closePane(doc: LayoutDoc, paneId: string): LayoutDoc {
  const path = pathOf(doc, paneId);
  if (path === null) return doc;
  let nextFocus = doc.focusId;
  if (path !== '') {
    const parts = segs(path);
    const parentPath = parts.slice(0, -1).join('.');
    const side = parts[parts.length - 1];
    const parent = getAt(doc, parentPath) as Split;
    const sibling = parent[side === 'a' ? 'b' : 'a'];
    if (doc.focusId === paneId || !doc.focusId) nextFocus = leaves(sibling)[0]?.id;
  } else {
    nextFocus = undefined;
  }
  const root = removeLeaf(doc, paneId);
  const out: LayoutDoc = { ...doc, root };
  if (nextFocus) out.focusId = nextFocus;
  else delete out.focusId;
  if (out.maximizedId === paneId) out.maximizedId = null;
  return normalize(out);
}

export function swapPanes(doc: LayoutDoc, aId: string, bId: string): LayoutDoc {
  if (aId === bId) return doc;
  const la = findLeaf(doc, aId);
  const lb = findLeaf(doc, bId);
  if (!la || !lb) return doc;
  const swap = (n: Node): Node => {
    if (n.type === 'pane') return n.id === aId ? lb : n.id === bId ? la : n;
    return { ...n, a: swap(n.a), b: swap(n.b) };
  };
  return { ...doc, root: doc.root ? swap(doc.root) : null };
}

export function restore(doc: LayoutDoc): LayoutDoc {
  return { ...doc, maximizedId: null };
}

export function maximize(doc: LayoutDoc, paneId: string): LayoutDoc {
  if (!findLeaf(doc, paneId)) return doc;
  return { ...doc, maximizedId: paneId };
}

export function movePane(doc: LayoutDoc, paneId: string, targetId: string, zone: DropZone): LayoutDoc {
  if (paneId === targetId) return doc;
  const leaf = findLeaf(doc, paneId);
  if (!leaf || !findLeaf(doc, targetId)) return doc;
  let d = doc.maximizedId ? restore(doc) : doc;
  if (zone === 'center') return normalize(swapPanes(d, paneId, targetId));
  const root = removeLeaf(d, paneId);
  const stripped: LayoutDoc = { ...d, root };
  const dir: 'h' | 'v' = zone === 'top' || zone === 'bottom' ? 'v' : 'h';
  const side: 'after' | 'before' = zone === 'top' || zone === 'left' ? 'before' : 'after';
  const res = splitPane(stripped, targetId, dir, leaf, side);
  return normalize({ ...res.doc, focusId: leaf.id });
}

export function resize(doc: LayoutDoc, path: SplitPath, ratio: number): LayoutDoc {
  const n = getAt(doc, path);
  if (!n || n.type !== 'split') return doc;
  return { ...doc, root: replaceAt(doc.root, path, { ...n, ratio: clampRatio(ratio) }) };
}

export function tidyGrid(doc: LayoutDoc): LayoutDoc {
  const all = leaves(doc);
  const n = all.length;
  const out: LayoutDoc = { version: 1, root: null };
  if (doc.focusId && all.some((l) => l.id === doc.focusId)) out.focusId = doc.focusId;
  out.maximizedId = null;
  if (n === 0) return out;
  const cols = Math.ceil(Math.sqrt(n));
  const rows: Leaf[][] = [];
  for (let i = 0; i < n; i += cols) rows.push(all.slice(i, i + cols));
  const chain = (items: Node[], dir: 'h' | 'v'): Node => {
    const build = (i: number): Node => {
      if (i === items.length - 1) return items[i];
      return { type: 'split', dir, ratio: 1 / (items.length - i), a: items[i], b: build(i + 1) };
    };
    return build(0);
  };
  const rowNodes = rows.map((r) => chain(r, 'h'));
  out.root = chain(rowNodes, 'v');
  return out;
}

// Buka agent thread di satu pane khusus: bila pane itu sudah ada, ganti sesinya; bila belum, buat.
export function openThreadAgent(doc: LayoutDoc, nl: NewLeaf): { doc: LayoutDoc; id: string } {
  const sameSession = findLeafBy(doc, (l) => l.sessionId !== undefined && l.sessionId === nl.sessionId);
  if (sameSession) return { doc: setFocus(doc, sameSession.id), id: sameSession.id };
  const slot = findLeafBy(doc, (l) => !!l.threadAgent);
  if (!slot) return openPane(doc, { ...nl, threadAgent: true });
  const rec = (n: Node): Node =>
    n.type === 'pane'
      ? n.id === slot.id
        ? {
            ...n,
            sessionId: nl.sessionId,
            projectId: nl.projectId,
            provider: nl.provider,
            workspaceId: nl.workspaceId,
            title: nl.title ?? n.title,
          }
        : n
      : { ...n, a: rec(n.a), b: rec(n.b) };
  return { doc: setFocus({ ...doc, root: doc.root ? rec(doc.root) : null }, slot.id), id: slot.id };
}

export function renamePane(doc: LayoutDoc, paneId: string, title: string): LayoutDoc {
  const t = (title ?? '').trim();
  if (!t || !findLeaf(doc, paneId)) return doc;
  const rec = (n: Node): Node =>
    n.type === 'pane' ? (n.id === paneId ? { ...n, title: t } : n) : { ...n, a: rec(n.a), b: rec(n.b) };
  return { ...doc, root: doc.root ? rec(doc.root) : null };
}

const isStr = (v: unknown): v is string => typeof v === 'string';

function hydrateNode(raw: unknown): Node | null {
  if (!raw || typeof raw !== 'object') return null;
  const r = raw as Record<string, unknown>;
  if (r.type === 'pane') {
    if (!isStr(r.id) || !r.id || !isStr(r.kind)) return null;
    const leaf: Leaf = {
      type: 'pane',
      id: r.id,
      kind: r.kind as PaneKind,
      title: isStr(r.title) && r.title ? r.title : 'Pane',
    };
    for (const k of ['sessionId', 'projectId', 'provider', 'filePath', 'workspaceId', 'root', 'threadId', 'sourceId', 'tableId', 'viewId', 'nodeToken'] as const) {
      if (isStr(r[k])) leaf[k] = r[k] as string;
    }
    if (typeof r.readOnly === 'boolean') leaf.readOnly = r.readOnly;
    if (typeof r.threadAgent === 'boolean') leaf.threadAgent = r.threadAgent;
    return leaf;
  }
  if (r.type === 'split') {
    if (r.dir !== 'h' && r.dir !== 'v') return null;
    if (typeof r.ratio !== 'number') return null;
    const a = hydrateNode(r.a);
    const b = hydrateNode(r.b);
    if (!a) return b;
    if (!b) return a;
    return { type: 'split', dir: r.dir, ratio: clampRatio(r.ratio), a, b };
  }
  return null;
}

export function hydrateDoc(raw: unknown): LayoutDoc {
  try {
    if (!raw || typeof raw !== 'object') return emptyDoc();
    const r = raw as Record<string, unknown>;
    if (r.version !== 1) return emptyDoc();
    const root = hydrateNode(r.root);
    const seen = new Set<string>();
    // buang duplikat id agar pohon konsisten
    const dedupe = (n: Node | null): Node | null => {
      if (!n) return null;
      if (n.type === 'pane') {
        if (seen.has(n.id)) return null;
        seen.add(n.id);
        return n;
      }
      const a = dedupe(n.a);
      const b = dedupe(n.b);
      if (!a) return b;
      if (!b) return a;
      return { ...n, a, b };
    };
    const clean = dedupe(root);
    const doc: LayoutDoc = { version: 1, root: clean };
    if (isStr(r.focusId)) doc.focusId = r.focusId;
    if (isStr(r.maximizedId)) doc.maximizedId = r.maximizedId;
    else if (r.maximizedId === null) doc.maximizedId = null;
    return JSON.parse(JSON.stringify(normalize(doc)));
  } catch {
    return emptyDoc();
  }
}
