# Plan: Daemon: skema v2 (grup + layout) dan IPC layout

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-06-workspace-persist.md`
- Fitur: `schema-v2-daemon`
- ownedPaths: `internal/workspace/`, `cmd/shikigamid/wire_workspace.go`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; fitur ini tidak bergantung pada fitur lain)

## Tujuan

Naikkan `workspaces.json` ke versi 2 (workspace -> grup -> layout, plus blok `ui`) dengan migrasi otomatis dari v1, lalu sediakan `Store.GetLayout/SetLayout/GetUI/SetUI` dan method IPC `workspace.layout.get/set`, `workspace.ui.get/set`, event `workspace.layoutChanged`. Skema lengkap (tipe, aturan, migrasi, tabel IPC, kode error) sudah ditetapkan di `overview.md` bagian 2.2 dan 2.3; plan ini tidak mengulanginya, hanya langkah implementasinya. Semua dikerjakan di paket `workspace` (Go, tanpa dependency baru, tanpa unit test).

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `internal/workspace/config.go`, `types.go`, `store.go`, `ipc.go` (boleh menambah file baru di folder ini, mis. `layout.go`)
- `cmd/shikigamid/wire_workspace.go` (hanya bila tanda tangan `Register`/`NewStore` berubah; rencananya TIDAK berubah, jadi file ini tidak diubah)

Jangan sentuh `frontend/`, `app/`, `internal/appconfig`, `internal/ipc`, `docs/` (termasuk `overview.md`), atau file wiring/path fitur lain.

## Kontrak

Disediakan (rujuk `overview.md` 2.2, 2.3):
- Tipe Go: `Group`, `SidebarState`, `UIState` (di `types.go`); `Workspace.Groups []Group` (`json:"groups"`), `Workspace.ActiveGroupID string` (`json:"activeGroupId"`); `workspacesDoc.UI UIState` (`json:"ui"`).
- Konstanta: `workspacesFileVersion = 2`, `configFileVersion = 1` (menggantikan `fileVersion`).
- Store API baru (semua mengembalikan salinan dalam):
  - `func (s *Store) GetLayout(wsID, groupID string) (resolvedGroupID string, layout json.RawMessage, err error)`
  - `func (s *Store) SetLayout(wsID, groupID string, layout json.RawMessage) (resolvedGroupID string, err error)`
  - `func (s *Store) GetUI() UIState`
  - `func (s *Store) SetUI(p UIPatch) (UIState, error)`
  - `type UIPatch struct { LeftSidebar *SidebarState; RightSidebar *SidebarState; ActiveWorkspaceID *string }`
- IPC: `workspace.layout.get`, `workspace.layout.set`, `workspace.ui.get`, `workspace.ui.set`; event `workspace.layoutChanged {workspaceId, groupId}`; `workspace.list/create/rename/addProject/removeProject` ikut mengembalikan `groups` + `activeGroupId` (otomatis lewat tipe `Workspace`).

Dipakai: `writeJSONAtomic`, `newID`, `ErrNotFound/ErrInvalid/ErrConflict`, `mapErr`, `decode`, `publish`, `ipc.Server.Register/Publish`, `ipc.NewError` (semua sudah ada, tidak diubah).

## Langkah

1. **`types.go`**: tambah `import "encoding/json"`. Tambah `Group`, `SidebarState`, `UIState` persis seperti `overview.md` 2.2 (`Layout json.RawMessage` TANPA `omitempty`; `ActiveTab` `omitempty`). Tambah ke `Workspace`: `Groups []Group \`json:"groups"\`` dan `ActiveGroupID string \`json:"activeGroupId"\``. Tambah `UIPatch` (di atas) dan fungsi `defaultUI() UIState` (`LeftSidebar{true,260,""}`, `RightSidebar{true,280,"files"}`, `ActiveWorkspaceID ""`) dan `newDefaultGroup() Group` (`ID: newID(), Name: "Utama", Order: 0, Layout: nil`).

