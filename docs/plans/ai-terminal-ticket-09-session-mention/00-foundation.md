# Foundation: Epic 09 Session Mention

Baca dulu `overview.md` di folder ini (terutama bagian 1.2, 2.2 sampai 2.4, 2.7) dan tiket `docs/tickets/ai-terminal-ticket-09-session-mention.md`. Foundation ini dikerjakan SATU agent sebelum fitur paralel. Satu-satunya file yang dibuat: `internal/mention/types.go`. Tidak ada tes; guard hanya build.

## Ruang lingkup

- Boleh dibuat: `internal/mention/types.go`.
- Dilarang menyentuh: file lain di repo (termasuk `go.mod`, `internal/provider`, `internal/session`, `internal/status`). Tidak boleh mengimpor `aiterm/internal/status` (paket itu belum tentu ada; Foundation tidak membutuhkannya) dan tidak boleh menambah dependency.

## Langkah

### 1. Periksa prasyarat

- [ ] Pastikan `go.mod` memuat module `aiterm` dan `internal/session` mengekspor `Session`, `AttachResult` (`grep -n "type Session\|type AttachResult" internal/session/types.go`).
- [ ] Pastikan `internal/mention/` belum ada (`ls internal/mention`); bila sudah ada file lain, jangan menimpa, hanya buat `types.go` dan laporkan.

### 2. Tulis `internal/mention/types.go`

Paket `mention`. Impor stdlib: `context`, `crypto/rand`, `errors`, `log/slog`, `regexp`, `sort`, `strings`, `sync`, `sync/atomic` (hanya yang terpakai), `time`; plus `aiterm/internal/session`.

- [ ] Tipe dan konstanta persis seperti overview 2.2: `State` (+4 konstanta), `SourceAgent`/`SourceUser`, `FromUser`, `Mode` (+3 konstanta), `Message` (tag JSON camelCase termasuk `error,omitempty`), `NameInfo`.
- [ ] Error sentinel (`errors.New`, pesan Indonesia): `ErrNotFound` "sesi tidak ditemukan", `ErrInvalid` "masukan tidak valid", `ErrConflict` "antrean penuh", `ErrCrossWorkspace` "bukan workspace yang sama", `ErrUserOnly` "mode user_only: pesan dari agent ditolak". Komentar menyebut pemetaan ke kode IPC (overview 2.2), tetapi pemetaan itu sendiri diimplementasikan di `ipc.go` oleh `message-queue`, bukan di sini (jangan impor `internal/ipc`).
- [ ] Interface: `SessionSource`, `Directory`, `StatusFunc`, `Enqueuer`, `QueueReader`, struct `Runtime` (overview 2.3). Tambahkan komentar bahwa `*session.Manager` memenuhi `SessionSource`. Sertakan `var _ SessionSource = (*session.Manager)(nil)` agar kecocokan signature terverifikasi saat build.
- [ ] `NewMessageID() string`: ULID 26 karakter Crockford base32 (10 karakter waktu ms + 16 karakter acak dari `crypto/rand`); aman konkuren. Boleh disederhanakan: 48 bit ms big-endian + 80 bit acak, dikodekan 5 bit per karakter seperti `internal/session` (jangan impor fungsi privat; tulis ulang kecil). Monotonik tidak wajib.
- [ ] `Namer`:
  - `NewNamer(src SessionSource, dir Directory, statusOf StatusFunc) *Namer`; `dir` atau `statusOf` nil ditangani (nama project `""`, status `"running"`).
  - Fungsi privat `baseName(providerID, projectName string) string`: gabung `providerID + "-" + projectName`, huruf kecil, ganti setiap karakter di luar `[a-z0-9]` dengan `-`, lipat `-` berurutan, buang `-` di tepi; project kosong memakai segmen `noproject`; provider kosong memakai `shell`.
  - Fungsi privat `snapshot(workspaceID string) []namedSession` yang mengambil `src.List()`, menyaring sesi `Status == session.StatusRunning` (dan `WorkspaceID == workspaceID` bila workspaceID tidak kosong), urut `CreatedAt` lalu `ID`, lalu menetapkan nama: sesi pertama dengan nama dasar tertentu memakai nama dasar, berikutnya `-2`, `-3`, dst. Penamaan dihitung per workspace (kelompokkan menurut `WorkspaceID` bila snapshot lintas workspace).
  - `NameOf(sessionID)`: `src.Get(id)`; bila gagal atau `exited` kembalikan `""`; selain itu hitung nama dalam workspace sesi itu.
  - `ResolveName(workspaceID, name)`: cocokkan `name` (huruf kecil, `strings.TrimPrefix(name, "@")`) dengan hasil snapshot workspace; tak ada -> `ErrNotFound`.
  - `Resolve(workspaceID, to)`: `to` kosong -> `ErrInvalid`; coba sebagai id (`src.Get`): bila ada dan running, `WorkspaceID == workspaceID` -> kembalikan id, workspace beda -> `ErrCrossWorkspace`; id ada tetapi exited -> `ErrNotFound`. Jika bukan id, coba `ResolveName`; bila nama hanya ada di workspace lain (cek snapshot lintas workspace) -> `ErrCrossWorkspace`, selain itu `ErrNotFound`.
  - `List(workspaceID)`: `[]NameInfo` (tidak pernah nil), `ProjectName` dari `dir.ProjectName(ws, projectID)`, `Status` dari `statusOf(id)` (kosong -> `"running"`).
- [ ] Hook start dan URL: `var startMCP func(ctx context.Context, rt *Runtime) (string, error)`; `Start(ctx, rt)`: bila `rt == nil` -> `ErrInvalid`; bila `startMCP == nil` -> `("", nil)`; selain itu panggil `startMCP`, simpan URL lewat `setURL`, kembalikan. `URL()` dan `setURL()` thread-safe (mutex atau `atomic.Value`).
- [ ] Komentar dokumentasi tiap simbol ekspor (Indonesia, ringkas).

### 3. Guard

- [ ] `go build ./...` lolos. Perbaiki hingga hijau (jangan sentuh file lain; bila `go vet`/impor tak terpakai mengganggu, rapikan impor di `types.go`).
- [ ] `gofmt -l internal/mention` tidak mengeluarkan apa pun.

### 4. Selesai bila

- `internal/mention/types.go` ada, memuat semua simbol di overview 2.2 sampai 2.4 dengan signature persis (fitur paralel mengimpornya tanpa menebak).
- `go build ./...` lolos.
- Laporkan: daftar simbol ekspor dan setiap penyimpangan dari overview (idealnya tidak ada).
