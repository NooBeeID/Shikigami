# Plan: Daemon: grup, keanggotaan sesi, dan method IPC

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-07-session-groups.md` (fitur `group-daemon`)
- Rujukan kontrak: `docs/plans/shikigami-v2-ui-ticket-07-session-groups/overview.md` bagian 2.2 (skema data, tabel method IPC, `GroupsState`, aturan nama, event) dan bagian 6 (build guard). Plan ini mandiri; overview hanya dirujuk untuk kontrak, tidak diulang.
- Spec: `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md` (bagian 5b)
- ownedPaths:
  - `internal/workspace/groups.go` (baru)
  - `internal/workspace/ipc_groups.go` (baru)
  - `internal/workspace/types.go` (perluasan aditif)
  - `internal/workspace/store.go` (perluasan aditif)
  - `internal/workspace/config.go` (perluasan aditif)
  - `internal/workspace/ipc.go` (perluasan aditif, satu baris)
- Depends on foundation: **tidak**. Foundation hanya frontend; fitur ini berdiri sendiri di sisi Go. Frontend (`groups-store`) hanya bergantung pada kontrak IPC.

## Tujuan

Menjadikan daemon dan `workspaces.json` (tetap `version: 2`) satu-satunya sumber kebenaran grup sesi. Menambah penanda grup bawaan (`Group.Main`), peta keanggotaan sesi (`Workspace.Membership`), normalisasi saat load untuk file v2 lama, enam method `Store` untuk mengelola grup dan keanggotaan, enam method IPC `workspace.group.*`, dan event `workspace.groupsChanged`. Semua perubahan aditif: method dan event layout/ui yang ada (`workspace.layout.get/set`, `workspace.ui.get/set`, `workspace.changed`, `workspace.layoutChanged`) tidak berubah perilaku.

## Path yang dimiliki

HANYA path berikut yang boleh dibuat atau diubah:

- `internal/workspace/groups.go` (baru)
- `internal/workspace/ipc_groups.go` (baru)
- `internal/workspace/types.go` (hanya tambahan: field `Membership`, field `Main`, `Main: true` di `newDefaultGroup`)
- `internal/workspace/store.go` (hanya `cloneWorkspace` dan `Store.Create`)
- `internal/workspace/config.go` (hanya satu pemanggilan `normalizeGroups` di `loadWorkspaces`)
- `internal/workspace/ipc.go` (hanya satu baris `registerGroupMethods(srv, s)` di akhir `Register`)

Jangan menyentuh `internal/workspace/layout.go` (cukup dipakai: `findGroup`), `cmd/` (termasuk `cmd/shikigamid/wire_workspace.go`), `app/`, `frontend/`, maupun file wiring dan path fitur lain (`groups-store`, `group-tabs-ui`). Jangan menaikkan `workspacesFileVersion`. Jangan menambah dependency Go, goroutine, timer, atau polling. Jangan menyentuh flag izin agent.

## Kontrak

### Disediakan

Tipe dan field (Go, paket `workspace`):

```go
// types.go (aditif)
type Workspace struct {
    // ...field lama tidak berubah...
    Membership map[string]string `json:"membership"` // sessionId -> groupId; non-nil saat keluar
}
type Group struct {
    // ...field lama tidak berubah...
    Main bool `json:"main,omitempty"` // grup bawaan; tepat satu per workspace
}

