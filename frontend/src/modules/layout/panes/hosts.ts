import { mount, unmount } from 'svelte';
import type { Leaf, Tab } from '../model';
import { getPaneKind } from '../registry';
import PaneHostRoot from './PaneHostRoot.svelte';

interface Entry {
  host: HTMLElement;
  kind: string;
  instance: { update(pane: Tab, paneId: string, visible: boolean): void } | null;
  unmountFn: (() => void) | null;
  tab: Tab;
  paneId: string;
  visible: boolean;
}

const entries = new Map<string, Entry>();
const MUTED = 'padding:0.75rem;color:var(--fg-muted,#888)';

function showText(entry: Entry, text: string): void {
  entry.host.textContent = text;
  entry.host.style.cssText += ';' + MUTED;
}

function resetHostStyle(entry: Entry): void {
  entry.host.style.cssText = 'width:100%;height:100%;min-height:0;display:flex;flex-direction:column';
}

function mountInto(entry: Entry): void {
  const spec = getPaneKind(entry.kind);
  entry.instance = null;
  entry.unmountFn = null;
  if (!spec) {
    showText(entry, `Pane tidak dikenal: ${entry.kind}`);
    return;
  }
  try {
    const inst = mount(PaneHostRoot, {
      target: entry.host,
      props: { component: spec.component, pane: entry.tab, paneId: entry.paneId, visible: entry.visible },
    }) as unknown as { update(pane: Tab, paneId: string, visible: boolean): void };
    entry.instance = inst;
    entry.unmountFn = () => {
      try { unmount(inst as unknown as Record<string, unknown>); } catch { /* sudah lepas */ }
    };
  } catch (e) {
    showText(entry, 'Gagal memuat pane: ' + (e instanceof Error ? e.message : String(e)));
  }
}

function updateEntry(entry: Entry, tab: Tab, paneId: string, visible: boolean): void {
  entry.tab = tab;
  entry.paneId = paneId;
  entry.visible = visible;
  if (tab.kind !== entry.kind) {
    entry.unmountFn?.();
    entry.host.textContent = '';
    resetHostStyle(entry);
    entry.kind = tab.kind;
    mountInto(entry);
    return;
  }
  if (!entry.instance) {
    if (getPaneKind(entry.kind)) {
      entry.host.textContent = '';
      resetHostStyle(entry);
      mountInto(entry);
    }
    return;
  }
  try {
    entry.instance.update(tab, paneId, visible);
  } catch { /* abaikan */ }
}

export function ensureHost(tab: Tab, paneId: string, visible: boolean): HTMLElement {
  const existing = entries.get(tab.id);
  if (existing) {
    updateEntry(existing, tab, paneId, visible);
    return existing.host;
  }
  const host = document.createElement('div');
  host.className = 'pane-host';
  host.style.cssText = 'width:100%;height:100%;min-height:0;display:flex;flex-direction:column';
  const entry: Entry = { host, kind: tab.kind, instance: null, unmountFn: null, tab, paneId, visible };
  entries.set(tab.id, entry);
  mountInto(entry);
  return host;
}

export function getHost(tabId: string): HTMLElement | null {
  return entries.get(tabId)?.host ?? null;
}

export function attachHost(tabId: string, slot: HTMLElement): void {
  const e = entries.get(tabId);
  if (e && e.host.parentElement !== slot) slot.appendChild(e.host);
}

// Lepas dari DOM, TIDAK unmount.
export function detachHost(tabId: string): void {
  entries.get(tabId)?.host.remove();
}

export function releaseHost(tabId: string): void {
  const e = entries.get(tabId);
  if (!e) return;
  e.unmountFn?.();
  e.host.remove();
  entries.delete(tabId);
}

export function syncPanes(
  list: { pane: Leaf; tab: Tab }[],
  visibleTabIds: ReadonlySet<string>,
): void {
  const ids = new Set<string>();
  for (const { pane, tab } of list) {
    ids.add(tab.id);
    ensureHost(tab, pane.id, visibleTabIds.has(tab.id));
  }
  for (const id of [...entries.keys()]) if (!ids.has(id)) releaseHost(id);
}
