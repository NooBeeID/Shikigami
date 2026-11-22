# Overview: welcome-screen-ticket-02-welcome-screen (Layar welcome dan terakhir dibuka)

Sumber: tiket `docs/tickets/welcome-screen-ticket-02-welcome-screen.md` dan spec `docs/superpowers/specs/2026-11-22-welcome-screen-design.md`. Dokumen ini adalah sumber kebenaran kontrak bersama; plan per fitur hanya merinci langkah kerja. Frontend: Svelte 5 (runes) + Vite di `frontend/` (script `build` = `vite build`, `check` = `svelte-check`). Epic ini murni frontend; tidak ada perubahan Go.

Prasyarat: tiket 1 (`welcome-screen-ticket-01-session-workspaces`) sudah terimplementasi: `openWorkspaceIds`, `openWorkspace`, `closeWorkspaceTab`, `Workspace.lastOpenedAt`, RPC `workspace.touch`, dan `newWorkspaceFromFolder` yang memakai `openWorkspace`. Epic ini hanya memakai, tidak mengubahnya. Bila nama ekspor di kode berbeda sedikit dari tiket, periksa kode dan ikuti kode.

## 1. Ringkasan epic dan urutan eksekusi

Tujuan: saat `activeWorkspaceId === null` (start aplikasi, atau setelah tab terakhir ditutup), area tengah merender `Welcome` alih-alih `LayoutArea`. Welcome punya dua tombol (Open Workspace, Create New Workspace) dan daftar "Terakhir dibuka" (maks 5).

Urutan:
1. **Foundation: tidak ada** (`hasFoundation=false`, tidak ada `00-foundation.md`). Tidak ada skeleton paket atau tipe baru; folder `modules/welcome/` dibuat oleh fitur yang pertama menulis file di dalamnya. Kontrak tanda tangan fungsi di bagian 2 cukup agar tiap fitur bisa dikompilasi sendiri.
2. **Fitur paralel (gelombang A)**: `welcome-logic` dan `welcome-ui` (tidak berbagi file). Agar `welcome-ui` dapat dibuild sendiri bila `logic.ts` belum merge, ia mengikuti kontrak tanda tangan di bagian 2.2 persis; build akhir yang valid dicek setelah keduanya ada.
3. **Wiring (gelombang B)**: `welcome-wiring` dikerjakan terakhir, setelah `welcome-logic` dan `welcome-ui` merge (butuh `Welcome` dari `modules/welcome/index.ts`). Ia sekaligus verifikasi end-to-end (bagian 5).

## 2. Kontrak bersama

### 2.1 Folder dan penamaan
- Modul baru: `frontend/src/modules/welcome/` berisi `logic.ts`, `Welcome.svelte`, `index.ts`. Tidak ada file lain.
- Teks UI bahasa Indonesia untuk status/kosong; label tombol tetap Inggris. Identifier kode bahasa Inggris. Komentar singkat dalam bahasa Indonesia.
- Tanpa dependency baru, tanpa unit test, tanpa `registerPanel` (Welcome menggantikan area tengah, bukan panel).