// groups.go
type GroupInfo struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Order int    `json:"order"`
    Main  bool   `json:"main,omitempty"`
}
type GroupsState struct {
    WorkspaceID   string            `json:"workspaceId"`
    ActiveGroupID string            `json:"activeGroupId"`
    Groups        []GroupInfo       `json:"groups"`     // urut Order, non-nil
    Membership    map[string]string `json:"membership"` // non-nil
}
```

Method `Store` (semua mengambil `wsID` pertama; galat `ErrNotFound`/`ErrInvalid`/`ErrConflict` bungkus dengan `fmt.Errorf("%w: ...", ...)`):

```go
func (s *Store) CreateGroup(wsID, name string) (GroupsState, string, error)           // state, groupId baru
func (s *Store) RenameGroup(wsID, groupID, name string) (GroupsState, bool, error)    // bool = berubah
func (s *Store) DeleteGroup(wsID, groupID string) (GroupsState, error)
func (s *Store) ReorderGroup(wsID, groupID string, toIndex int) (GroupsState, bool, error)
func (s *Store) SetActiveGroup(wsID, groupID string) (GroupsState, bool, error)
func (s *Store) MoveSession(wsID, sessionID, toGroupID string) (GroupsState, bool, error)
```

Fungsi paket: `normalizeGroups(w *Workspace)` (internal, dipanggil `loadWorkspaces`), `registerGroupMethods(srv *ipc.Server, s *Store)`, `publishGroups(srv *ipc.Server, kind string, st GroupsState)`.

Method IPC (params, efek, hasil persis tabel overview 2.2): `workspace.group.create {workspaceId, name?}` (hasil `GroupsState` + `groupId`), `workspace.group.rename {workspaceId, groupId, name}`, `workspace.group.delete {workspaceId, groupId}`, `workspace.group.reorder {workspaceId, groupId, toIndex}`, `workspace.group.setActive {workspaceId, groupId}`, `workspace.group.moveSession {workspaceId, sessionId, toGroupId}`. Hasil selain create = `GroupsState` (objek datar: `workspaceId`, `activeGroupId`, `groups`, `membership`).

Event: `workspace.groupsChanged` payload `{kind, workspaceId, activeGroupId, groups, membership}` dengan `kind` salah satu `groupCreated`, `groupRenamed`, `groupDeleted`, `groupReordered`, `groupActivated`, `sessionMoved`. Dipublikasikan HANYA setelah persist berhasil dan tidak untuk no-op.

`workspace.list` otomatis mengeluarkan `membership` (objek, tidak pernah `null`) dan `main` pada grup bawaan karena `Workspace`/`Group` di-marshal langsung.

### Dipakai (sudah ada, tidak diubah perilakunya)

`Store` (`mu`, `workspaces`, `ui`, `indexLocked`, `persistLocked`), `cloneWorkspace`/`cloneWorkspaces`, `findGroup` (`layout.go`), `newID`, `newDefaultGroup`, sentinel `ErrNotFound`/`ErrInvalid`/`ErrConflict`, `mapErr`, `decode` (`ipc.go`), `ipc.Server.Register`/`Publish`.

## Langkah

Kerjakan berurutan. Baca dulu `internal/workspace/types.go`, `store.go`, `config.go`, `ipc.go`, `layout.go` untuk memastikan nama asli (plan ditulis dari kode saat ini).

### 1. `types.go` (aditif)

1. Di struct `Workspace`, setelah `ActiveGroupID`, tambah:
   `Membership map[string]string `json:"membership"` // sessionId -> groupId; non-nil saat keluar`
2. Di struct `Group`, antara `Order` dan `Layout`, tambah:
   `Main bool `json:"main,omitempty"` // grup bawaan "Utama"; tepat satu per workspace`
3. `newDefaultGroup()` menjadi `Group{ID: newID(), Name: "Utama", Order: 0, Main: true}`.

Catatan: `Membership` ikut tertulis di `workspaces.json` (tanpa `omitempty`), tetapi selalu dikosongkan saat load, jadi aman.

### 2. `store.go` (aditif)

1. `cloneWorkspace`: setelah penyalinan `Groups`, tambah salinan map selalu non-nil:
   ```go
   c.Membership = make(map[string]string, len(w.Membership))
   for k, v := range w.Membership {
       c.Membership[k] = v
   }
   ```
2. `Store.Create`: inisialisasi `Membership: map[string]string{}` pada literal `Workspace`.
3. Jangan ubah hal lain di berkas ini. (Catatan: `SetLayout` di `layout.go` menyalin `Workspace` dangkal sehingga map `Membership` dipakai bersama antar generasi state; aman karena map hanya dimutasi pada salinan hasil `cloneWorkspaces` di `groups.go`, jangan pernah memutasi `s.workspaces[i].Membership` langsung.)

### 3. `groups.go` (baru)

Paket `workspace`; impor `fmt`, `sort`, `strconv`, `strings`, `unicode/utf8`.

1. Konstanta: `const maxGroupNameRunes = 40`, `const mainGroupName = "Utama"`.
2. Tipe `GroupInfo` dan `GroupsState` persis seperti bagian Kontrak.
3. `normalizeGroups(w *Workspace)`:
   - Bila `len(w.Groups) > 0`: `sort.SliceStable(w.Groups, ...)` menurut `Order` naik; lalu `w.Groups[i].Order = i`.
   - Pilih induk `Main`: kumpulkan indeks grup ber-`Main`; bila ada, pertahankan yang pertama (order terkecil karena sudah terurut) dan kosongkan `Main` pada lainnya. Bila tidak ada: cari grup pertama (urut order) dengan `strings.EqualFold(strings.TrimSpace(name), mainGroupName)`; bila tidak ada, indeks 0. Set `Main = true` pada yang terpilih dan `false` pada lainnya. Tidak pernah membuat grup baru (bila `Groups` kosong lewati bagian ini tanpa panik).
   - `w.Membership = map[string]string{}` (selalu baru, kosong; entri file lama dibuang).
