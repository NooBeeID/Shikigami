# Overview: multi-tab-pane-ticket-01-tab-container

Tiket: `docs/tickets/multi-tab-pane-ticket-01-tab-container.md`
Spec: `docs/superpowers/specs/2026-11-22-multi-tab-pane-design.md`

Semua perubahan frontend-only (Svelte 5 + TypeScript) di `frontend/`. Tanpa perubahan daemon/IPC/Go. Tanpa unit test (jangan tambah vitest).

## 1. Ringkasan epic dan urutan eksekusi

Pane (`Leaf`) menjadi wadah tab: `tabs[]` + `activeTabId`. `openPane` menambah tab ke pane fokus (tidak lagi memecah pane). Pohon split (`Split`, `ratio`, `Splitter`) tidak berubah. Header pane menampilkan deretan tab. Terminal di tab non-aktif tetap ter-mount (hidup). Layout lama tersimpan dimigrasi menjadi pane satu tab.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`, satu agent, sekuensial): tipe `Tab`/`Leaf` baru, helper murni kecil, stub signature operasi tab, pemindahan `hydrateDoc` keluar dari `model.ts`. Setelah selesai, kontrak tipe tersedia untuk semua fitur. Build penuh BELUM harus lulus.
2. **Fitur paralel** (setelah foundation): `tab-model-foundation`, `tab-persist-migration`, `tab-callers`, `tab-hosts-render`, `tab-header-ui`. ownedPaths terpisah.
3. **Wiring** (`tab-wiring`, terakhir): sambungkan, perbaiki sisa error tipe lintas fitur, jalankan build guard.

Catatan: karena `Leaf` berubah bentuk, build penuh baru lulus setelah semua fitur selesai. Tiap fitur boleh meninggalkan error tipe di file milik fitur lain; yang wajib dipenuhi adalah kontrak di bawah persis, dan kode di ownedPaths sendiri benar terhadap kontrak itu.

## 2. Kontrak bersama

Semua dibuat di `frontend/src/modules/layout/model.ts` kecuali disebut lain. Bentuk tidak boleh diubah oleh tiket lain (hanya boleh ditambah).

### 2.1 Tipe

```ts
export type PaneKind = /* tetap */;

export interface Tab {
  id: string;               // prefix 't_'
  kind: PaneKind;
  sessionId?: string; projectId?: string; provider?: string;
  filePath?: string; threadId?: string; workspaceId?: string;
  root?: string; readOnly?: boolean; threadAgent?: boolean;
  sourceId?: string; tableId?: string; viewId?: string; nodeToken?: string;
  number?: number; pipelineId?: string;
  title: string;            // wajib string (default 'Pane' bila kosong), sama seperti Leaf lama
}

