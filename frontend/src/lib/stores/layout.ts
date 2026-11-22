import { derived, get, writable, type Readable } from 'svelte/store';
import * as M from '../../modules/layout/model';
import type { DropZone, Leaf, LayoutDoc, NewLeaf, SplitPath } from '../../modules/layout/model';

export interface LayoutStore extends Readable<LayoutDoc> {
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
    openPane(leaf, opts) {
      let id = '';
      update((d) => {
        const r = M.openPane(d, leaf, opts);
        id = r.id;
        return r.doc;
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
    renamePane: (id, title) => update((d) => M.renamePane(d, id, title)),
    hydrate: (raw) => set(M.hydrateDoc(raw)),
    serialize: () => JSON.parse(JSON.stringify(get(store))) as LayoutDoc,
    paneCount: derived(store, (d) => M.leaves(d).length),
    leaves: derived(store, (d) => M.leaves(d)),
  };
}

export const layout: LayoutStore = createLayoutStore();