4. Pembantu murni (tanpa lock):
   - `mainGroupID(w Workspace) string`: ID grup ber-`Main`; fallback `w.Groups[0].ID`.
   - `stateOf(w Workspace) GroupsState`: salin `Groups` ke `[]GroupInfo` (non-nil, `make(..., 0, n)`), salin `Membership` ke map baru non-nil, isi `WorkspaceID: w.ID`, `ActiveGroupID: w.ActiveGroupID`.
   - `cleanName(raw string) (string, error)`: `strings.TrimSpace`; bila `utf8.RuneCountInString(t) > maxGroupNameRunes` kembalikan `fmt.Errorf("%w: nama grup maksimal %d karakter", ErrInvalid, maxGroupNameRunes)` (tidak dipotong); selain itu `t, nil`. Tidak menolak kosong di sini (penolakan dilakukan pemanggil).
   - `nameTaken(gs []Group, name, exceptID string) bool`: true bila ada grup dengan `ID != exceptID` dan `strings.EqualFold(g.Name, name)`.
   - `uniqueName(gs []Group, name, exceptID string) string`: bila `!nameTaken` kembalikan `name`; selain itu `for n := 2; ; n++ { c := name + " " + strconv.Itoa(n); if !nameTaken(gs, c, exceptID) { return c } }` (hasil boleh melewati 40).
   - `defaultName(gs []Group) string`: `n := len(gs) + 1`; ulangi `"Grup " + strconv.Itoa(n)` dengan `n++` selama `nameTaken(gs, c, "")`.
   - `pruneMembership(w *Workspace)`: hapus entri `Membership` yang nilainya bukan ID grup yang ada (pakai `findGroup(*w, id) < 0`). Menghapus saat iterasi `range` map aman di Go.
5. Pembantu mutasi tunggal (pola kloning-ubah-persist-tukar), agar semua method seragam:
   ```go
   // mutateGroups memegang s.mu.Lock, mengkloning s.workspaces, memanggil fn pada
   // salinan workspace target, dan hanya bila fn mengembalikan changed=true
   // mempersist lalu menukar s.workspaces. Persist gagal -> state memori tidak berubah.
   func (s *Store) mutateGroups(wsID string, fn func(w *Workspace) (bool, error)) (GroupsState, bool, error)
   ```
   Isi: `s.mu.Lock(); defer s.mu.Unlock()`; `i := s.indexLocked(wsID)`; bila `<0` kembalikan `ErrNotFound` (`"%w: workspace %q"`); `next := cloneWorkspaces(s.workspaces)`; `changed, err := fn(&next[i])`; bila `err != nil` kembalikan `GroupsState{}, false, err`; bila `!changed` kembalikan `stateOf(s.workspaces[i]), false, nil`; `if err := s.persistLocked(next, s.ui); err != nil { return GroupsState{}, false, err }`; `s.workspaces = next`; kembalikan `stateOf(next[i]), true, nil`.