export interface Leaf {
  type: 'pane';
  id: string;               // prefix 'p_' (tidak berubah)
  tabs: Tab[];              // tidak pernah kosong
  activeTabId: string;      // selalu menunjuk tab di tabs
}
// Split, Node, LayoutDoc (version: 1), DropZone, SplitPath, NewLeaf: tidak berubah.
```

`focusId` dan `maximizedId` tetap id pane. `makeLeaf(nl: NewLeaf): Tab` (nama dipertahankan) menghasilkan tab dengan id `t_...`.

### 2.2 Helper murni (diekspor dari model.ts)

- `newPaneId(): string` (prefix `p_`; ganti nama dari `newLeafId` privat menjadi ekspor), `newTabId(): string` (prefix `t_`, algoritma sama).
- `activeTab(leaf: Leaf): Tab` (jatuh ke `tabs[0]` bila activeTabId tidak ditemukan).
- `allTabs(doc: LayoutDoc): { pane: Leaf; tab: Tab }[]` (urutan: pane urutan pohon, tab urutan array).
- `findTab(doc: LayoutDoc, pred: (tab: Tab, pane: Leaf) => boolean): { pane: Leaf; tab: Tab } | null`.
- Tetap ada: `leaves(n)`, `findLeaf(doc, paneId)`, `findLeafBy(doc, pred: (l: Leaf) => boolean)` (hanya untuk penelusuran pane, `Leaf` tidak punya field konten).
- `normalize(doc)` diekspor (dipakai `persist/hydrate.ts`).

### 2.3 Operasi (murni, `LayoutDoc -> LayoutDoc` kecuali disebut; tidak memutasi masukan)

- `openPane(doc, nl: NewLeaf): LayoutDoc`
  - Dedupe lintas semua pane (lima aturan): `sessionId`; `filePath` + `projectId`; `threadId`; lark-board per `sourceId`; lark-doc per `sourceId` + `nodeToken`. Ketemu: aktifkan tab, fokus pane, tanpa tab baru.
  - `root` null: pane baru satu tab, fokus.
  - Selain itu: sisipkan tab baru tepat di kanan tab aktif pada pane fokus, aktifkan, fokus. `focusId` tidak valid: pakai pane terakhir (urutan `leaves`).
  - Signature publik store `openPane(nl, opts?)` tidak berubah; `opts.dir` diterima tapi diabaikan.
- `activateTab(doc, paneId, tabId)`: set `activeTabId`; tidak mengubah fokus (fokus pane dilakukan pemanggil lewat `setFocus`/store).
- `closeTab(doc, paneId, tabId)`: tab terakhir = `closePane(doc, paneId)`; bila tab aktif ditutup, aktifkan tetangga (utamakan kanan, lalu kiri). `focusId` mengikuti aturan `closePane` yang ada.
- `renameTab(doc, paneId, tabId, title)`: trim; kosong atau tab tidak ada = doc tak berubah.
- `openThreadAgent(doc, nl): { doc: LayoutDoc; id: string }`: `id` = id PANE yang berisi tab-nya. Bila tab dengan `sessionId` sama sudah ada: aktifkan + fokus. Bila ada tab `threadAgent: true`: ganti `sessionId/projectId/provider/workspaceId/title` di tab itu, aktifkan, fokus. Bila belum: seperti `openPane` dengan `threadAgent: true`.
- `splitPane(...)` (Cmd+D): pane baru berisi satu tab sesi baru; pane asal tidak berubah; pada tab aktif non-terminal = tidak melakukan apa-apa (signature yang ada dipertahankan, hanya cara membaca konten yang berubah ke `activeTab`).
- `movePane`, `swapPanes`, `maximize`, `restore`, `tidyGrid`, `resize`, `closePane`, `setFocus`: perilaku tetap pada pane utuh.
- `renamePane` DIHAPUS (diganti `renameTab`). Jalur lama `renamePane(paneId, title)` pada store boleh dipertahankan sebagai pembungkus yang memanggil `renameTab` pada tab aktif pane itu, hanya bila masih ada pemanggil.

### 2.4 Store layout (`frontend/src/lib/stores/layout.ts`)

Ditambah (nama persis):
- `activateTab(paneId: string, tabId: string): void` (memanggil `activateTab` model lalu `setFocus(paneId)`).
- `closeTab(paneId: string, tabId: string): void`.
- `renameTab(paneId: string, tabId: string, title: string): void`.
- `openPane(nl, opts?)` dan `openThreadAgent(nl)` memakai fungsi model baru. `openThreadAgent` tetap mengembalikan id pane.

### 2.5 Registry (`frontend/src/modules/layout/registry.ts`)

```ts
export interface PaneKindSpec {
  kind: string;
  component: Component<{ pane: Tab; paneId: string; visible: boolean }>;
  titleFor?: (tab: Tab) => string;
  onEndSession?: (tab: Tab) => Promise<void>;
  badgeFor?: (tab: Tab) => string | null;
}
```

Nama prop `pane` dipertahankan (tipe `Tab`). `paneId` selalu diteruskan host (komponen yang tidak butuh boleh mengabaikannya; Svelte 5 `$props()` hanya mendestruktur yang dipakai).

### 2.6 Persistensi (`persist/hydrate.ts`)

- `hydrateDoc(raw: unknown): LayoutDoc` hidup di `frontend/src/modules/persist/hydrate.ts`. `model.ts` mengekspor ulang: `export { hydrateDoc } from '../persist/hydrate';` agar `import { hydrateDoc } from '.../layout/model'` (dipakai `lib/stores/groups.ts`, `lib/stores/layout.ts`) tetap valid. Siklus impor aman karena hanya dipakai saat dipanggil (bukan top-level).
- Terima dua bentuk leaf (lama: field konten langsung + `kind`; baru: `tabs` + `activeTabId`). Lama dibungkus pane satu tab: id pane dipertahankan, id tab = `newTabId()`.
- Autosave menulis bentuk baru (doc apa adanya). `LayoutDoc.version` tetap `1`.

### 2.7 Host konten (`layout/panes/hosts.ts`)

- Di-key dengan id TAB. `syncPanes(entries: { pane: Leaf; tab: Tab }[], ctx)`: bentuk argumen pasti ditentukan agent `tab-hosts-render`, dipanggil HANYA dari `LayoutArea.svelte` (milik fitur yang sama), jadi aman.
- `releaseHost(tabId)` dipanggil bila tab hilang dari doc (ditutup). Tab non-aktif tetap ter-mount dengan `visible=false`.
- Memindah tab antar pane (tiket 02) tidak boleh me-remount: host dikunci id tab saja.

### 2.8 Pemanggil (`shell/helpers.ts`)

`findSessionPane`, `findFilePane`, `openFilePane`, `focusOrOpenSession` mengembalikan `{ paneId: string; tabId: string }` (atau `null` bila pencarian). Kontrak publik `openPane(nl, opts?)` tidak berubah, sehingga pemanggil lain (file tree, palette, lark, git) tidak disentuh.

### 2.9 Konvensi

- Prefix id: pane `p_`, tab `t_`. Kode TypeScript berbahasa Inggris, komentar boleh Indonesia (gaya repo).
- Tidak memutasi `LayoutDoc` masukan; hasilkan objek baru.
- Titik ekstensi untuk tiket berikutnya: `PaneHeader` (tab row), `model.ts` (`moveTab` ditambah tiket 02). Jangan menambah `moveTab`, `registerFileTab`, `confirmCloseTabs`, dirty dot di tiket ini.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `tab-model-foundation` | Operasi tab murni di model.ts, registry, store layout (lihat 2.3-2.5). Ekspor sesuai kontrak. | `frontend/src/modules/layout/model.ts`, `frontend/src/modules/layout/registry.ts`, `frontend/src/lib/stores/layout.ts` |
| `tab-persist-migration` | Implementasi lengkap `persist/hydrate.ts` dua bentuk; adaptasi Restore/TerminalGuard/DeadSession/persist store ke `allTabs`/`findTab`. | `frontend/src/modules/persist/`, `frontend/src/lib/stores/persist.ts` |
| `tab-callers` | Adaptasi pemanggil ke pencarian tab (helpers, jembatan v1, shortcuts, groups, sessionDrag, openfolder, grimoire, thread-store). | `frontend/src/modules/shell/helpers.ts`, `frontend/src/modules/tabs/index.ts`, `frontend/src/modules/layout/shortcuts.ts`, `frontend/src/lib/stores/groups.ts`, `frontend/src/modules/groups/sessionDrag.ts`, `frontend/src/modules/openfolder/logic.ts`, `frontend/src/modules/grimoire/openPage.ts`, `frontend/src/modules/thread/thread-store.ts` |
| `tab-hosts-render` | Host per id tab, LayoutArea, komponen pane menerima `Tab`, refit terminal, registrasi kind. | `frontend/src/modules/layout/panes/`, `frontend/src/modules/layout/LayoutArea.svelte`, `frontend/src/modules/lark/board/BoardPane.svelte`, `frontend/src/modules/git/MrPane.svelte`, `frontend/src/modules/git/PipelinePane.svelte`, `frontend/src/modules/thread/ThreadPane.svelte` |
| `tab-header-ui` | Deretan tab di PaneHeader, menu "Tutup pane", minimize per tab, akhiri sesi tab aktif, a11y. | `frontend/src/modules/layout/PaneHeader.svelte`, `frontend/src/modules/layout/PaneMenu.svelte`, `frontend/src/modules/shell/LayoutHeaderHost.svelte` |
| `tab-wiring` | Dijalankan terakhir: sambungan, perbaikan error tipe lintas fitur, verifikasi build. | `frontend/src/App.svelte`, `frontend/src/modules/layout/index.ts` |

Detail lengkap tiap fitur ada di bagian Fitur tiket (deskripsi dan acceptance criteria). Kontrak di dokumen ini yang menang bila ada ketidaksesuaian kecil.

Catatan penting per fitur:
- `tab-hosts-render` boleh mengubah `FilePane.svelte` hanya agar menerima `Tab` (registri dirty milik tiket 03, jangan disentuh). Tidak boleh menyentuh `PaneHeader`.
- `tab-header-ui` memakai aksi store 2.4 (`activateTab`, `closeTab`) dan `ep.onEndSession`/`badgeFor`/`titleFor` dari registry 2.5. Tutup pane terminal = akhiri sesi seperti perilaku sekarang (pakai `onEndSession` spec kind per tab sebelum `closePane`/`closeTab`).
- `tab-callers` untuk `thread-store.ts`: `openThreadAgent` store mengembalikan id pane; konsumsi sesuai.

## 4. Foundation

Ada (`hasFoundation = true`), lihat `00-foundation.md`. Isi: tipe baru, helper kecil, stub operasi (signature kontrak), ekspor `normalize`/`newPaneId`/`newTabId`, pemindahan `hydrateDoc` ke `persist/hydrate.ts` + ekspor ulang. Foundation menyentuh `model.ts` dan membuat `persist/hydrate.ts`; setelah selesai kepemilikan kembali ke fitur masing-masing.

## 5. Wiring checklist (dibaca agent `tab-wiring`)

1. `frontend/src/modules/layout/index.ts` masih mengimpor pendaftaran kind (`./panes/terminal`, `./panes/file`, dan kind lain yang sudah terdaftar) dan mengekspor API yang dipakai; tidak ada ekspor ke simbol yang dihapus (`renamePane`, field konten Leaf).
2. `frontend/src/App.svelte` tidak memakai API lama: `renamePane`, akses field konten `Leaf` (`.sessionId`, `.filePath`, dst. pada leaf), `opts.dir`. Ganti dengan `allTabs`/`findTab`/`activeTab`.
3. `LayoutArea` memakai `allTabs` untuk `syncPanes`; `PaneHeader` menerima aksi tab dari store (`activateTab`, `closeTab`) via `shell/LayoutHeaderHost.svelte`; semua alur (file tree, sesi baru, thread agent, lark, git, grimoire, drag sesi sidebar) menghasilkan tab terlihat di header.
4. Jalankan `grep -rn "findLeafBy\|findLeaf\|leaves(\|renamePane" frontend/src` dan pastikan tidak ada lagi akses field konten pada `Leaf`; perbaiki sisa error tipe lintas fitur seminimal mungkin (hanya file lintas fitur yang perlu untuk menyambung).
5. Jalankan build guard (bagian 6) sampai lulus.
6. Verifikasi manual bila memungkinkan dan catat di pesan commit: buka file dari file tree berulang kali (tab bertambah di pane fokus), file sama lagi (tab ada diaktifkan), pindah tab terminal/file (terminal hidup), tutup tab, minimize terminal, Cmd+D terminal, muat ulang dengan layout lama dan baru.

## 6. Build guard

Satu-satunya guard (dari root repo `/Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal`):

- `cd frontend && npm run check` (svelte-check; error tipe nol)
- `cd frontend && npm run build` (vite build)
- `go build ./...` (dan/atau perintah build aplikasi proyek bila ada, mis. wails/Makefile; cek `Makefile`/README)

Selama fase paralel, error tipe di file milik fitur lain DIABAIKAN; agent fitur cukup memastikan tidak ada error baru di ownedPaths-nya (filter output `npm run check` ke path sendiri). Build penuh lulus diwajibkan di `tab-wiring`.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya. Jangan menyentuh path fitur lain, file wiring (`App.svelte`, `layout/index.ts`), atau file milik tiket 02/03 (`layout/dnd.ts`, `layout/DropOverlay.svelte`, `layout/TabContextMenu.svelte`, `modules/files/`).
- Bila butuh perubahan di file milik fitur lain, ikuti kontrak apa adanya dan catat di laporan; jangan edit.
- Jangan mengubah bentuk kontrak (hanya boleh menambah). Jangan menambah vitest/unit test.
- Satu agent pada satu waktu mengedit `model.ts` (pemiliknya `tab-model-foundation`; `hydrateDoc` ada di `persist/hydrate.ts` milik `tab-persist-migration`).
- Jangan commit sebelum diminta alur `implement-feature`; jangan pkill pola luas.
