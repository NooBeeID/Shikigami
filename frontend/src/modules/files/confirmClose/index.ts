import { mount, unmount } from 'svelte';
import { getFileTab } from '../tabRegistry';
import ConfirmCloseDialog from './ConfirmCloseDialog.svelte';

// Hanya satu dialog sekali. Panggilan kedua saat dialog terbuka langsung `false` (tidak antre).
let open = false;

// Meminta konfirmasi sebelum menutup tab. `labels` memetakan id tab ke nama tampilan;
// tanpa entri, id tab dipakai sebagai label.
// true  = lanjut menutup (tidak ada tab kotor, Simpan berhasil, atau Buang).
// false = batalkan (Batal / Escape / klik latar / dialog lain sedang terbuka).
// Fungsi ini tidak menyentuh tab/layout; hanya mengembalikan keputusan.
export function confirmCloseTabs(tabIds: string[], labels?: Record<string, string>): Promise<boolean> {
  const dirtyIds = tabIds.filter((id) => getFileTab(id)?.isDirty());
  if (dirtyIds.length === 0) return Promise.resolve(true);
  if (open) return Promise.resolve(false);
  open = true;
  const items = dirtyIds.map((id) => ({ id, label: labels?.[id] ?? id }));

  return new Promise<boolean>((resolve) => {
    let instance: ReturnType<typeof mount> | undefined;
    let settled = false;
    const finish = (result: boolean) => {
      if (settled) return;
      settled = true;
      if (instance) void unmount(instance);
      open = false;
      resolve(result);
    };

    // Simpan berurutan; semua file dicoba agar yang bisa tersimpan tetap tersimpan.
    // Handle yang sudah hilang dianggap berhasil.
    const onSave = async (): Promise<{ ok: true } | { ok: false; failedLabels: string[] }> => {
      const failedLabels: string[] = [];
      for (const item of items) {
        const h = getFileTab(item.id);
        const ok = h ? await h.save().catch(() => false) : true;
        if (!ok) failedLabels.push(item.label);
      }
      if (failedLabels.length > 0) return { ok: false, failedLabels };
      finish(true);
      return { ok: true };
    };

    try {
      instance = mount(ConfirmCloseDialog, {
        target: document.body,
        props: { items, onSave, onDiscard: () => finish(true), onCancel: () => finish(false) },
      });
    } catch {
      open = false;
      settled = true;
      resolve(false);
    }
  });
}
