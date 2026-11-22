# Plan: Field LastOpenedAt dan RPC workspace.touch

## Metadata
- Tiket: `welcome-screen-ticket-01-session-workspaces` (`docs/tickets/welcome-screen-ticket-01-session-workspaces.md`)
- Fitur: `workspace-touch-go`
- ownedPaths: `internal/workspace/types.go`, `internal/workspace/store.go`, `internal/workspace/ipc.go`, `internal/workspace/touch.go`, `cmd/shikigamictl/cmd_workspace.go`
- Depends on foundation: tidak (epic ini tidak punya foundation; lihat overview bagian 4)
- Module Go: `github.com/NooBeeID/shikigami`
- Rujukan kontrak: `overview.md` bagian 2.1 dan 6 (di folder plan yang sama; jangan diubah).

## Tujuan
Backend mencatat kapan workspace terakhir dibuka. Tambah field `LastOpenedAt` pada `Workspace`, method `Store.Touch(id)`, RPC `workspace.touch {id}` yang mengembalikan workspace penuh dan mem-publish event `updated`, serta subperintah debug `shikigamictl workspace touch <id>`. Data lama tanpa field harus tetap termuat (nilai 0).

## Path yang dimiliki
HANYA file ini yang boleh disentuh:
- `internal/workspace/types.go` (ubah)
- `internal/workspace/store.go` (ubah)
- `internal/workspace/ipc.go` (ubah, hanya SATU baris tambahan)
- `internal/workspace/touch.go` (baru)
- `cmd/shikigamictl/cmd_workspace.go` (ubah)

Jangan menyentuh file frontend, file wiring lain, atau path fitur lain (`open-set-store`, `null-workspace-safety`, `tabs-close-menu`). Jangan menambah unit test atau dependency. Jangan commit/merge di luar alur implement-feature.

## Kontrak
Disediakan (sesuai overview 2.1):
- `Workspace.LastOpenedAt int64` dengan tag `json:"lastOpenedAt"` (tanpa `omitempty`; unix ms; 0 = belum pernah dibuka / data lama).
- `func (s *Store) Touch(id string) (Workspace, error)`; id tak dikenal -> `fmt.Errorf("%w: workspace %q", ErrNotFound, id)`.
- `func registerTouch(srv *ipc.Server, s *Store)` di `touch.go`, mendaftarkan method IPC `workspace.touch`.
- RPC `workspace.touch` params `{ "id": string }` -> objek `Workspace` penuh (tidak dibungkus). Event: `workspace.changed` `{ kind: "updated", workspaceId, workspace }`.
- CLI: `shikigamictl workspace touch <id>`.

Dipakai (sudah ada, jangan diubah selain yang dinyatakan di Langkah): `indexLocked`, `cloneWorkspaces`, `cloneWorkspace`, `persistLocked`, `ErrNotFound`, `decode`, `mapErr`, `publish` (di `ipc.go`), `callAndPrint` (di paket `main` shikigamictl), `RegisterCommand`.

## Langkah

1. `internal/workspace/types.go`: pada struct `Workspace`, tambahkan field setelah `Membership`:
   ```go
   LastOpenedAt int64 `json:"lastOpenedAt"` // unix ms; 0 = belum pernah dibuka / data lama
   ```
   Tanpa `omitempty`. `cloneWorkspace` memakai `c := w` sehingga field ikut tersalin; JANGAN ubah `cloneWorkspace`. Data JSON lama tanpa field terbaca 0 secara otomatis (tidak perlu migrasi, jangan naikkan `workspacesFileVersion`). `time` sudah diimpor di file ini; tidak perlu impor baru di `types.go`.

2. `internal/workspace/store.go`: tambahkan impor `"time"` ke blok import. Tambahkan method tepat setelah `Rename`:
   ```go
   // Touch mencatat waktu buka terakhir workspace (unix ms) dan mempersistnya.
   func (s *Store) Touch(id string) (Workspace, error) {
       s.mu.Lock()
       defer s.mu.Unlock()
       i := s.indexLocked(id)
       if i < 0 {
           return Workspace{}, fmt.Errorf("%w: workspace %q", ErrNotFound, id)
       }
       next := cloneWorkspaces(s.workspaces)
       next[i].LastOpenedAt = time.Now().UnixMilli()
       if err := s.persistLocked(next, s.ui); err != nil {
           return Workspace{}, err
       }
       s.workspaces = next
       return cloneWorkspace(next[i]), nil
   }
   ```
   Pola identik dengan `Rename` (copy-on-write; `s.workspaces` hanya diganti setelah persist sukses). Tulis dengan indentasi tab sesuai gofmt.

