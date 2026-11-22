// Model pohon pane: tipe dan fungsi murni (tanpa IPC, tanpa mutasi masukan).

export type PaneKind = 'terminal' | 'file' | 'thread' | 'lark-board' | 'lark-doc' | 'git-mr' | 'git-pipeline'; // string terbuka lewat registerPaneKind
export interface Tab {
  id: string; // prefix 't_'
  kind: PaneKind;
  sessionId?: string;
  projectId?: string;
  provider?: string;
  filePath?: string;
  threadId?: string;
  workspaceId?: string; // untuk tab file
  root?: string; // root eksplisit untuk tab file read-only (mis. direktori grimoire)
  readOnly?: boolean;
  // Satu-satunya tab terminal untuk agent thread; membuka agent lain mengganti sesinya.
  threadAgent?: boolean;
  sourceId?: string;
  tableId?: string;
  viewId?: string;
  nodeToken?: string;
  number?: number; // nomor MR/PR
  pipelineId?: string;
  title: string;
}
export interface Leaf {
  type: 'pane';
  id: string; // prefix 'p_'
  tabs: Tab[]; // tidak pernah kosong
  activeTabId: string; // selalu menunjuk tab di tabs
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
  number?: number; // nomor MR/PR
  pipelineId?: string;
}

export const emptyDoc = (): LayoutDoc => ({ version: 1, root: null });

export const clampRatio = (r: number): number => {
  if (typeof r !== 'number' || Number.isNaN(r)) return 0.5;
  return Math.min(0.9, Math.max(0.1, r));
};

let counter = 0;
function newId(prefix: string): string {
  let hex = '';
  try {
    hex = crypto.randomUUID().replace(/-/g, '').slice(0, 10);
  } catch {
    counter += 1;
    hex = (Date.now().toString(16) + counter.toString(16).padStart(4, '0')).slice(-10).padStart(8, '0');
  }
  return `${prefix}${hex}`;
}
export const newPaneId = (): string => newId('p_');
export const newTabId = (): string => newId('t_');

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

export const activeTab = (leaf: Leaf): Tab => leaf.tabs.find((t) => t.id === leaf.activeTabId) ?? leaf.tabs[0];

export const allTabs = (doc: LayoutDoc): { pane: Leaf; tab: Tab }[] =>
  leaves(doc).flatMap((pane) => pane.tabs.map((tab) => ({ pane, tab })));

export const findTab = (
  doc: LayoutDoc,
  pred: (tab: Tab, pane: Leaf) => boolean,
): { pane: Leaf; tab: Tab } | null => allTabs(doc).find((e) => pred(e.tab, e.pane)) ?? null;

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