6. Method `Store` (semua lewat `mutateGroups`):
   - `CreateGroup(wsID, name string)`: di closure: `t, err := cleanName(name)`; bila `t == ""` pakai `defaultName(w.Groups)`, selain itu `uniqueName(w.Groups, t, "")`; `g := Group{ID: newID(), Name: final, Order: len(w.Groups)}` (Layout nil, Main false); `w.Groups = append(w.Groups, g)`; `pruneMembership(w)`; tangkap `g.ID` ke variabel luar; changed=true. Tidak mengubah `ActiveGroupID`. Kembalikan `(state, id, err)`.
   - `RenameGroup(wsID, groupID, name)`: `t, err := cleanName(name)`; bila `t == ""` -> `ErrInvalid` ("nama grup kosong"); `j := findGroup(*w, groupID)`, `<0` -> `ErrNotFound` (`"%w: grup %q"`); `final := uniqueName(w.Groups, t, groupID)`; bila `final == w.Groups[j].Name` kembalikan `false, nil` (no-op); set nama; `pruneMembership(w)`; changed=true. Boleh untuk grup `Main`. Catatan: rename "utama" -> "Utama" (beda huruf besar/kecil, grup sama) tidak bentrok karena `exceptID`; karena `final != Name` secara string, tetap dianggap berubah.
   - `DeleteGroup(wsID, groupID)`: `j` tidak ada -> `ErrNotFound`; `w.Groups[j].Main` -> `fmt.Errorf("%w: grup bawaan tidak bisa dihapus", ErrConflict)` (state tidak berubah karena closure mengembalikan error sebelum mutasi); `mainID := mainGroupID(*w)`; untuk setiap `k, v` di `w.Membership` bila `v == groupID` set `w.Membership[k] = mainID` (menetapkan nilai pada kunci yang ada saat iterasi aman); hapus grup dari slice (`append(w.Groups[:j], w.Groups[j+1:]...)`, aman karena `w` adalah salinan hasil clone); nomor ulang `Order = i`; bila `w.ActiveGroupID == groupID` set ke `mainID`; `pruneMembership(w)`; changed=true. Method mengembalikan `(state, err)` (buang bool). Layout grup ikut hilang; tidak ada pemanggilan ke sesi.
   - `ReorderGroup(wsID, groupID, toIndex)`: `from := findGroup(*w, groupID)`, `<0` -> `ErrNotFound`; jepit `toIndex` ke `0..len-1`; bila `toIndex == from` -> `false, nil`; ambil grup, hapus dari slice, sisipkan di `toIndex` (bangun slice baru agar tidak ada aliasing); nomor ulang `Order = i`; changed=true.
   - `SetActiveGroup(wsID, groupID)`: grup tidak ada -> `ErrNotFound`; bila `w.ActiveGroupID == groupID` -> `false, nil`; set; changed=true. Tidak menyentuh layout.
   - `MoveSession(wsID, sessionID, toGroupID)`: `strings.TrimSpace(sessionID) == ""` -> `ErrInvalid`; `findGroup(*w, toGroupID) < 0` -> `ErrNotFound`; hitung grup efektif saat ini: `cur, ok := w.Membership[sessionID]`; bila `!ok || findGroup(*w, cur) < 0` maka `cur = mainGroupID(*w)`; bila `cur == toGroupID` -> `false, nil` (no-op; jangan menambah entri); selain itu `w.Membership[sessionID] = toGroupID`; `pruneMembership(w)`; changed=true. Keberadaan sesi tidak diperiksa. `w.Membership` dijamin non-nil karena `cloneWorkspace`.
   - Urutan validasi: workspace (di `mutateGroups`), lalu parameter/grup di closure, sehingga ID workspace tak dikenal selalu `ErrNotFound`.

### 4. `config.go` (aditif)

Di `loadWorkspaces`, di akhir badan loop `for i := range ws { ... }` (setelah blok pengecekan `ActiveGroupID` yang ada), tambah satu baris: `normalizeGroups(w)`. Jangan mengubah urutan logika lain, jangan menulis ulang file untuk versi >= 2 (penulisan ulang hanya terjadi pada jalur migrasi v1 yang sudah ada), jangan mengubah `workspacesFileVersion`.

### 5. `ipc_groups.go` (baru)

Paket `workspace`; impor `context`, `encoding/json`, `strings` bila perlu, dan `github.com/NooBeeID/shikigami/internal/ipc`.

1. Tipe event dan hasil create (struct embed meratakan field saat di-marshal):
   ```go
   type groupsEvent struct {
       Kind string `json:"kind"`
       GroupsState
   }
   type createGroupResult struct {
       GroupsState
       GroupID string `json:"groupId"`
   }
   ```
2. `publishGroups(srv *ipc.Server, kind string, st GroupsState)`: `srv.Publish("workspace.groupsChanged", groupsEvent{Kind: kind, GroupsState: st})`.
3. `registerGroupMethods(srv *ipc.Server, s *Store)`: daftarkan enam method dengan pola persis `ipc.go` (`var p struct{...}`, `decode(params, &p)`, panggil Store, `mapErr(err)`), signature handler `func(ctx context.Context, params json.RawMessage) (any, error)`:
   - `workspace.group.create`: params `{WorkspaceID `workspaceId`, Name `name`}`; `st, id, err := s.CreateGroup(...)`; `publishGroups(srv, "groupCreated", st)`; kembalikan `createGroupResult{GroupsState: st, GroupID: id}`.
   - `workspace.group.rename`: params `{WorkspaceID, GroupID `groupId`, Name}`; `st, changed, err`; bila `changed` publish `"groupRenamed"`; kembalikan `st`.
   - `workspace.group.delete`: params `{WorkspaceID, GroupID}`; `st, err`; publish `"groupDeleted"`; kembalikan `st`.
   - `workspace.group.reorder`: params `{WorkspaceID, GroupID, ToIndex int `toIndex`}`; publish `"groupReordered"` bila changed.
   - `workspace.group.setActive`: params `{WorkspaceID, GroupID}`; publish `"groupActivated"` bila changed.
   - `workspace.group.moveSession`: params `{WorkspaceID, SessionID `sessionId`, ToGroupID `toGroupId`}`; publish `"sessionMoved"` bila changed.
   - Setiap handler: galat `Store` -> `return nil, mapErr(err)` (tanpa publish). Publish hanya setelah Store sukses (persist sudah berhasil).