2. **`config.go` konstanta**: ganti `fileVersion = 1` dengan `workspacesFileVersion = 2` dan `configFileVersion = 1`; tambah `workspacesBackupSuffix = ".v1.bak"`. `SetGlobalConfig` memakai `configFileVersion`. `workspacesDoc` tambah `UI UIState \`json:"ui"\``. Pastikan `grep -n fileVersion internal/workspace` tidak menyisakan nama lama.

3. **`config.go` `loadWorkspaces`**: ubah tanda tangan menjadi `loadWorkspaces(path string) ([]Workspace, UIState, error)` (satu-satunya pemanggil: `NewStore`; sesuaikan). Alur:
   - File tidak ada: `[]Workspace{}`, `defaultUI()`, nil (tidak menulis apa pun).
   - Baca bytes `data`, `json.Unmarshal` ke `workspacesDoc`; error korup tetap seperti sekarang.
   - `doc.Version > workspacesFileVersion`: return error jelas, mis. `fmt.Errorf("workspaces.json versi %d tidak didukung (maksimum %d); file tidak diubah", ...)`. Jangan menulis, jangan membuat cadangan.
   - Normalisasi di memori (selalu, tanpa menulis): `Workspaces` nil -> `[]`; tiap workspace `Projects` nil -> `[]`; `Groups` kosong -> `[]Group{newDefaultGroup()}`; `Layout` bernilai literal `null`/kosong -> `nil`; `ActiveGroupID` kosong atau tidak ada di `Groups` -> `Groups[0].ID`. UI: bila `LeftSidebar.Width == 0` pakai default kiri; bila `RightSidebar.Width == 0` pakai default kanan; `RightSidebar.ActiveTab == ""` -> `"files"`; `LeftSidebar.ActiveTab` dikosongkan; `ActiveWorkspaceID` yang tidak cocok dengan workspace mana pun -> `""`.
   - Bila `doc.Version < 2` (v1 atau 0): untuk v1 normalisasi di atas sudah memberi grup "Utama"; lalu setel `ui = defaultUI()` dengan `ActiveWorkspaceID` = ID workspace pertama (atau kosong). Lalu migrasi tulis: (a) cadangan `path + workspacesBackupSuffix`: cek `os.Stat`; bila belum ada, salin bytes asli `data` secara atomik (tulis temp di folder yang sama lalu `os.Link`/`os.Rename` hanya bila target belum ada; cukup `os.OpenFile(..., O_WRONLY|O_CREATE|O_EXCL, 0o600)` + `Write` + `Sync` + `Close`, abaikan `os.ErrExist`); gagal cadangan selain ErrExist -> return error dan JANGAN lanjut menulis v2; (b) `writeJSONAtomic(path, workspacesDoc{Version: workspacesFileVersion, Workspaces: ws, UI: ui})`.
   - `doc.Version == 2`: tidak menulis apa pun (idempoten, tidak ada cadangan baru).
   - Return `ws, ui, nil`.

4. **`store.go` struct dan `NewStore`**: tambah field `ui UIState` di `Store`; `NewStore` memanggil `s.workspaces, s.ui, err = loadWorkspaces(s.wsPath)`.

5. **`store.go` clone**: `cloneWorkspace` menyalin `Projects` dan juga `Groups`: `make([]Group, len)`; tiap `Layout` disalin byte (`append(json.RawMessage(nil), g.Layout...)`; `nil` tetap `nil`). `Groups` hasil klon tidak pernah nil (bila sumber nil -> `[]Group{}`). Tambah `cloneUI` bila perlu (UIState berupa nilai, `c := s.ui` cukup).

6. **`store.go` persist**: ganti `persistWorkspacesLocked(ws)` menjadi `persistLocked(ws []Workspace, ui UIState) error` yang menulis `workspacesDoc{Version: workspacesFileVersion, Workspaces: ws, UI: ui}` lewat `writeJSONAtomic`. Semua mutator yang ada (`Create`, `Rename`, `AddProject`, `RemoveProject`, `SetWorkspaceConfig` di `config.go`) memanggil `s.persistLocked(next, s.ui)`. Pola salin-lalu-tukar tetap: tulis dulu, baru `s.workspaces = next`.

