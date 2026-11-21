import { mount, unmount } from 'svelte';
import type { Leaf } from '../model';
import { getPaneKind } from '../registry';
import PaneHostRoot from './PaneHostRoot.svelte';

interface Entry {
  host: HTMLElement;
  kind: string;
  instance: { update(pane: Leaf, visible: boolean): void } | null;
  unmountFn: (() => void) | null;
  pane: Leaf;
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
    showText(entry, 'Jenis pane tidak dikenal');
    return;
  }
  try {
    const inst = mount(PaneHostRoot, {
      target: entry.host,
      props: { component: spec.component, pane: entry.pane, visible: entry.visible },
    }) as unknown as { update(pane: Leaf, visible: boolean): void };
    entry.instance = inst;
    entry.unmountFn = () => {
      try { unmount(inst as unknown as Record<string, unknown>); } catch { /* sudah lepas */ }
    };
  } catch (e) {
    showText(entry, 'Gagal memuat pane: ' + (e instanceof Error ? e.message : String(e)));
  }
}

function updateEntry(entry: Entry, leaf: Leaf, visible: boolean): void {
  entry.pane = leaf;
  entry.visible = visible;
  if (leaf.kind !== entry.kind) {
    entry.unmountFn?.();
    entry.host.textContent = '';
    resetHostStyle(entry);
    entry.kind = leaf.kind;
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
    entry.instance.update(leaf, visible);
  } catch { /* abaikan */ }
}

export function ensureHost(leaf: Leaf, visible: boolean): HTMLElement {
  const existing = entries.get(leaf.id);
  if (existing) {
    updateEntry(existing, leaf, visible);
    return existing.host;
  }
  const host = document.createElement('div');
  host.className = 'pane-host';
  host.style.cssText = 'width:100%;height:100%;min-height:0;display:flex;flex-direction:column';
  const entry: Entry = { host, kind: leaf.kind, instance: null, unmountFn: null, pane: leaf, visible };
  entries.set(leaf.id, entry);
  mountInto(entry);
  return host;
}

export function getHost(paneId: string): HTMLElement | null {
  return entries.get(paneId)?.host ?? null;
}

export function attachHost(paneId: string, slot: HTMLElement): void {
  const e = entries.get(paneId);
  if (e && e.host.parentElement !== slot) slot.appendChild(e.host);
}

export function detachHost(paneId: string): void {
  entries.get(paneId)?.host.remove();
}

export function releaseHost(paneId: string): void {
  const e = entries.get(paneId);
  if (!e) return;
  e.unmountFn?.();
  e.host.remove();
  entries.delete(paneId);
}

export function syncPanes(leaves: Leaf[], visibleIds: ReadonlySet<string>): void {
  const ids = new Set(leaves.map((l) => l.id));
  for (const l of leaves) ensureHost(l, visibleIds.has(l.id));
  for (const id of [...entries.keys()]) if (!ids.has(id)) releaseHost(id);
}