### 2.2 `frontend/src/modules/welcome/logic.ts` (milik `welcome-logic`)
Ekspor persis:
```ts
import type { Workspace } from '../../lib/stores';

export function recentWorkspaces(workspaces: Workspace[], openIds: string[], limit = 5): Workspace[];
export function findWorkspaceByPath(workspaces: Workspace[], path: string): Workspace | undefined;
export async function openFromDisk(): Promise<void>;
export async function createNew(): Promise<Workspace | null>;
export async function openRecent(id: string): Promise<void>;
```
Perilaku:
- `recentWorkspaces` murni: filter `(lastOpenedAt ?? 0) > 0` dan `!openIds.includes(id)`; urut `lastOpenedAt` menurun (jangan mutasi input, salin dulu); `slice(0, limit)`.
- `findWorkspaceByPath` murni: normalisasi path (ulangi buang `/` atau `\` di akhir selama panjang > 1) lalu bandingkan dengan `projects[].path` ternormalisasi tiap workspace; kembalikan yang pertama cocok.
- `openFromDisk`: guard `busy` modul; `pickDirectory()` (dari `../appmenu`); hasil kosong/batal -> return tanpa menyentuh store; bila `findWorkspaceByPath(get(workspaces), path)` ada -> `openWorkspace(ws.id)`; bila tidak -> `newWorkspaceFromFolder(path)` (sudah memanggil `openWorkspace`). Semua error: `showToast(errMessage(e), 'error')`, tidak dilempar. `busy` direset di `finally`.
- `createNew`: `createWorkspace(defaultWorkspaceName(get(workspaces)))`; kembalikan workspace baru; TIDAK memanggil `openWorkspace`. Error -> toast, kembalikan `null`.
- `openRecent(id)`: `openWorkspace(id)` dengan try/catch toast.

Impor yang dipakai (nama persis di kode):
- `workspaces`, `openWorkspace`, `createWorkspace`, tipe `Workspace` dari `../../lib/stores` (barrel `export * from './workspaces'`).
- `newWorkspaceFromFolder`, `errMessage` dari `../openfolder/logic`.
- `pickDirectory` dari `../appmenu`.
- `showToast` dari `../openfolder/toast-store`.
- `defaultWorkspaceName` dari `../topbar/util`.
- `get` dari `svelte/store`.

### 2.3 `Welcome.svelte` dan `index.ts` (milik `welcome-ui`)
- `index.ts`: `export { default as Welcome } from './Welcome.svelte';` (hanya itu).
- `Welcome.svelte`: tanpa props. Memakai `logic.ts` (kontrak 2.2) dan store `workspaces`, `openWorkspaceIds`, `openWorkspace`, `renameWorkspace` dari `../../lib/stores`, `showToast` dari `../openfolder/toast-store`, `errMessage`/`errText` sesuai kebutuhan.
- Daftar reaktif: `recentWorkspaces($workspaces, $openWorkspaceIds)`.
- Alur Create New: tombol -> `createNew()` -> bila bukan `null`, tampilkan input inline (state lokal `$state`, mis. `pendingId`, `draftName`, `committing`), fokus+seleksi seperti `focusSelect` di `WorkspaceTabs.svelte` (jangan impor dari sana, salin polanya). Enter/blur -> commit: bila `trim()` tidak kosong dan beda -> `renameWorkspace` (error -> toast, lanjut); lalu `openWorkspace(id)`. Escape -> batal rename, tetap `openWorkspace(id)`. Guard `committing` agar tidak ganda. Tombol lain `disabled` selama input aktif.
- Visual: variabel tema dari `styles/theme.css`: `--bg`, `--bg-elev`, `--bg-hover`, `--border`, `--fg`, `--fg-muted`, `--accent`, `--danger`. Tanpa warna hard-coded. `height: 100%`, tanpa scroll horizontal, fokus keyboard terlihat (`:focus-visible`).
- Teks tetap: "Terakhir dibuka", "Belum ada workspace terakhir", "Open Workspace", "Create New Workspace".

### 2.4 `App.svelte` (milik `welcome-wiring`)
Hanya tambahan impor `activeWorkspaceId` (ke baris impor `./lib/stores` yang sudah ada) dan `Welcome` dari `./modules/welcome`, serta ubah snippet `center`:
```svelte
{#snippet center()}{#if $activeWorkspaceId === null}<Welcome />{:else}<LayoutArea />{/if}{/snippet}
```

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `welcome-logic` | `recentWorkspaces`, `findWorkspaceByPath` (murni) dan aksi `openFromDisk`, `createNew`, `openRecent` dengan guard `busy` dan toast error. | `frontend/src/modules/welcome/logic.ts` |
| `welcome-ui` | Komponen `Welcome.svelte` (dua tombol berikon, daftar Terakhir dibuka, input nama inline setelah Create New) dan `index.ts`. | `frontend/src/modules/welcome/Welcome.svelte`, `frontend/src/modules/welcome/index.ts` |
| `welcome-wiring` | `App.svelte` memilih `Welcome` atau `LayoutArea`; verifikasi manual end-to-end; catat temuan crash null untuk tiket 1. | `frontend/src/App.svelte` |

Ketergantungan: `welcome-logic` dan `welcome-ui` paralel (kontrak 2.2 dipegang). `welcome-wiring` setelah keduanya.

## 4. Foundation

Tidak ada. Alasan: tidak ada skeleton paket/tipe bersama yang harus ada sebelum paralel; kontrak bagian 2 cukup. `hasFoundation=false`, file `00-foundation.md` tidak dibuat.

## 5. Wiring checklist

Dikerjakan oleh `welcome-wiring` setelah `welcome-logic` dan `welcome-ui` merge:
- [ ] `App.svelte`: impor `activeWorkspaceId` dari `./lib/stores` dan `Welcome` dari `./modules/welcome`.
- [ ] Snippet `center` merender `<Welcome />` bila `$activeWorkspaceId === null`, selain itu `<LayoutArea />`; bagian lain `App.svelte` (koneksi, toast, palette, settings, zoom) tidak berubah.
- [ ] Tidak ada registrasi (`registerPanel`) dan tidak ada rute baru.
- [ ] Verifikasi manual di aplikasi (tiket 1 harus sudah aktif; daemon dibangun ulang agar `workspace.touch` tersedia):
  1. Start: welcome tanpa tab; "Belum ada workspace terakhir" bila belum ada riwayat; tanpa error konsol.
  2. Open Workspace -> folder baru -> tab dengan nama folder, welcome hilang; batal dialog -> tidak ada efek.
  3. Tutup tab (menu "Tutup") -> welcome kembali; workspace tadi muncul di "Terakhir dibuka" dengan path project pertama.
  4. Klik baris itu -> tab terbuka; entri hilang dari daftar.
  5. Open Workspace -> folder yang sama (termasuk varian slash akhir) -> workspace yang ada dibuka, tanpa duplikat.
  6. Create New Workspace -> input inline -> ganti nama (Enter/blur) -> tab dengan nama baru; Escape -> tab dengan nama default.
  7. Buka lebih dari 5 workspace lalu tutup semua -> daftar hanya 5 terbaru.
  8. Restart app -> welcome tanpa tab; daftar terisi dari riwayat.
- [ ] Temuan crash konsumen `activeWorkspace` saat null di luar `App.svelte` hanya dicatat sebagai temuan untuk tiket 1, tidak diperbaiki.

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- `go build ./...` (root repo; harus tetap lulus, tidak ada perubahan Go).
- `cd frontend && npm run build`, ditambah `npm run check` bila skrip ada (ada: `svelte-check`).
- `welcome-logic` dan `welcome-ui` cukup build frontend; `welcome-wiring` menjalankan keduanya.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain.
- Dilarang mengubah: `WorkspaceTabs.svelte`, `lib/stores/*`, `modules/openfolder/*`, `modules/topbar/*`, `modules/persist/*`, kode Go. Jangan mengubah mekanisme ganti nama di `WorkspaceTabs` (`editingId` lokal komponen itu).
- Hanya `welcome-wiring` yang boleh menyentuh `App.svelte`; `welcome-logic` dan `welcome-ui` tidak.
- Jangan menambah dependency atau unit test. Jangan commit/merge di luar alur implement-feature.
