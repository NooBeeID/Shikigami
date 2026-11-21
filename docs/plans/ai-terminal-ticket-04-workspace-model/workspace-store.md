# Plan: Store dan persistensi

## Metadata

- Tiket: `ai-terminal-ticket-04-workspace-model` (`docs/tickets/ai-terminal-ticket-04-workspace-model.md`)
- Fitur: `workspace-store` (Store dan persistensi)
- ownedPaths: `internal/workspace/store.go`, `internal/workspace/config.go`
- Depends on foundation: ya. Foundation sudah menulis `internal/workspace/types.go` final (tipe, `ErrNotFound`/`ErrInvalid`/`ErrConflict`, `newID()`) serta stub `store.go` dan `config.go`. Plan ini mengganti TOTAL kedua stub itu.

Rujukan: `overview.md` bagian 2 (kontrak bersama), 6 (build guard), 7 (aturan paralel). Isi overview tidak diulang di sini.

## Tujuan

Mengimplementasikan versi final `Store` dan konfigurasi di paket `aiterm/internal/workspace`: CRUD workspace dan project, validasi path project, persistensi atomik ke `workspaces.json` dan `config.json`, rollback bila persist gagal, deep copy, `EffectiveConfig`, dan validasi URL http/https. Hanya stdlib, tanpa dependensi ke `internal/ipc`, tanpa goroutine latar, tanpa unit test.

## Path yang dimiliki

HANYA dua file ini yang boleh dibuat/diubah:

- `internal/workspace/store.go`
- `internal/workspace/config.go`

Dilarang menyentuh: `internal/workspace/types.go` (final dari Foundation; baca saja, ubah hanya bila ada bug nyata dan jangan ubah signature/tag), `internal/workspace/ipc.go`, `cmd/aitermd/wire_workspace.go`, `cmd/aitermctl/cmd_workspace.go` (milik `workspace-ipc-ctl`), file wiring, `go.mod`, `go.sum`, `Makefile`, `internal/ipc/`, `internal/paths/`, `frontend/`, `app/`, dan path epic lain.

## Kontrak

Disediakan (signature PERSIS, receiver pointer `*Store`; lihat overview 2.3):

```go
func NewStore(dataDir string) (*Store, error)
func (s *Store) List() []Workspace
func (s *Store) Get(id string) (Workspace, error)
func (s *Store) Create(name string) (Workspace, error)
func (s *Store) Rename(id, name string) (Workspace, error)
func (s *Store) Delete(id string) error
func (s *Store) AddProject(wsID, path, name string) (Workspace, Project, error)
func (s *Store) RemoveProject(wsID, projectID string) (Workspace, error)
func (s *Store) GetConfig(wsID string) (GlobalConfig, WorkspaceConfig, GlobalConfig, error)
func (s *Store) SetGlobalConfig(c GlobalConfig) error
func (s *Store) SetWorkspaceConfig(wsID string, c WorkspaceConfig) (Workspace, error)
func (s *Store) EffectiveConfig(wsID string) (GlobalConfig, error)
```

Tidak boleh menambah method publik baru di `Store`. Helper internal harus huruf kecil.

Dipakai dari `types.go` (sudah ada, jangan ditulis ulang): tipe `Project`, `WorkspaceConfig`, `Workspace`, `GlobalConfig`; sentinel `ErrNotFound`, `ErrInvalid`, `ErrConflict`; `newID() string`.

Konvensi error: bungkus dengan `fmt.Errorf("%w: detail", ErrX)` agar `errors.Is` bekerja dan `err.Error()` layak dipakai sebagai pesan IPC. Pesan detail memakai bahasa Indonesia atau Inggris singkat, tanpa pernah memuat nilai `GrimoireKey`.

