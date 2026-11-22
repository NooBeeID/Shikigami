# Plan: Wiring konfirmasi dan titik belum disimpan

## Metadata

- Tiket: `docs/tickets/multi-tab-pane-ticket-03-unsaved-confirm.md` (epic `multi-tab-pane-ticket-03-unsaved-confirm`)
- Fitur: `unsaved-wiring` (tahap penutup, BUKAN paralel; dikerjakan setelah `file-tab-registry` dan `confirm-close-dialog` selesai)
- ownedPaths:
  - `frontend/src/modules/layout/PaneHeader.svelte`
  - `frontend/src/modules/layout/PaneMenu.svelte`
- Depends on foundation: ya (`files/tabRegistry.ts` dari `00-foundation.md` harus sudah ada; sudah pasti ada bila dua fitur paralel selesai)
- Prasyarat lain: tiket 01 dan 02 sudah ter-merge. Catatan: saat plan ini ditulis, `PaneHeader.svelte` (276 baris) dan `PaneMenu.svelte` (128 baris) di repo masih bentuk lama (satu `onClose`, `onMinimize`, `onEndSession` sebagai props dari luar, belum ada deretan tab). Agent WAJIB membaca kode aktual di working tree dulu; bila nama aktual berbeda dari kontrak di bawah, ikuti kode aktual dan catat di laporan.

Rujukan: `overview.md` bagian 2 (kontrak), 2.5 (titik dirty dan pemanggil tutup), 5 (wiring checklist), 6 (build guard).

## Tujuan

Menyambungkan hasil dua fitur paralel ke UI pane:

1. Semua aksi tutup yang dipicu pengguna di header/menu pane menunggu `await confirmCloseTabs(ids, labels)` dan hanya lanjut bila `true`.
2. Tab file yang kotor menampilkan titik "belum disimpan" (aktif maupun non-aktif) tanpa menggeser tombol tutup tab.
3. Verifikasi end-to-end, build frontend dan aplikasi lulus, dan catat (jangan kerjakan) penanganan tutup jendela/keluar aplikasi dengan file kotor.

## Path yang dimiliki

HANYA boleh disentuh:

- `frontend/src/modules/layout/PaneHeader.svelte`
- `frontend/src/modules/layout/PaneMenu.svelte`

Jangan sentuh: `files/index.ts`, `files/tabRegistry.ts`, `files/editor/FileTab.svelte`, `files/confirmClose/`, `layout/panes/FilePane.svelte` (milik fitur lain), serta `layout/model.ts`, `layout/registry.ts`, `layout/LayoutArea.svelte`, `layout/dnd.ts`, `layout/DropOverlay.svelte`, `layout/TabContextMenu.svelte`, `files/shared/shellBridge.ts`, dan pemanggil `closeTab`/`closePane` di modul lain (shell, groups, jembatan v1). Bila ada yang kurang di file-file itu, LAPORKAN, jangan diedit.

## Kontrak

Dipakai (disediakan fitur lain, bentuk tidak boleh diubah; detail di overview 2.2 dan 2.3):

```ts
// dari frontend/src/modules/files (index.ts)
export const dirtyTabs: Readable<ReadonlySet<string>>; // id tab kotor; baca di Svelte dengan $dirtyTabs
export function isTabDirty(tabId: string): boolean;

// dari frontend/src/modules/files/confirmClose (index.ts)
export function confirmCloseTabs(
  tabIds: string[],
  labels?: Record<string, string>, // tabId -> nama tampilan
): Promise<boolean>; // true = lanjut tutup; false = batalkan
```

Perilaku penting `confirmCloseTabs`: tanpa tab kotor langsung `true` tanpa UI; panggilan kedua saat dialog terbuka langsung mengembalikan `false` (tidak antre); fungsi ini tidak menyentuh layout.