7. **`store.go` `Create`**: workspace baru `Workspace{ID: newID(), Name: name, Projects: []Project{}, Groups: []Group{g}, ActiveGroupID: g.ID}` dengan `g := newDefaultGroup()`.

8. **`store.go` `Delete`**: bila `s.ui.ActiveWorkspaceID == id`, `nextUI := s.ui; nextUI.ActiveWorkspaceID = ""`; panggil `persistLocked(next, nextUI)` (satu tulisan atomik untuk daftar + ui); setelah sukses `s.workspaces = next; s.ui = nextUI`.

9. **`layout.go` (file baru)**: 
   - `const maxLayoutBytes = 1 << 20`.
   - `func validateLayout(raw json.RawMessage) (json.RawMessage, error)`: `t := bytes.TrimSpace(raw)`; kosong atau `null` -> `nil, nil`; `len(t) > maxLayoutBytes` -> `ErrInvalid` ("layout terlalu besar"); `!json.Valid(t)` -> `ErrInvalid`; byte pertama bukan `'{'` -> `ErrInvalid` ("layout harus objek JSON atau null"); kembalikan salinan (`append(json.RawMessage(nil), t...)`). Dipanggil SEBELUM mengambil lock.
   - `func (s *Store) GetLayout(wsID, groupID string) (string, json.RawMessage, error)`: `RLock`; cari workspace (`ErrNotFound`); `groupID == ""` -> `ActiveGroupID`; cari grup (`ErrNotFound: grup %q`); kembalikan ID grup + salinan byte layout (nil bila kosong). Salin sebelum `RUnlock` (defer cukup).
   - `func (s *Store) SetLayout(wsID, groupID string, layout json.RawMessage) (string, error)`: `v, err := validateLayout(layout)` dulu; `Lock`; cari workspace + grup; bangun `next := append([]Workspace(nil), s.workspaces...)` (slice dangkal; byte Layout tidak pernah dimutasi di tempat jadi aman dibagi), salin hanya `Groups` workspace target (`nw := next[i]; nw.Groups = append([]Group(nil), nw.Groups...); nw.Groups[j].Layout = v; next[i] = nw`); `persistLocked(next, s.ui)`; sukses -> `s.workspaces = next`. Lock hanya selama bangun salinan + tulis + tukar; tidak ada pekerjaan lain di dalamnya. Return ID grup terpakai.
   - `func (s *Store) GetUI() UIState`: `RLock`, kembalikan `s.ui`.
   - `func (s *Store) SetUI(p UIPatch) (UIState, error)`: validasi sebelum lock: `Width < 0` -> `ErrInvalid`. `Lock`; `nu := s.ui`; `LeftSidebar != nil` -> `nu.LeftSidebar = *p.LeftSidebar` dengan `ActiveTab = ""`; `RightSidebar != nil` -> ganti utuh, `ActiveTab` kosong -> pertahankan nilai lama (atau `"files"` bila lama kosong); `ActiveWorkspaceID != nil` -> bila `*v != ""` dan `indexLocked(*v) < 0` -> `ErrInvalid` ("workspace %q tidak dikenal"), selain itu set. `persistLocked(s.workspaces, nu)` (tanpa menyalin workspaces; tidak dimutasi); sukses -> `s.ui = nu`; return `nu`.