Semantik yang diandalkan `ipc.go` (wajib persis):
- `GetConfig("")` mengembalikan `(global, WorkspaceConfig{}, global, nil)`. wsID tak ada -> `ErrNotFound`.
- `SetGlobalConfig` dan `SetWorkspaceConfig` menerima nilai LENGKAP (bukan patch). Validasi URL di dalam keduanya -> `ErrInvalid`.
- `List()` mengembalikan `[]Workspace` non-nil (`[]` saat kosong) berurutan sesuai pembuatan; `Projects` selalu non-nil di semua nilai kembalian (juga setelah load dari file yang berisi `"projects":null`).
- `EffectiveConfig("")` mengembalikan global; wsID tak ada -> `ErrNotFound`.
- Persistensi: `<dataDir>/workspaces.json` = `{"version":1,"workspaces":[...]}`; `<dataDir>/config.json` = `{"version":1,"global":{...}}`; file 0600, dir 0700.

## Langkah

### 1. `config.go` (ganti total stub Foundation)

Hapus komentar `STUB(foundation)` dan fungsi stub lama. Isi, `package workspace`, import stdlib saja (`encoding/json`, `errors`, `fmt`, `net/url`, `os`, `strings`):

1. Konstanta nama file dan versi (unexported): `workspacesFile = "workspaces.json"`, `configFile = "config.json"`, `fileVersion = 1`.
2. Tipe file on-disk (unexported):
   ```go
   type workspacesDoc struct {
       Version    int         `json:"version"`
       Workspaces []Workspace `json:"workspaces"`
   }
   type configDoc struct {
       Version int          `json:"version"`
       Global  GlobalConfig `json:"global"`
   }
   ```
3. `func validateURL(raw string) error`: string kosong -> nil. Selain itu `url.Parse`; error -> `fmt.Errorf("%w: url tidak valid: %v", ErrInvalid, err)`; scheme (lowercase) harus `http` atau `https`, dan `u.Host != ""`, jika tidak `fmt.Errorf("%w: url harus http/https dengan host", ErrInvalid)`. Jangan `TrimSpace` diam-diam: URL dengan spasi di tepi ditolak oleh `url.Parse`/cek host atau dinormalisasi dengan `strings.TrimSpace` sebelum disimpan; pilih satu perilaku dan konsisten (disarankan: `TrimSpace` pada URL dan key di `SetGlobalConfig`/`SetWorkspaceConfig`, lalu validasi hasilnya).
4. `func loadConfig(path string) (GlobalConfig, error)`: `os.ReadFile`; `errors.Is(err, os.ErrNotExist)` -> `GlobalConfig{}, nil`; error lain dibungkus `fmt.Errorf("baca %s: %w", path, err)`. `json.Unmarshal` ke `configDoc`; gagal -> `fmt.Errorf("config korup (%s): %w", path, err)` (TIDAK menimpa file). Kembalikan `doc.Global`.
5. `func loadWorkspaces(path string) ([]Workspace, error)`: pola sama; file tak ada -> `[]Workspace{}, nil`; JSON korup -> error jelas tanpa menimpa. Setelah unmarshal: jika slice nil jadikan `[]Workspace{}`; untuk tiap workspace, jika `Projects == nil` set `[]Project{}`.
6. `func writeJSONAtomic(path string, v any) error`: 
   - `data, err := json.MarshalIndent(v, "", "  ")`, tambah newline akhir.
   - `dir := filepath.Dir(path)` (tambahkan import `path/filepath`); `f, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")`.
   - `defer`-style cleanup: bila ada kegagalan di langkah berikutnya, `f.Close()` dan `os.Remove(tmpName)`.
   - `f.Chmod(0o600)` (CreateTemp sudah 0600, tetap eksplisit), `f.Write(data)`, `f.Sync()`, `f.Close()`, lalu `os.Rename(tmpName, path)`. Jangan pernah menulis langsung ke path target.
   - Opsional: setelah rename, `os.Chmod(path, 0o600)` untuk memastikan izin.
