// Shim v1 -> v2: menjembatani store `tabs`/`activeTabId` dengan store `layout` (dua arah).
import { get } from 'svelte/store';
import { tabs, activeTabId, closeTab, sessions } from '../../lib/stores';
import type { Tab } from '../../lib/stores';
import { layout } from '../../lib/stores/layout';
import { leaves } from '../layout/model';
import type { Leaf, NewLeaf, LayoutDoc } from '../layout/model';

let syncing = false;
let known = new Set<string>(); // tabId yang pernah punya pane

function run(fn: () => void): void {
  const prev = syncing;
  syncing = true;
  try {
    fn();
  } finally {
    syncing = prev;
  }
}

function tabIdOfLeaf(l: Leaf): string | null {
  if (l.kind === 'terminal' && l.sessionId) return `session:${l.sessionId}`;
  if (l.kind === 'file' && l.filePath !== undefined) {
    return `file:${JSON.stringify({ path: l.filePath, projectId: l.projectId, workspaceId: l.workspaceId })}`;
  }
  return null;
}

function leafOfTab(tab: Tab, doc: LayoutDoc): Leaf | null {
  for (const l of leaves(doc)) {
    if (tab.type === 'session') {
      if (l.kind === 'terminal' && l.sessionId === tab.refId) return l;
    } else if (l.kind === 'file' && tabIdOfLeaf(l) === tab.id) {
      return l;
    }
  }
  return null;
}

function newLeafOfTab(tab: Tab): NewLeaf | null {
  if (tab.type === 'session') {
    const s = get(sessions).find((x) => x.id === tab.refId);
    return {
      kind: 'terminal',
      sessionId: tab.refId,
      projectId: s?.projectId,
      provider: s?.providerId,
      title: tab.title,
    };
  }
  if (tab.type === 'file') {
    try {
      const d = JSON.parse(tab.refId) as { path: string; projectId?: string; workspaceId?: string };
      if (typeof d.path !== 'string') return null;
      return { kind: 'file', filePath: d.path, projectId: d.projectId, workspaceId: d.workspaceId, title: tab.title };
    } catch {
      return null;
    }
  }
  return null;
}

function ensurePane(tab: Tab): void {
  const leaf = leafOfTab(tab, get(layout));
  if (leaf) {
    layout.setFocus(leaf.id);
    return;
  }
  const nl = newLeafOfTab(tab);
  if (nl) layout.openPane(nl);
}

function focusedTabId(doc: LayoutDoc): string | null {
  const f = doc.focusId ? leaves(doc).find((l) => l.id === doc.focusId) : undefined;
  return f ? tabIdOfLeaf(f) : null;
}

// tab -> pane
tabs.subscribe((list) => {
  if (syncing) return;
  const ids = new Set(list.map((t) => t.id));
  run(() => {
    // tab hilang tetapi pane masih ada
    const doc = get(layout);
    for (const l of leaves(doc)) {
      const tid = tabIdOfLeaf(l);
      if (tid && known.has(tid) && !ids.has(tid)) {
        layout.closePane(l.id);
        known.delete(tid);
      }
    }
    // tab baru tanpa pane
    const active = get(activeTabId);
    for (const t of list) {
      if (leafOfTab(t, get(layout))) continue;
      const nl = newLeafOfTab(t);
      if (!nl) continue;
      const id = layout.openPane(nl);
      if (t.id === active) layout.setFocus(id);
    }
  });
});

// fokus tab -> fokus pane
activeTabId.subscribe((id) => {
  if (syncing || !id) return;
  const tab = get(tabs).find((t) => t.id === id);
  if (!tab) return;
  run(() => ensurePane(tab));
});

// pane -> tab (fokus dan penutupan)
layout.subscribe((doc) => {
  const tabIds = new Set(get(tabs).map((t) => t.id));
  const present = new Set<string>();
  for (const l of leaves(doc)) {
    const tid = tabIdOfLeaf(l);
    if (tid) present.add(tid);
  }
  if (!syncing) {
    const closed: string[] = [];
    for (const tid of known) if (!present.has(tid) && tabIds.has(tid)) closed.push(tid);
    const tid = focusedTabId(doc);
    if (closed.length) {
      run(() => {
        for (const c of closed) closeTab(c);
        const cur = focusedTabId(get(layout));
        activeTabId.set(cur && get(tabs).some((t) => t.id === cur) ? cur : null);
      });
    } else if (tid && tabIds.has(tid) && get(activeTabId) !== tid) {
      run(() => activeTabId.set(tid));
    }
  }
  known = new Set([...present].filter((t) => get(tabs).some((x) => x.id === t)));
});