10. **`ipc.go`**: 
    - Helper `publishLayout(srv, wsID, groupID)` -> `srv.Publish("workspace.layoutChanged", map[string]any{"workspaceId": wsID, "groupId": groupID})`.
    - Di `Register` tambah:
      - `workspace.layout.get`: params `{workspaceId, groupId}` (string); panggil `s.GetLayout`; hasil `map[string]any{"workspaceId": ..., "groupId": gid, "layout": layout}` (`layout` bertipe `json.RawMessage`; nil ter-marshal sebagai `null`).
      - `workspace.layout.set`: params `{workspaceId string, groupId string, layout json.RawMessage}`; `s.SetLayout`; `publishLayout` (JANGAN panggil `publish(... "workspace.changed" ...)`); hasil `map[string]bool{"ok": true}`. Error lewat `mapErr`.
      - `workspace.ui.get`: `decode` params kosong; hasil `s.GetUI()`.
      - `workspace.ui.set`: params berupa `UIPatch` dengan tag json `leftSidebar`, `rightSidebar`, `activeWorkspaceId` (beri tag json pada `UIPatch` di `types.go`, semua pointer `omitempty`); `s.SetUI`; hasil `UIState`; tidak memublikasi event (opsional, tidak diwajibkan).
    - `workspace.delete`, `create`, dll tidak berubah selain yang sudah otomatis (grup di tipe `Workspace`; pembersihan ui ada di `Store.Delete`).
    - Perbarui komentar dokumentasi `Register`.

11. **`cmd/shikigamid/wire_workspace.go`**: tidak ada perubahan; `workspace.NewStore(dir)` dan `workspace.Register(srv, store)` tetap kompatibel. Periksa saja. Bila ada error `NewStore` karena versi > 2, `logWorkspaceErr` sudah mencatatnya dan modul dilewati (file tidak tersentuh).

12. **Guard**: jalankan `gofmt -l internal/workspace` (harus kosong), `go vet ./internal/workspace/...`, lalu `go build ./...`. Opsional uji manual cepat tanpa menambah file repo: data dir sementara di `$TMPDIR` berisi `workspaces.json` v1 dan jalankan `NewStore` lewat daemon; jangan membuat file uji di dalam repo.

## Perubahan

- Diubah: `internal/workspace/types.go`, `internal/workspace/config.go`, `internal/workspace/store.go`, `internal/workspace/ipc.go`
- Dibuat: `internal/workspace/layout.go`
- Tidak diubah: `cmd/shikigamid/wire_workspace.go` (hanya diperiksa)

## Wiring

Fitur ini tidak menyambungkan apa pun sendiri. Yang diekspos:
- `workspace.Register(srv, store)` (tanda tangan tidak berubah) kini mendaftarkan `workspace.layout.get/set` dan `workspace.ui.get/set`; sudah terpasang lewat `wire_workspace.go` yang ada.
- Event `workspace.layoutChanged {workspaceId, groupId}` untuk frontend `autosave-restore`.
- Respons `workspace.list` (dan method workspace lain) kini memuat `groups` (dengan `layout`) dan `activeGroupId`.
Langkah Wiring (`overview.md` bagian 5, langkah 1) hanya memeriksa dan menguji migrasi manual terhadap `workspaces.json` v1 contoh.

## Selesai bila

- `go build ./...` sukses (guard fitur ini); `gofmt -l internal/workspace` kosong; tanpa unit test.
- Tidak ada referensi `fileVersion` lama; `config.json` tetap menulis versi 1, `workspaces.json` versi 2.
- `loadWorkspaces` untuk v1 menghasilkan grup "Utama" + `activeGroupId` + `ui` default, menulis `workspaces.json.v1.bak` hanya sekali dan file versi 2; membaca ulang v2 tidak menulis apa pun; `version` > 2 menghasilkan error tanpa mengubah file/cadangan.
- `Create` membuat grup "Utama" dan `activeGroupId`; `Delete` workspace aktif mengosongkan `ui.activeWorkspaceId` dalam satu tulisan atomik.
- `layout.set` menolak non-objek/> 1 MiB (`invalid`), workspace/grup tak ada (`not_found`), `groupId` kosong memakai grup aktif, `null` menghapus; hanya `workspace.layoutChanged` yang dipublikasikan.
- `ui.set` patch parsial menggabung per field, `activeWorkspaceId` tak dikenal ditolak (kosong diizinkan).
- Semua akses lewat `Store.mu`; hasil get adalah salinan dalam (Groups + Layout); semua tulis lewat `writeJSONAtomic`.

Asumsi: batas ukuran frame IPC di `internal/ipc` tidak diubah (di luar path milik); validasi 1 MiB di sini adalah batas logis layout.