7. Method konfigurasi pada `Store` (di file ini agar logika config terkumpul; semua receiver `*Store`; jangan menambah method publik di luar kontrak):
   - `GetConfig`, `SetGlobalConfig`, `SetWorkspaceConfig`, `EffectiveConfig` boleh ditulis di `config.go` (disarankan) atau `store.go`; pilih SATU lokasi dan jangan duplikasi. Rencana ini menaruhnya di `config.go`.
   - helper murni `func mergeConfig(global GlobalConfig, ws WorkspaceConfig) GlobalConfig`: hasil `GrimoireURL = ws.GrimoireURL` bila non-kosong, selain itu `global.GrimoireURL`; sama untuk `GrimoireKey` (per field independen).
   - `GetConfig(wsID)`: `RLock`; `wsID == ""` -> `(s.global, WorkspaceConfig{}, s.global, nil)`; cari workspace lewat `s.indexLocked(wsID)` (helper di store.go), tak ada -> `fmt.Errorf("%w: workspace %q", ErrNotFound, wsID)`; kembalikan `(s.global, ws.Config, mergeConfig(s.global, ws.Config), nil)`.
   - `EffectiveConfig(wsID)`: pakai `GetConfig` internal (atau logika serupa) dan kembalikan elemen efektif. Hati-hati deadlock: jangan memanggil method publik yang mengambil lock dari dalam lock yang sudah dipegang.
   - `SetGlobalConfig(c)`: validasi `validateURL(c.GrimoireURL)` sebelum mengambil lock tulis; `Lock`; simpan `old := s.global`; `s.global = c`; `if err := writeJSONAtomic(s.cfgPath, configDoc{fileVersion, c}); err != nil { s.global = old; return fmt.Errorf("simpan config: %w", err) }`.
   - `SetWorkspaceConfig(wsID, c)`: validasi URL; `Lock`; cari index (tak ada -> `ErrNotFound`); salin snapshot `next := cloneWorkspaces(s.workspaces)`; `next[i].Config = c`; `s.persistWorkspacesLocked(next)` (lihat store.go); sukses -> `s.workspaces = next`, kembalikan `cloneWorkspace(next[i])`.
   - Urutan cek di `SetWorkspaceConfig`: validasi URL lebih dulu atau NotFound lebih dulu keduanya diterima; disarankan NotFound dulu bila wsID tak ada (cari dulu di bawah RLock tidak perlu; cukup di bawah Lock), agar kode error konsisten dengan tabel tiket.

### 2. `store.go` (ganti total stub Foundation)

Hapus semua komentar `STUB(foundation)` dan method stub. Import stdlib: `errors`, `fmt`, `os`, `path/filepath`, `strings`, `sync`.

1. Struct:
   ```go
   type Store struct {
       mu         sync.RWMutex
       dir        string
       wsPath     string
       cfgPath    string
       workspaces []Workspace // urut pembuatan; Projects selalu non-nil
       global     GlobalConfig
   }
   ```
2. `NewStore(dataDir string) (*Store, error)`:
   - `dataDir` kosong -> `fmt.Errorf("%w: dataDir kosong", ErrInvalid)`.
   - `os.MkdirAll(dataDir, 0o700)`; gagal -> bungkus error. (Dir yang sudah ada tidak diubah izinnya.)
   - `wsPath = filepath.Join(dataDir, workspacesFile)`, `cfgPath = filepath.Join(dataDir, configFile)`.
   - `loadWorkspaces` dan `loadConfig`; error apa pun dikembalikan apa adanya (file korup -> `NewStore` error, tidak menimpa apa pun). TIDAK menulis file saat load (file belum ada = state kosong, tidak dibuat sampai mutasi pertama).
   - Kembalikan `*Store` terisi.
3. Helper deep copy (unexported):
   - `cloneWorkspace(w Workspace) Workspace`: salin struct, `Projects` baru dengan `make([]Project, len(w.Projects))` + `copy` (selalu non-nil, panjang 0 bila kosong). `Project` dan `WorkspaceConfig` berisi string saja sehingga cukup nilai-salin.
   - `cloneWorkspaces(ws []Workspace) []Workspace`: hasil non-nil (`make([]Workspace, len(ws))`), tiap elemen lewat `cloneWorkspace`.