Dari tiket 01/02 (bentuk mengikuti kode aktual): `Tab` (`id`, `kind`, `filePath`, `title`, ...), `Leaf = { tabs: Tab[], activeTabId, ... }`, `closeTab(paneId, tabId)`, `closePane`, menu konteks tab.

Disediakan fitur ini: tidak ada API baru. Perubahan murni internal di dua komponen; props publik `PaneHeader`/`PaneMenu` sebisa mungkin tidak berubah bentuk (lihat Langkah 3).

## Langkah

1. Verifikasi prasyarat (hanya baca, jangan edit file fitur lain):
   - `frontend/src/modules/files/index.ts` mengekspor `dirtyTabs` dan `isTabDirty`.
   - `frontend/src/modules/files/confirmClose/index.ts` mengekspor `confirmCloseTabs(tabIds, labels?)`.
   - `frontend/src/modules/layout/panes/FilePane.svelte` meneruskan `tabId={pane.id}` ke `<FileTab>` (cabang non-readOnly).
   - Bila ada yang kurang, hentikan bagian terkait dan laporkan (jangan menambal di file milik fitur lain).
2. Inventaris jalur tutup. Baca `PaneHeader.svelte`, `PaneMenu.svelte`, dan (hanya baca) `LayoutArea.svelte`/`TabContextMenu.svelte`. Telusuri SEMUA tempat yang memicu penutupan tab/pane: `grep -n "closeTab\|closePane\|onClose\|onCloseTab\|onCloseOthers\|onCloseAll\|Tutup" PaneHeader.svelte PaneMenu.svelte`. Buat daftar singkat (di catatan laporan) tiap jalur: tombol tutup tab, "Tutup pane" (tombol header lama `hbtn close` dan item menu), "Tutup tab lain"/"Tutup semua"/"Tutup ke kanan" bila ada, dan klasifikasikan: dipicu pengguna (dilindungi) atau dikecualikan (lihat Langkah 7).
   - Bila aksi tutup dieksekusi lewat callback props dari `LayoutArea` (mis. `onClose`, `onCloseTab`), JANGAN ubah `LayoutArea`. Bungkus di sisi `PaneHeader`/`PaneMenu`: komponen itu memanggil `confirmCloseTabs` dulu, lalu memanggil callback props hanya bila `true`.
   - Bila `TabContextMenu.svelte` (tidak dimiliki) memanggil `closeTab`/`closePane` langsung tanpa lewat `PaneHeader`/`PaneMenu`, jalur itu tidak bisa dilindungi dari path ini: catat di laporan sebagai temuan, jangan ubah file itu.
3. `PaneHeader.svelte`: tambah impor dan helper di `<script>`:
   ```ts
   import { dirtyTabs } from '../files';
   import { confirmCloseTabs } from '../files/confirmClose';
   ```
   Tambah helper label dan guard (nama bebas, signature kira-kira):
   ```ts
   function tabLabel(t: Tab): string {
     return t.title?.trim() || t.filePath?.split('/').pop() || t.id;
   }
   let closing = $state(false); // guard klik ganda selama dialog/penutupan berlangsung
   async function guardedClose(tabs: Tab[], proceed: () => void): Promise<void> {
     if (closing || tabs.length === 0) return;
     closing = true;
     try {
       const labels = Object.fromEntries(tabs.map((t) => [t.id, tabLabel(t)]));
       const ok = await confirmCloseTabs(tabs.map((t) => t.id), labels);
       if (ok) proceed();
     } finally {
       closing = false;
     }
   }
   ```
   Pakai tipe `Tab` dari `./model` (sesuaikan nama impor dengan kode aktual). `proceed` dijalankan setelah `await`; pastikan tab/pane masih ada saat `proceed` (baca ulang dari props/store, bukan snapshot lama), dan pertahankan perilaku lama bila tab sudah hilang (no-op).
