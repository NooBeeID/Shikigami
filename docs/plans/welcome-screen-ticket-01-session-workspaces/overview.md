# Overview: welcome-screen-ticket-01-session-workspaces (Workspace terbuka per sesi)

Sumber: tiket `docs/tickets/welcome-screen-ticket-01-session-workspaces.md` dan spec `docs/superpowers/specs/2026-11-22-welcome-screen-design.md`. Dokumen ini adalah sumber kebenaran kontrak bersama; plan per fitur hanya merinci langkah kerja. Module Go: `github.com/NooBeeID/shikigami`. Frontend: Svelte 5 + Vite di `frontend/` (script `build` = `vite build`, `check` = `svelte-check`).

## 1. Ringkasan epic dan urutan eksekusi

Tujuan: aplikasi mulai kosong (tanpa tab, tanpa workspace aktif). Tab hanya untuk workspace yang dibuka di sesi aplikasi itu (`openWorkspaceIds`, memori saja). Backend mencatat `lastOpenedAt` lewat RPC `workspace.touch`. Menu tab mendapat aksi "Tutup" di samping "Hapus". Layar `Welcome` dan perubahan `App.svelte` adalah tiket 02 (jangan disentuh).

Urutan:
1. **Foundation: tidak ada** (`hasFoundation=false`, tidak ada `00-foundation.md`). Tiga fitur pertama tidak berbagi file dan hanya saling bergantung lewat kontrak tertulis di bagian 2, yang cukup untuk build masing-masing (RPC dipanggil lewat `call` string; state null diuji lewat `activeWorkspaceId.set(null)`).
2. **Fitur paralel (gelombang A)**: `workspace-touch-go`, `open-set-store`, `null-workspace-safety`.
3. **Fitur penutup (gelombang B)**: `tabs-close-menu`. Dikerjakan setelah `open-set-store` merge (memakai `openWorkspaceIds`, `openWorkspace`, `closeWorkspaceTab`), idealnya setelah `workspace-touch-go`. Fitur ini sekaligus menjadi Wiring epic (lihat bagian 5); tidak ada fase wiring terpisah di luar verifikasi akhir.

Keadaan yang disengaja: workspace lama yang sudah tersimpan tidak punya jalur UI untuk dibuka kembali di tiket ini (hadir di tiket 02).

## 2. Kontrak bersama

### 2.1 Backend Go (`internal/workspace`)
- `Workspace` di `types.go` bertambah field:
  `LastOpenedAt int64 `json:"lastOpenedAt"`` (unix ms; 0 = belum pernah dibuka / data lama). Tanpa `omitempty`. JSON lama tanpa field terbaca 0. `cloneWorkspace` (salinan struct) otomatis membawa field.
- `func (s *Store) Touch(id string) (Workspace, error)` di `store.go`: pola sama dengan `Rename` (`s.mu.Lock`, `indexLocked`, `cloneWorkspaces`, ubah salinan, `persistLocked(next, s.ui)`, assign `s.workspaces = next`, return `cloneWorkspace(next[i])`). Id tak dikenal: `fmt.Errorf("%w: workspace %q", ErrNotFound, id)`.
- `touch.go` (file baru): `func registerTouch(srv *ipc.Server, s *Store)` mendaftarkan `workspace.touch`. `ipc.go` hanya bertambah SATU baris `registerTouch(srv, s)` di akhir `Register()`.
- Handler: `decode` params `{ id string }` -> `s.Touch(id)` -> error lewat `mapErr` -> `publish(srv, "updated", ws.ID, &ws)` -> return `ws` (objek `Workspace` penuh, bukan dibungkus).
- RPC `workspace.touch` params `{ "id": string }` -> `Workspace` dengan `lastOpenedAt` = unix ms sekarang. Event: `workspace.changed` `{ kind: "updated", workspaceId, workspace }`.
- `workspace.list` tidak berubah kecuali field baru.
- CLI debug: `shikigamictl workspace touch <id>` (`callAndPrint(c, "workspace.touch", map[string]any{"id": rest[0]})`); perbarui `workspaceUsage` dan `Summary` di `cmd/shikigamictl/cmd_workspace.go`. Argumen salah -> pesan pemakaian.