4. Helper:
   - `func (s *Store) indexLocked(id string) int`: loop linear, `-1` bila tak ada. Dipanggil hanya saat lock dipegang.
   - `func (s *Store) persistWorkspacesLocked(ws []Workspace) error`: `writeJSONAtomic(s.wsPath, workspacesDoc{Version: fileVersion, Workspaces: ws})`, bungkus error `fmt.Errorf("simpan workspaces: %w", err)`. Pastikan `ws` non-nil dan semua `Projects` non-nil agar serialisasi `[]` bukan `null`.
5. Pola mutasi (rollback): untuk setiap mutasi, di bawah `Lock`: buat `next := cloneWorkspaces(s.workspaces)`, ubah `next`, panggil `persistWorkspacesLocked(next)`; HANYA bila sukses lakukan `s.workspaces = next`. Gagal persist -> state in-memory tidak berubah dan error dikembalikan. Ini memenuhi syarat rollback tanpa kode undo manual.
6. Operasi:
   - `List()`: `RLock`; `return cloneWorkspaces(s.workspaces)`.
   - `Get(id)`: `RLock`; index; tak ada -> `fmt.Errorf("%w: workspace %q", ErrNotFound, id)`; kembalikan `cloneWorkspace`.
   - `Create(name)`: `name = strings.TrimSpace(name)`; kosong -> `fmt.Errorf("%w: nama workspace kosong", ErrInvalid)`; `w := Workspace{ID: newID(), Name: name, Projects: []Project{}}`; append ke `next`; persist; kembalikan `cloneWorkspace(w)`. Nama duplikat diperbolehkan.
   - `Rename(id, name)`: TrimSpace; kosong -> `ErrInvalid`; id tak ada -> `ErrNotFound` (cek keberadaan id boleh sebelum cek nama atau sesudahnya; disarankan nama dulu karena murah, tidak masalah mana yang lebih dulu); ubah `next[i].Name`; persist; kembalikan salinan.
   - `Delete(id)`: id tak ada -> `ErrNotFound`; `next` = hapus elemen i dengan menjaga urutan (`append(next[:i], next[i+1:]...)`; karena `next` salinan, aman); persist. TIDAK menyentuh disk project. Pastikan hasil `next` non-nil bila jadi kosong (`cloneWorkspaces` sudah non-nil; setelah append-delete tetap non-nil).
   - `AddProject(wsID, path, name)`:
     1. `normalizeProjectPath(path)` (helper di bawah) SEBELUM atau sesudah lock; boleh sebelum lock karena murni I/O baca. Namun cek keberadaan workspace harus menghasilkan `ErrNotFound` untuk wsID tak ada; urutan error yang konsisten dengan tiket: workspace tak ada -> `ErrNotFound`, path buruk -> `ErrInvalid`, duplikat -> `ErrConflict`. Lakukan: `Lock`, cari index (tak ada -> NotFound), baru normalisasi path (-> Invalid), lalu cek duplikat.
     2. `name = strings.TrimSpace(name)`; kosong -> `filepath.Base(normalizedPath)`.
     3. Duplikat: bandingkan `normalizedPath` dengan `Path` tiap project di workspace itu (string sama persis, karena yang tersimpan sudah ternormalisasi) -> `fmt.Errorf("%w: path sudah ada di workspace: %s", ErrConflict, normalizedPath)`. Path yang sama di workspace lain boleh.
     4. `p := Project{ID: newID(), Path: normalizedPath, Name: name}`; append ke `next[i].Projects`; persist; kembalikan `(cloneWorkspace(next[i]), p, nil)`.
     5. Kembalikan `(Workspace{}, Project{}, err)` pada error.
   - `RemoveProject(wsID, projectID)`: workspace tak ada -> `ErrNotFound`; project tak ada di workspace itu -> `ErrNotFound` (`fmt.Errorf("%w: project %q", ErrNotFound, projectID)`); hapus pada `next[i].Projects` menjaga urutan dan non-nil; persist; kembalikan salinan.
