// Hidrasi layout tersimpan: terima bentuk leaf lama (konten langsung) dan baru (tabs).
import { emptyDoc, clampRatio, normalize, newPaneId, newTabId } from '../layout/model';
import type { LayoutDoc, Leaf, Node, PaneKind, Tab } from '../layout/model';

const isStr = (v: unknown): v is string => typeof v === 'string';
const isObj = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);

const STR_KEYS = ['sessionId', 'projectId', 'provider', 'filePath', 'workspaceId', 'root', 'threadId', 'sourceId', 'tableId', 'viewId', 'nodeToken', 'pipelineId', 'reviewSessionId'] as const;
const MAX_DEPTH = 64;

// Tab dari objek tab (bentuk baru) atau objek leaf lama (id sengaja dikosongkan oleh pemanggil).
function hydrateTab(raw: unknown, usedTabIds: Set<string>): Tab | null {
  if (!isObj(raw)) return null;
  if (!isStr(raw.kind) || !raw.kind) return null;
  let id = isStr(raw.id) && raw.id.startsWith('t_') && !usedTabIds.has(raw.id) ? raw.id : newTabId();
  while (usedTabIds.has(id)) id = newTabId();
  usedTabIds.add(id);
  const tab: Tab = { id, kind: raw.kind as PaneKind, title: isStr(raw.title) && raw.title ? raw.title : 'Pane' };
  for (const k of STR_KEYS) {
    if (isStr(raw[k])) tab[k] = raw[k] as string;
  }
  if (typeof raw.readOnly === 'boolean') tab.readOnly = raw.readOnly;
  if (typeof raw.threadAgent === 'boolean') tab.threadAgent = raw.threadAgent;
  if (typeof raw.number === 'number' && Number.isFinite(raw.number)) tab.number = raw.number;
  return tab;
}

function hydrateLeaf(raw: Record<string, unknown>, usedPaneIds: Set<string>, usedTabIds: Set<string>): Leaf | null {
  let tabs: Tab[];
  let activeTabId: string;
  if (Array.isArray(raw.tabs)) {
    tabs = [];
    for (const item of raw.tabs) {
      const t = hydrateTab(item, usedTabIds);
      if (t) tabs.push(t);
    }
    if (!tabs.length) return null;
    activeTabId = isStr(raw.activeTabId) && tabs.some((t) => t.id === raw.activeTabId) ? raw.activeTabId : tabs[0].id;
  } else {
    const t = hydrateTab({ ...raw, id: undefined }, usedTabIds);
    if (!t) return null;
    tabs = [t];
    activeTabId = t.id;
  }
  let id = isStr(raw.id) && raw.id && !usedPaneIds.has(raw.id) ? raw.id : newPaneId();
  while (usedPaneIds.has(id)) id = newPaneId();
  usedPaneIds.add(id);
  return { type: 'pane', id, tabs, activeTabId };
}

function hydrateNode(raw: unknown, usedPaneIds: Set<string>, usedTabIds: Set<string>, depth: number): Node | null {
  if (!isObj(raw) || depth > MAX_DEPTH) return null;
  if (raw.type === 'pane') return hydrateLeaf(raw, usedPaneIds, usedTabIds);
  if (raw.type === 'split') {
    if (raw.dir !== 'h' && raw.dir !== 'v') return null;
    if (typeof raw.ratio !== 'number') return null;
    const a = hydrateNode(raw.a, usedPaneIds, usedTabIds, depth + 1);
    const b = hydrateNode(raw.b, usedPaneIds, usedTabIds, depth + 1);
    if (!a) return b;
    if (!b) return a;
    return { type: 'split', dir: raw.dir, ratio: clampRatio(raw.ratio), a, b };
  }
  return null;
}

function collectPaneIds(n: Node | null): string[] {
  if (!n) return [];
  if (n.type === 'pane') return [n.id];
  return [...collectPaneIds(n.a), ...collectPaneIds(n.b)];
}

export function hydrateDoc(raw: unknown): LayoutDoc {
  try {
    if (!isObj(raw)) return emptyDoc();
    if (raw.version !== 1) return emptyDoc();
    const root = hydrateNode(raw.root, new Set(), new Set(), 0);
    const doc: LayoutDoc = { version: 1, root };
    if (isStr(raw.focusId)) doc.focusId = raw.focusId;
    if (isStr(raw.maximizedId)) doc.maximizedId = raw.maximizedId;
    else if (raw.maximizedId === null) doc.maximizedId = null;
    const out = normalize(doc);
    const ids = new Set(collectPaneIds(out.root));
    if (out.focusId && !ids.has(out.focusId)) delete out.focusId;
    if (out.maximizedId && !ids.has(out.maximizedId)) delete out.maximizedId;
    return JSON.parse(JSON.stringify(out));
  } catch {
    return emptyDoc();
  }
}