export function makeLeaf(nl: NewLeaf): Tab {
  const leaf: Tab = {
    id: newTabId(),
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
  if (nl.number !== undefined) leaf.number = nl.number;
  if (nl.pipelineId !== undefined) leaf.pipelineId = nl.pipelineId;
  return leaf;
}

// Bungkus satu tab menjadi pane baru.
function wrapTab(tab: Tab): Leaf {
  return { type: 'pane', id: newPaneId(), tabs: [tab], activeTabId: tab.id };
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

export function normalize(doc: LayoutDoc): LayoutDoc {
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

// Ganti leaf dengan id paneId memakai hasil fn; node lain dipakai ulang.
function mapLeaf(doc: LayoutDoc, paneId: string, fn: (l: Leaf) => Leaf): LayoutDoc {
  if (!findLeaf(doc, paneId)) return doc;
  const rec = (n: Node): Node => (n.type === 'pane' ? (n.id === paneId ? fn(n) : n) : { ...n, a: rec(n.a), b: rec(n.b) });
  return { ...doc, root: doc.root ? rec(doc.root) : null };
}

// Aturan dedupe: apakah tab sudah memuat konten yang diminta.
function sameContent(nl: NewLeaf, t: Tab): boolean {
  if (nl.sessionId !== undefined && t.sessionId === nl.sessionId) return true;
  if (nl.filePath !== undefined && t.filePath === nl.filePath && t.projectId === nl.projectId) return true;
  if (nl.threadId !== undefined && t.threadId === nl.threadId) return true;
  if (nl.kind === 'lark-board' && t.kind === 'lark-board' && nl.sourceId !== undefined && t.sourceId === nl.sourceId) return true;
  if (
    nl.kind === 'lark-doc' &&
    t.kind === 'lark-doc' &&
    nl.sourceId !== undefined &&
    t.sourceId === nl.sourceId &&
    (t.nodeToken ?? '') === (nl.nodeToken ?? '')
  )
    return true;
  return false;
}

function focusPaneId(doc: LayoutDoc): string | null {
  if (doc.focusId && findLeaf(doc, doc.focusId)) return doc.focusId;
  const all = leaves(doc);
  return all.length ? all[all.length - 1].id : null;
}

function revealTab(doc: LayoutDoc, paneId: string, tabId: string): LayoutDoc {
  return setFocus(activateTab(doc, paneId, tabId), paneId);
}

export function activateTab(doc: LayoutDoc, paneId: string, tabId: string): LayoutDoc {
  const pane = findLeaf(doc, paneId);
  if (!pane || !pane.tabs.some((t) => t.id === tabId) || pane.activeTabId === tabId) return doc;
  return mapLeaf(doc, paneId, (l) => ({ ...l, activeTabId: tabId }));
}

export function renameTab(doc: LayoutDoc, paneId: string, tabId: string, title: string): LayoutDoc {
  const t = (title ?? '').trim();
  const tab = findLeaf(doc, paneId)?.tabs.find((x) => x.id === tabId);
  if (!t || !tab || tab.title === t) return doc;
  return mapLeaf(doc, paneId, (l) => ({ ...l, tabs: l.tabs.map((x) => (x.id === tabId ? { ...x, title: t } : x)) }));
}

export function closeTab(doc: LayoutDoc, paneId: string, tabId: string): LayoutDoc {
  const pane = findLeaf(doc, paneId);
  if (!pane || !pane.tabs.some((t) => t.id === tabId)) return doc;
  if (pane.tabs.length === 1) return closePane(doc, paneId);
  const idx = pane.tabs.findIndex((t) => t.id === tabId);
  const tabs = pane.tabs.filter((t) => t.id !== tabId);
  const activeTabId = pane.activeTabId !== tabId ? pane.activeTabId : (tabs[idx] ?? tabs[idx - 1]).id;
  return mapLeaf(doc, paneId, (l) => ({ ...l, tabs, activeTabId }));
}

// Split dengan leaf utuh (dipakai splitPane dan movePane).
function splitWith(
  doc: LayoutDoc,
  paneId: string,
  dir: 'h' | 'v',
  leaf: Leaf,
  side: 'after' | 'before',
): { doc: LayoutDoc; id: string } {
  const target = pathOf(doc, paneId);
  const targetNode = findLeaf(doc, paneId);
  if (target === null || !targetNode) return { doc, id: '' };
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

export function splitPane(
  doc: LayoutDoc,
  paneId: string,
  dir: 'h' | 'v',
  content: NewLeaf | Leaf,
  side: 'after' | 'before' = 'after',
): { doc: LayoutDoc; id: string } {
  const target = findLeaf(doc, paneId);
  if (!target) return { doc, id: '' };
  if ('tabs' in content) return splitWith(doc, paneId, dir, content, side);
  // Cmd+D hanya berlaku pada tab terminal aktif.
  if (activeTab(target).kind !== 'terminal') return { doc, id: '' };
  return splitWith(doc, paneId, dir, wrapTab(makeLeaf(content)), side);
}

export function openPane(doc: LayoutDoc, nl: NewLeaf): LayoutDoc {
  const hit = findTab(doc, (t) => sameContent(nl, t));
  if (hit) return revealTab(doc, hit.pane.id, hit.tab.id);
  const tab = makeLeaf(nl);
  const pid = doc.root ? focusPaneId(doc) : null;
  if (pid === null) {
    const pane = wrapTab(tab);
    return { ...doc, root: pane, focusId: pane.id };
  }
  const next = mapLeaf(doc, pid, (l) => {
    const at = l.tabs.findIndex((x) => x.id === l.activeTabId);
    const i = at < 0 ? l.tabs.length : at + 1;
    return { ...l, tabs: [...l.tabs.slice(0, i), tab, ...l.tabs.slice(i)], activeTabId: tab.id };
  });
  return { ...next, focusId: pid };
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
  const res = splitWith(stripped, targetId, dir, leaf, side);
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

// Buka agent thread di satu tab khusus: bila tab itu sudah ada, ganti sesinya; bila belum, buat.
export function openThreadAgent(doc: LayoutDoc, nl: NewLeaf): { doc: LayoutDoc; id: string } {
  const sameSession = nl.sessionId !== undefined ? findTab(doc, (t) => t.sessionId === nl.sessionId) : null;
  if (sameSession) return { doc: revealTab(doc, sameSession.pane.id, sameSession.tab.id), id: sameSession.pane.id };
  const slot = findTab(doc, (t) => !!t.threadAgent);
  if (slot) {
    const swapped = mapLeaf(doc, slot.pane.id, (l) => ({
      ...l,
      tabs: l.tabs.map((t) =>
        t.id === slot.tab.id
          ? {
              ...t,
              sessionId: nl.sessionId,
              projectId: nl.projectId,
              provider: nl.provider,
              workspaceId: nl.workspaceId,
              title: nl.title ?? t.title,
            }
          : t,
      ),
    }));
    return { doc: revealTab(swapped, slot.pane.id, slot.tab.id), id: slot.pane.id };
  }
  const next = openPane(doc, { ...nl, threadAgent: true });
  return { doc: next, id: next.focusId ?? '' };
}

export { hydrateDoc } from '../persist/hydrate';
