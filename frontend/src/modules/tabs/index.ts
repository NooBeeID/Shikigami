// Shim v1 -> v2: menjembatani store `tabs`/`activeTabId` dengan store `layout` (dua arah).
import { get } from 'svelte/store';
import { tabs, activeTabId, closeTab, sessions } from '../../lib/stores';
import type { Tab } from '../../lib/stores';
import { layout } from '../../lib/stores/layout';
import { allTabs, findTab, activeTab, findLeaf } from '../layout/model';
import type { Tab as LayoutTab, Leaf, NewLeaf, LayoutDoc } from '../layout/model';

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

function tabIdOfLayoutTab(t: LayoutTab): string | null {
  if (t.kind === 'terminal' && t.sessionId) return `session:${t.sessionId}`;
  if (t.kind === 'file' && t.filePath !== undefined) {
    return `file:${JSON.stringify({ path: t.filePath, projectId: t.projectId, workspaceId: t.workspaceId })}`;
  }
  return null;
}

function layoutTabOf(tab: Tab, doc: LayoutDoc): { pane: Leaf; tab: LayoutTab } | null {
  return findTab(doc, (t) =>
    tab.type === 'session'
      ? t.kind === 'terminal' && t.sessionId === tab.refId
      : t.kind === 'file' && tabIdOfLayoutTab(t) === tab.id,
  );
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
  const r = layoutTabOf(tab, get(layout));
  if (r) {
    layout.activateTab(r.pane.id, r.tab.id);
    return;
  }
  const nl = newLeafOfTab(tab);
  if (nl) layout.openPane(nl);
}

function focusedTabId(doc: LayoutDoc): string | null {
  const f = doc.focusId ? findLeaf(doc, doc.focusId) : null;
  return f ? tabIdOfLayoutTab(activeTab(f)) : null;
}

// tab -> pane
tabs.subscribe((list) => {
  if (syncing) return;
  const ids = new Set(list.map((t) => t.id));
  run(() => {
    // tab hilang tetapi pane masih ada
    const toClose: { paneId: string; tabId: string; tid: string }[] = [];
    for (const { pane, tab } of allTabs(get(layout))) {
      const tid = tabIdOfLayoutTab(tab);
      if (tid && known.has(tid) && !ids.has(tid)) toClose.push({ paneId: pane.id, tabId: tab.id, tid });
    }
    for (const c of toClose) {
      layout.closeTab(c.paneId, c.tabId);
      known.delete(c.tid);
    }
    // tab baru tanpa pane
    const active = get(activeTabId);
    for (const t of list) {
      if (layoutTabOf(t, get(layout))) continue;
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
  for (const { tab } of allTabs(doc)) {
    const tid = tabIdOfLayoutTab(tab);
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