7. `func normalizeProjectPath(p string) (string, error)`:
   - `p = strings.TrimSpace(p)`? TIDAK perlu mengubah path; cukup periksa `p == ""` -> `ErrInvalid` ("path kosong").
   - `!filepath.IsAbs(p)` -> `fmt.Errorf("%w: path harus absolut: %s", ErrInvalid, p)`.
   - `clean := filepath.Clean(p)`.
   - `fi, err := os.Stat(clean)`; err -> `fmt.Errorf("%w: path tidak dapat diakses: %v", ErrInvalid, err)`; `!fi.IsDir()` -> `fmt.Errorf("%w: bukan direktori: %s", ErrInvalid, clean)`.
   - `if resolved, err := filepath.EvalSymlinks(clean); err == nil { clean = filepath.Clean(resolved) }` (gagal EvalSymlinks tapi Stat sukses -> pakai hasil Clean).
   - Kembalikan `clean, nil`. Catatan macOS: `/tmp` menjadi `/private/tmp` setelah EvalSymlinks; ini perilaku yang disengaja.
8. Pastikan seluruh nilai kembalian publik adalah salinan dalam sehingga pemanggil tidak dapat memutasi state internal, dan tidak ada slice internal yang bocor.
9. Penguncian: method baca memakai `RLock`, mutasi memakai `Lock`; tidak ada method publik yang memanggil method publik lain sambil memegang lock (hindari deadlock `sync.RWMutex` yang tidak reentrant).
10. Tidak ada log, tidak ada `panic` (satu-satunya `panic` ada di `newID` milik Foundation), tidak ada goroutine, tidak ada `init()`.

### 3. Verifikasi

Jalankan dari root repo:

- `gofmt -l internal/workspace` (harus kosong)
- `go build ./...` (harus sukses; `workspace-ipc-ctl` membuild terhadap signature yang sama)
- `go vet ./internal/workspace/...` (opsional)
- `grep -rn "belum diimplementasi\|STUB(foundation)" internal/workspace/store.go internal/workspace/config.go` (harus kosong)
- Pastikan `git`/`diff` menunjukkan `go.mod` dan `go.sum` tidak berubah, dan tidak ada file lain di luar ownedPaths yang berubah.

Tanpa unit test (aturan epic). Bila ingin cek cepat manual, gunakan program sementara di luar repo (mis. direktori scratch) dan jangan commit.

## Perubahan

- Diubah (ganti total): `internal/workspace/store.go`
- Diubah (ganti total): `internal/workspace/config.go`
- Tidak ada file dibuat, tidak ada file lain diubah.

## Wiring

Tidak ada yang disambungkan oleh fitur ini. Yang diekspos untuk langkah wiring dan fitur `workspace-ipc-ctl`:

- `workspace.NewStore(dataDir)` dan 11 method `*Store` sesuai Kontrak, dengan error sentinel `ErrNotFound`/`ErrInvalid`/`ErrConflict` yang dipetakan di `ipc.go` memakai `errors.Is`.
- Perilaku persis `GetConfig("")` dan Set* bernilai lengkap seperti di bagian Kontrak.
- `wire_workspace.go` (milik fitur lain) akan membuat store dari `Deps.DataDir`; fitur ini tidak mendaftarkan modul, method IPC, atau event.
- Pastikan setelah selesai tidak tersisa penanda stub (cek wiring overview 5 butir 3).

## Selesai bila

- `internal/workspace/store.go` dan `internal/workspace/config.go` berisi implementasi final; tidak ada teks "belum diimplementasi" atau `STUB(foundation)`.
- Signature publik sama persis dengan Kontrak; tidak ada method publik tambahan; tidak ada import selain stdlib dan tidak ada import `internal/ipc`.
- Perilaku sesuai acceptance criteria tiket: dir dibuat 0700, state kosong untuk file tak ada, file korup -> `NewStore` error tanpa menimpa, penulisan temp+Sync+Rename dengan file 0600, validasi path (absolut, ada, direktori, Clean+EvalSymlinks, duplikat per workspace -> `ErrConflict`), `Projects` selalu `[]`, `EffectiveConfig` per field, validasi URL http/https dengan host, rollback bila persist gagal, deep copy.
- `go build ./...` sukses dan `gofmt -l internal/workspace` kosong.
- Tidak ada file di luar `ownedPaths` yang diubah (termasuk `types.go`, `go.mod`, `go.sum`).