### 2.2 Frontend (TypeScript)
- `frontend/src/lib/stores/types.ts`: `Workspace.lastOpenedAt?: number`.
- `frontend/src/lib/stores/workspaces.ts` (ekspor juga lewat `lib/stores/index.ts` bila barrel perlu):
  - `export const openWorkspaceIds: Writable<string[]> = writable<string[]>([])` (memori, urutan = urutan tab, TIDAK dipersist).
  - `export async function openWorkspace(id: string): Promise<void>`: tambah ke akhir `openWorkspaceIds` bila belum ada (idempotent), `await selectWorkspace(id)`, lalu `try { const res = await call('workspace.touch', { id }); replaceWorkspace(normalisasi res) } catch {}`. Normalisasi: objek langsung atau `{ workspace }` (seperti `createWorkspace`).
  - `export async function closeWorkspaceTab(id: string): Promise<void>`: hitung urutan sebelum dihapus; keluarkan id; bila id aktif, pilih tetangga (kanan, kalau tidak ada kiri) lewat `selectWorkspace`, atau `activeWorkspaceId = null`, `activeProjectId = null`, `sessions.set([])` bila kosong. Tidak memanggil `workspace.delete`. Menutup tab non-aktif tidak mengubah workspace aktif.
  - `deleteWorkspace(id)`: setelah `workspace.delete`, keluarkan id dari `openWorkspaceIds`; bila aktif, pilih tetangga dari workspace TERBUKA (bukan `rest[0]` seluruh daftar) atau null.
  - `loadWorkspaces()`: tidak lagi memilih `list[0]`. Bila `activeWorkspaceId` tidak ada di daftar baru (termasuk start): `activeWorkspaceId = null`, `activeProjectId = null`, `sessions.set([])`; buang id yang hilang dari `openWorkspaceIds`. Bila workspace aktif masih ada: perilaku lama (validasi `activeProjectId`).
- `lib/stores/init.ts`: handler `workspace.changed` tetap memanggil `loadWorkspaces()`; event `updated` dari touch tidak boleh mengaktifkan workspace atau mengubah `openWorkspaceIds`.
- `modules/persist/index.ts` `run()`: `target = null` selalu. Tetap `applyUi(ui)`; jangan `selectWorkspace`; `await restoreWorkspace(null)`; lanjutkan alur (`enableWrites`, `startWatching`, subscriber `activeWorkspaceId`). `autosave.ts` TIDAK diubah (sudah aman untuk null).
- `modules/openfolder/logic.ts` `newWorkspaceFromFolder(path)`: `createWorkspace(basename(path))` -> `openWorkspace(ws.id)` -> `addProject(path)`; hapus `selectWorkspace` dan `activeWorkspaceId.set` yang tak perlu (impor yang tidak terpakai dibuang).
- `WorkspaceTabs.svelte`: render `$openWorkspaceIds` berurutan ke entri `$workspaces` (lewati yang tak ditemukan); `pick(id)` = `selectWorkspace`; `add()` = `createWorkspace(defaultWorkspaceName($workspaces))` -> `openWorkspace(ws.id)` -> `startEdit(ws.id, ws.name)`; menu konteks: Ganti nama, Tutup, Hapus.

### 2.3 Invarian lintas fitur
- Semua konsumen `activeWorkspace` / `activeWorkspaceId` / `activeProject` aman untuk `null`; `restoreWorkspace(null)` dan `loadSessions()` tanpa workspace menghasilkan state kosong tanpa melempar dan tanpa error konsol.
- Dengan workspace aktif, perilaku dan tampilan identik dengan sebelumnya.
- Tidak ada dependency baru, tidak ada unit test, tidak ada file baru di luar `touch.go`.

### 2.4 Konvensi
- Teks UI dan komentar dalam bahasa Indonesia; identifier kode dalam bahasa Inggris.
- Tabel pesan error UI memakai `errText` dan `say` yang sudah ada di `modules/topbar/util`; nama default workspace via `defaultWorkspaceName`.
- Tanpa library baru. Ikuti gaya file sekitarnya (penanda komentar `// FINAL(...)` yang sudah ada dipertahankan).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `workspace-touch-go` | Field `LastOpenedAt`, `Store.Touch`, RPC `workspace.touch` (+ event `updated`), subperintah ctl `workspace touch`. | `internal/workspace/types.go`, `internal/workspace/store.go`, `internal/workspace/ipc.go`, `internal/workspace/touch.go`, `cmd/shikigamictl/cmd_workspace.go` |
| `open-set-store` | Tipe TS `lastOpenedAt`, store `openWorkspaceIds`, `openWorkspace`, `closeWorkspaceTab`, ubah `loadWorkspaces`/`deleteWorkspace`, `init.ts`, start tanpa workspace aktif di `persist/index.ts`. | `frontend/src/lib/stores/workspaces.ts`, `frontend/src/lib/stores/types.ts`, `frontend/src/lib/stores/index.ts`, `frontend/src/lib/stores/init.ts`, `frontend/src/modules/persist/index.ts` |
| `null-workspace-safety` | Audit dan guard null (optional chaining, early return, `disabled`, `{#if}`) pada konsumen `activeWorkspace*`/`activeProject`; tanpa mengubah perilaku saat workspace ada. | `frontend/src/modules/sidebar-left`, `frontend/src/modules/sidebar-right`, `frontend/src/modules/workspace`, `frontend/src/modules/files`, `frontend/src/modules/lark/source.ts`, `frontend/src/modules/grimoire`, `frontend/src/modules/topbar/CenterSummary.svelte`, `frontend/src/modules/topbar/SessionCounter.svelte`, `frontend/src/modules/layout`, `frontend/src/lib/stores/groups.ts`, `frontend/src/lib/stores/sessions.ts` |
| `tabs-close-menu` | `WorkspaceTabs.svelte` hanya tab terbuka + menu Ganti nama/Tutup/Hapus; `newWorkspaceFromFolder` memakai `openWorkspace`. Fitur penutup epic (wiring). | `frontend/src/modules/topbar/WorkspaceTabs.svelte`, `frontend/src/modules/openfolder/logic.ts` |