3. `internal/workspace/touch.go` (file baru, `package workspace`):
   ```go
   package workspace

   import (
       "context"
       "encoding/json"

       "github.com/NooBeeID/shikigami/internal/ipc"
   )

   // registerTouch mendaftarkan RPC workspace.touch {id} -> Workspace.
   func registerTouch(srv *ipc.Server, s *Store) {
       srv.Register("workspace.touch", func(ctx context.Context, params json.RawMessage) (any, error) {
           var p struct {
               ID string `json:"id"`
           }
           if err := decode(params, &p); err != nil {
               return nil, err
           }
           ws, err := s.Touch(p.ID)
           if err != nil {
               return nil, mapErr(err)
           }
           publish(srv, "updated", ws.ID, &ws)
           return ws, nil
       })
   }
   ```

4. `internal/workspace/ipc.go`: di `Register()`, setelah baris `registerGroupMethods(srv, s)` (akhir fungsi) tambahkan tepat satu baris:
   ```go
   registerTouch(srv, s)
   ```
   Tidak ada perubahan lain di file ini (tanpa impor baru).

5. `cmd/shikigamictl/cmd_workspace.go`:
   - `workspaceUsage`: tambahkan baris `  touch <id>` setelah `  delete <id>` (jaga indentasi dua spasi seperti baris lain).
   - `Summary` di `init()`: ubah menjadi `"kelola workspace dan project (list, create, rename, delete, touch, add-project, remove-project, config)"`.
   - Di `runWorkspace`, tambahkan case setelah `case "delete"`:
     ```go
     case "touch":
         if len(rest) != 1 {
             return errors.New("pemakaian: shikigamictl workspace touch <id>")
         }
         return callAndPrint(c, "workspace.touch", map[string]any{"id": rest[0]})
     ```

6. Format dan guard: jalankan `gofmt -l internal/workspace cmd/shikigamictl` (harus kosong) lalu `go build ./...` dari root repo.

## Perubahan
- Diubah: `internal/workspace/types.go` (field `LastOpenedAt`)
- Diubah: `internal/workspace/store.go` (impor `time`, `Store.Touch`)
- Diubah: `internal/workspace/ipc.go` (satu baris `registerTouch(srv, s)`)
- Dibuat: `internal/workspace/touch.go` (`registerTouch`)
- Diubah: `cmd/shikigamictl/cmd_workspace.go` (`workspaceUsage`, `Summary`, case `touch`)

## Wiring
Fitur ini tidak menyambungkan apa pun di luar `ownedPaths`. Yang diekspos:
- RPC `workspace.touch {id}` -> `Workspace` penuh (dengan `lastOpenedAt`), dipanggil frontend lewat `call('workspace.touch', { id })` oleh fitur `open-set-store` (`openWorkspace`), best-effort.
- Event `workspace.changed` kind `updated` setiap touch.
- `workspace.list` kini memuat `lastOpenedAt` (dipakai tiket 02 untuk daftar "Terakhir dibuka").
- Pemanggil `registerTouch(srv, s)` sudah ditambahkan di `Register()` oleh fitur ini sendiri (satu-satunya baris yang diizinkan di `ipc.go`); tidak ada langkah wiring Go lain. Daemon perlu dibangun ulang saat verifikasi akhir epic agar RPC tersedia.

## Selesai bila
- [ ] `Workspace.LastOpenedAt` ada dengan tag JSON `lastOpenedAt`; data lama tanpa field termuat dan `workspace.list` mengembalikan 0.
- [ ] `Store.Touch` mengisi unix ms sekarang, persist, mengembalikan salinan; id tak dikenal -> error yang `errors.Is(err, ErrNotFound)`.
- [ ] `workspace.touch` terdaftar lewat `registerTouch` di `touch.go`; `ipc.go` hanya bertambah satu baris.
- [ ] `shikigamictl workspace touch <id>` terdaftar; jumlah argumen salah menampilkan pesan pemakaian; `workspaceUsage` dan `Summary` diperbarui.
- [ ] `gofmt -l` pada path fitur kosong.
- [ ] Guard: `go build ./...` sukses (tanpa unit test). Tidak ada file di luar ownedPaths yang berubah (`git status` hanya menampilkan lima path di atas).
