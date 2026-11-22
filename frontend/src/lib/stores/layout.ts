import { derived, get, writable, type Readable } from 'svelte/store';
import * as M from '../../modules/layout/model';
import type { DropZone, Leaf, LayoutDoc, NewLeaf, SplitPath } from '../../modules/layout/model';

export interface LayoutStore extends Readable<LayoutDoc> {
  // opts.dir diterima demi kompatibilitas dan diabaikan (openPane kini menambah tab).
  openPane(leaf: NewLeaf, opts?: { dir?: 'h' | 'v' }): string;
  splitPane(paneId: string, dir: 'h' | 'v', newLeaf: NewLeaf | Leaf, side?: 'after' | 'before'): string;
  openThreadAgent(leaf: NewLeaf): string;
  closePane(paneId: string): void;
  movePane(paneId: string, targetId: string, zone: DropZone): void;
  swapPanes(aId: string, bId: string): void;
  resize(path: SplitPath, ratio: number): void;
  maximize(paneId: string): void;
  restore(): void;
  tidyGrid(): void;
  setFocus(paneId: string): void;
  activateTab(paneId: string, tabId: string): void;
  closeTab(paneId: string, tabId: string): void;
  renameTab(paneId: string, tabId: string, title: string): void;
  // Pembungkus: ganti judul tab aktif pane.
  renamePane(paneId: string, title: string): void;
  hydrate(doc: unknown): void;
  serialize(): LayoutDoc;
  readonly paneCount: Readable<number>;
  readonly leaves: Readable<Leaf[]>;
}

export function createLayoutStore(initial?: LayoutDoc): LayoutStore {
  const store = writable<LayoutDoc>(initial ? M.hydrateDoc(initial) : M.emptyDoc());
  const { subscribe, update, set } = store;
  return {
    subscribe,
    openPane(leaf) {
      let id = '';
      update((d) => {
        const next = M.openPane(d, leaf);
        id = next.focusId ?? '';
        return next;
      });
      return id;
    },
    splitPane(paneId, dir, newLeaf, side) {
      let id = '';
      update((d) => {
        const r = M.splitPane(d, paneId, dir, newLeaf, side);
        id = r.id;
        return r.doc;
      });
      return id;
    },
    openThreadAgent(leaf) {
      let id = '';
      update((d) => {
        const r = M.openThreadAgent(d, leaf);
        id = r.id;
        return r.doc;
      });
      return id;
    },
    closePane: (id) => update((d) => M.closePane(d, id)),
    movePane: (id, target, zone) => update((d) => M.movePane(d, id, target, zone)),
    swapPanes: (a, b) => update((d) => M.swapPanes(d, a, b)),
    resize: (path, ratio) => update((d) => M.resize(d, path, ratio)),
    maximize: (id) => update((d) => M.maximize(d, id)),
    restore: () => update((d) => M.restore(d)),
    tidyGrid: () => update((d) => M.tidyGrid(d)),
    setFocus: (id) => update((d) => M.setFocus(d, id)),
    activateTab: (paneId, tabId) => update((d) => M.setFocus(M.activateTab(d, paneId, tabId), paneId)),
    closeTab: (paneId, tabId) => update((d) => M.closeTab(d, paneId, tabId)),
    renameTab: (paneId, tabId, title) => update((d) => M.renameTab(d, paneId, tabId, title)),
    renamePane: (id, title) =>
      update((d) => {
        const p = M.findLeaf(d, id);
        return p ? M.renameTab(d, id, M.activeTab(p).id, title) : d;
      }),
    hydrate: (raw) => set(M.hydrateDoc(raw)),
    serialize: () => JSON.parse(JSON.stringify(get(store))) as LayoutDoc,
    paneCount: derived(store, (d) => M.leaves(d).length),
    leaves: derived(store, (d) => M.leaves(d)),
  };
}

export const layout: LayoutStore = createLayoutStore();