Ketergantungan: `workspace-touch-go`, `open-set-store`, `null-workspace-safety` saling paralel (tidak berbagi file). `tabs-close-menu` setelah `open-set-store` (wajib) dan `workspace-touch-go` (opsional untuk build, touch best-effort); boleh paralel dengan `null-workspace-safety`.

Catatan `null-workspace-safety`: grep menunjukkan konsumen `activeWorkspace*` juga ada di luar ownedPaths (mis. `modules/lark/*` selain `source.ts`, `modules/mention`, `modules/palette`, `modules/runs`, `modules/shell`, `modules/thread`, `modules/persist/autosave.ts`/`DeadSession.svelte`, `modules/files` sudah termasuk). Agent TIDAK mengedit file itu; bila ditemukan crash null di sana, catat di laporan fitur sebagai temuan untuk tiket 02/epic lain. Perbaiki sebisanya hanya lewat ownedPaths.

## 4. Foundation

Tidak ada. Alasan: tidak ada skeleton paket atau tipe bersama yang harus ada sebelum paralel; kontrak di bagian 2 cukup. `hasFoundation=false`, file `00-foundation.md` tidak dibuat.

## 5. Wiring checklist

Penyambungan dilakukan oleh `tabs-close-menu` (gelombang B). Setelah semua fitur merge, pastikan:
- [ ] `WorkspaceTabs.svelte` (sudah dirender di topbar; tanpa registrasi baru) membaca `openWorkspaceIds` dan memanggil `openWorkspace` / `closeWorkspaceTab`.
- [ ] `newWorkspaceFromFolder` memakai `openWorkspace`.
- [ ] `openWorkspace` memanggil RPC `workspace.touch`; `ipc.go` memanggil `registerTouch(srv, s)`.
- [ ] `persist/index.ts` dan `loadWorkspaces` memastikan start tanpa workspace aktif.
- [ ] Tidak ada perubahan di `App.svelte` atau `modules/welcome` (milik tiket 02).
- [ ] Daemon dibangun ulang agar `workspace.touch` tersedia (bila belum, app tetap jalan karena touch best-effort).
- [ ] Verifikasi perilaku akhir: start dengan topbar hanya "+", tanpa error konsol; "+" membuat, membuka, dan masuk mode ganti nama; menu tab Ganti nama/Tutup/Hapus; Tutup tanpa konfirmasi dan data tetap di `workspace.list`; Hapus tetap konfirmasi; tutup tab terakhir -> topbar kosong; `shikigamictl workspace touch <id>` mengisi `lastOpenedAt`.

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- Backend: `go build ./...` (root repo).
- Frontend: `npm run build` di `frontend/` (opsional tambahan `npm run check`). Gunakan `wails build` hanya bila memang dipakai proyek.
- Fitur Go cukup `go build ./...`; fitur frontend cukup build frontend. `tabs-close-menu` menjalankan keduanya.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, `App.svelte`, `modules/welcome`, atau file di luar daftar "Path yang dimiliki" tiket.
- `modules/persist/index.ts` hanya diubah oleh `open-set-store`; `autosave.ts` tidak diubah siapa pun.
- `ipc.go` hanya bertambah satu baris pemanggil; handler di `touch.go`.
- `null-workspace-safety`: hanya perbaikan defensif null; tidak mengubah perilaku saat workspace ada.
- Jangan menambah dependency, unit test, atau file di luar ownedPaths. Jangan commit/merge di luar alur implement-feature.