4. `PaneHeader.svelte`: bungkus tombol tutup tab. Handler jadi `() => guardedClose([tab], () => <aksi closeTab lama untuk tab itu>)`. Tombol tutup tab TIDAK memakai `closing` sebagai `disabled` yang mengubah tata letak; cukup guard di fungsi (boleh `aria-disabled`). Tombol header "Tutup pane" (bila masih ada di header) dan aksi tutup massal yang dirender di header: `guardedClose(pane.tabs, <aksi tutup pane lama>)`, dengan `pane.tabs` = semua tab di pane itu.
5. `PaneMenu.svelte`: item "Tutup pane" (saat ini `run(onClose)`) harus melewati konfirmasi semua tab pane. Dua opsi, pilih yang cocok dengan kode aktual:
   - (a) Bila `PaneHeader` meneruskan `onClose` ke `PaneMenu`: biarkan `PaneMenu` memanggil `onClose` apa adanya, dan pastikan `onClose` yang diteruskan `PaneHeader` sudah dibungkus `guardedClose` (satu sumber guard, tidak ada konfirmasi ganda). Ini pilihan default agar props publik `PaneMenu` tidak berubah.
   - (b) Bila `PaneMenu` memanggil `closeTab`/`closePane` sendiri: impor `confirmCloseTabs` di `PaneMenu` dan lakukan pola yang sama (flag `closing` lokal, `labels` dari `Tab`), menerima `tabs` lewat prop baru bila perlu.
   Pastikan satu aksi pengguna hanya memicu satu dialog (jangan membungkus di kedua tingkat). Item "Akhiri sesi" TIDAK dibungkus (tab terminal tidak kotor; lihat Langkah 7). Setelah `run(...)` menutup menu, pastikan menu tidak menahan fokus yang mengganggu dialog (dialog dipasang di `document.body`).
6. Jalur tutup massal lain di menu konteks tab yang dirender oleh `PaneHeader`/`PaneMenu` ("Tutup tab lain", "Tutup semua", dst., bila ada menurut Langkah 2): kumpulkan id tab yang akan ditutup (bukan seluruh pane untuk "tab lain"), panggil `guardedClose(tabsYangDitutup, proceed)`. Satu dialog untuk seluruh set.
7. Pengecualian, JANGAN dibungkus (pastikan tidak sengaja tersentuh saat refactor): `moveTab`, "Pisahkan ke pane baru", reorder tab (drag), minimize tab terminal, "Akhiri sesi", penutupan sistem, serta pemanggil `closeTab`/`closePane` di modul lain.
8. `PaneHeader.svelte`: titik belum disimpan pada item tab. Di markup tiap tab (aktif maupun non-aktif):
   ```svelte
   {#if $dirtyTabs.has(tab.id)}
     <span class="dirty-dot" title="Belum disimpan" aria-hidden="true">●</span>
     <span class="sr-only">Belum disimpan</span>
   {/if}
   ```
   - Tempatkan di sebelah judul tab, atau menggantikan ikon status tab file, dengan lebar tetap/`flex: none` agar tombol tutup tab tidak bergeser (mis. slot indikator selalu mengalokasikan lebar yang sama, atau titik ditumpuk absolut di slot ikon). Tab non-file tidak berubah (`dirtyTabs` hanya berisi id tab file, jadi cukup cek `has`).
   - CSS memakai token `var(--...)` yang sudah dipakai di file itu (warna aksen/peringatan yang sudah ada), benar di tema gelap/terang. Bila kelas `.sr-only` belum ada di file/global, definisikan kelas lokal.
   - Pastikan `$dirtyTabs` dipakai dengan sintaks store Svelte (`$` prefix) agar reaktif.
