// Mekanisme slot thread. Tiket thread-02..05 memasang komponen lewat registerThreadSlot
// tanpa mengedit berkas lama.
//
// Nama slot:
//  - 'header-actions': di kanan ThreadHeader.
//  - 'above-input': di atas input pesan.
//  - 'message-extra': dirender per pesan (props memuat `message`).
//  - 'side-panel': kolom kanan tab; kolom hanya muncul bila ada komponen terdaftar.
// Komponen slot menerima props `{thread: Thread, message?: ThreadMessage}`.
import type { Component } from 'svelte';
import { derived, writable, type Readable } from 'svelte/store';

export type ThreadSlotName = 'header-actions' | 'above-input' | 'message-extra' | 'side-panel';
export interface ThreadSlotSpec { slot: ThreadSlotName; component: Component<any>; order?: number; id?: string }

const slotStore = writable<ThreadSlotSpec[]>([]);

export function registerThreadSlot(spec: ThreadSlotSpec): void {
  slotStore.update((list) => {
    if (spec.id) {
      const i = list.findIndex((s) => s.id === spec.id);
      if (i >= 0) {
        const next = list.slice();
        next[i] = spec;
        return next;
      }
    }
    return [...list, spec];
  });
}

export function threadSlots(name: ThreadSlotName): Readable<ThreadSlotSpec[]> {
  return derived(slotStore, (list) =>
    list
      .map((s, i) => ({ s, i }))
      .filter((x) => x.s.slot === name)
      .sort((a, b) => (a.s.order ?? 0) - (b.s.order ?? 0) || a.i - b.i)
      .map((x) => x.s),
  );
}

export function hasThreadSlot(name: ThreadSlotName): Readable<boolean> {
  return derived(slotStore, (list) => list.some((s) => s.slot === name));
}