### 6. `ipc.go` (satu baris)

Di akhir fungsi `Register`, sebagai pernyataan terakhir (setelah blok `config.set`), tambah tepat satu baris: `registerGroupMethods(srv, s)`. Tidak ada perubahan lain di `ipc.go`. `cmd/shikigamid/wire_workspace.go` tidak diubah karena sudah memanggil `workspace.Register`.

### 7. Verifikasi

1. `gofmt -l internal/workspace` harus kosong; `go vet ./internal/workspace/...` bersih.
2. `go build ./...` lulus.
3. Telaah mandiri terhadap invarian: (a) tidak ada tempat yang memutasi `s.workspaces` tanpa clone; (b) `Membership` tidak pernah `nil` pada keluaran `List`/`Get`/`stateOf`; (c) `Groups[i].Order == i` setelah setiap mutasi dan setelah load; (d) tepat satu `Main` setelah load dan `DeleteGroup` menolak `Main`; (e) v2 lama tanpa `membership`/`main` termuat tanpa menambah grup dan file tidak ditulis ulang saat load.

## Perubahan

Dibuat:
- `internal/workspace/groups.go`
- `internal/workspace/ipc_groups.go`

Diubah (aditif minimal):
- `internal/workspace/types.go`: field `Workspace.Membership`, `Group.Main`, `Main: true` di `newDefaultGroup`.
- `internal/workspace/store.go`: salinan `Membership` di `cloneWorkspace`; `Membership: map[string]string{}` di `Store.Create`.
- `internal/workspace/config.go`: satu baris `normalizeGroups(w)` di `loadWorkspaces`.
- `internal/workspace/ipc.go`: satu baris `registerGroupMethods(srv, s)` di akhir `Register`.

## Wiring

Fitur ini sudah menyambungkan dirinya di dalam paket `workspace` lewat satu baris di `Register` (itu bagian ownedPaths, bukan langkah wiring terpisah). Yang diekspos untuk konsumen lain:

- Method IPC `workspace.group.create|rename|delete|reorder|setActive|moveSession` dan event `workspace.groupsChanged` (bentuk di bagian Kontrak), dipakai `groups-store` di frontend.
- Field `membership` dan `main` pada hasil `workspace.list` (dan payload `workspace.changed` yang membawa `workspace`).

Langkah Wiring akhir hanya memverifikasi: `Register` memanggil `registerGroupMethods(srv, s)`, `cmd/shikigamid/wire_workspace.go` tidak berubah, `workspacesFileVersion` tetap 2. Jangan menyentuh `cmd/`, `app/`, atau frontend dari fitur ini.

## Selesai bila

- `go build ./...` sukses (guard utama fitur ini); `gofmt` dan `go vet` untuk `internal/workspace` bersih. Tanpa unit test.
- Semua simbol di bagian Kontrak ada dengan signature yang sama.
- File v2 tanpa `membership`/`main`: dimuat tanpa galat, tanpa grup baru, grup "Utama" (atau ber-order terkecil) bertanda `Main`, order rapat, `Membership` `{}`, file tidak ditulis ulang sampai ada mutasi.
- `create` menambah grup di akhir (`layout: null`), tidak mengubah grup aktif, nama bawaan "Grup N" unik; `rename`, `reorder`, `setActive`, `moveSession` mengubah state dan mempersist atomik; event `workspace.groupsChanged` dikirim setelah persist sukses dan tidak untuk no-op.
- `delete` pada `Main` -> `ErrConflict` (kode IPC conflict) tanpa mengubah state; pada grup lain keanggotaannya pindah ke `Main`, grup aktif bergeser ke `Main` bila perlu, order rapat.
- Aturan nama: trim, maks 40 rune (lebih ditolak `ErrInvalid`), kosong ditolak pada rename, bentrok tanpa beda huruf besar/kecil diberi akhiran " 2", " 3".
- `workspace.list` mengeluarkan `membership` (objek) dan `main`; method/event layout dan ui lama tidak berubah; tidak ada dependency, timer, atau goroutine baru; tidak ada file di luar daftar Path yang dimiliki yang berubah.