9. Build guard: `cd frontend && npm run build`; lalu `cd frontend && npm run check` (svelte-check) bila bersih di baseline, jika tidak pastikan tidak ada galat baru dari dua file yang diubah. Lalu build aplikasi: `make build` (atau minimal `make build-ui && make build-app`) dan `go build ./...` bila tersedia.
10. Verifikasi end-to-end (jalankan aplikasi; lihat skill `run` bila perlu):
    - Buka dua file di pane yang sama, ubah keduanya: kedua tab menampilkan titik (juga yang non-aktif).
    - Tutup satu tab kotor: dialog satu file ("Simpan" / "Buang" / "Batal"). Batal: tab tetap. Buang: tab tertutup tanpa menyimpan. Simpan: tersimpan, titik hilang, tab tertutup.
    - Tutup pane berisi dua file kotor: satu dialog berisi daftar dua file ("Simpan semua" / "Buang semua" / "Batal"). Uji ketiganya, serta Escape dan klik latar = Batal.
    - Simulasi galat simpan (file read-only di disk) bila memungkinkan: dialog tetap terbuka dengan pesan galat berisi nama file, tab tidak tertutup, Batal dan Buang tetap aktif.
    - Klik ganda cepat pada tombol tutup: hanya satu dialog.
    - Tab bersih dan tab terminal menutup tanpa dialog dan tanpa jeda yang terasa.
    - Drag tab ke pane lain, "Pisahkan ke pane baru", reorder, minimize terminal, "Akhiri sesi": tidak ada dialog; state kotor file tetap (tidak ter-remount) setelah dipindah.
    - Menu konteks tab: jalur tutup massal (bila ada) menampilkan satu dialog berisi hanya tab yang akan ditutup.
11. Catat (JANGAN kerjakan) di laporan akhir: apakah ada penanganan tutup jendela/keluar aplikasi dengan file kotor (cari mis. `beforeunload`, `onCloseRequested`, hook window close Wails di `frontend/src` dan Go app). Bila belum ada, tulis sebagai item terpisah. Catat juga jalur tutup yang tidak bisa dilindungi dari ownedPaths (Langkah 2) dan penyimpangan nama dari kontrak.

## Perubahan

Diubah:
- `frontend/src/modules/layout/PaneHeader.svelte`: impor `dirtyTabs` dan `confirmCloseTabs`; helper `tabLabel` dan `guardedClose` dengan flag `closing`; pembungkus tombol tutup tab, "Tutup pane", dan jalur tutup massal; titik belum disimpan beserta CSS.
- `frontend/src/modules/layout/PaneMenu.svelte`: "Tutup pane" (dan jalur tutup massal di menu bila ada) melewati konfirmasi, tanpa konfirmasi ganda.

Dibuat: tidak ada.

## Wiring

Ini sendiri adalah langkah wiring; tidak ada langkah wiring lanjutan. Fitur ini tidak mengekspos API baru. Yang dikonsumsi (sudah harus ada): `dirtyTabs` dari `files`, `confirmCloseTabs` dari `files/confirmClose`, dan `tabId={pane.id}` di `FilePane.svelte`. Jangan menyambungkan atau memperbaiki ketiganya dari path ini; bila kurang, laporkan.

## Selesai bila

- Tombol tutup tab file kotor membuka dialog; Batal membiarkan tab, Buang menutup tanpa menyimpan, Simpan menyimpan lalu menutup.
- "Tutup pane" dan jalur tutup massal di PaneHeader/PaneMenu mengumpulkan semua tab file kotor dalam satu dialog, tanpa dialog ganda.
- Tab bersih dan terminal menutup tanpa dialog; `moveTab`, drag, split, reorder, minimize, "Akhiri sesi" tidak memunculkan dialog dan tidak me-remount isi.
- Titik belum disimpan muncul pada tab file kotor (aktif maupun non-aktif), hilang setelah simpan, dan tidak menggeser tombol tutup.
- Hanya dua ownedPaths yang berubah (`git status` memastikan).
- Guard: `cd frontend && npm run build` sukses (dan `npm run check` tanpa galat baru dari file ini); build aplikasi (`make build` / `go build ./...`) sukses; aplikasi jalan. Tanpa unit test dan tanpa vitest.
- Laporan memuat catatan penanganan tutup jendela dengan file kotor (item terpisah, tidak dikerjakan) dan temuan jalur tutup di luar ownedPaths.
