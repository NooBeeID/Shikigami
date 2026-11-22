# Plan: review.comment lewat session.write

## Metadata

- Tiket: `agent-diff-review-ticket-04-review-comment` (`docs/tickets/agent-diff-review-ticket-04-review-comment.md`)
- Fitur: `review-comment-daemon`
- ownedPaths: `internal/review/comment.go` (baru), `cmd/shikigamid/wire_review.go` (hanya penambahan satu blok di `d.OnReady`)
- Depends on foundation: tidak (`hasFoundation=false`)
- Rujukan kontrak: `docs/plans/agent-diff-review-ticket-04-review-comment/overview.md` bagian 2.2, 2.3, 2.4 (jangan diubah).

## Tujuan

Menyediakan method IPC `review.comment {sessionId, path, line, text} -> {ok, sent, enter:false}`. Handler memvalidasi params, memastikan path berada di cwd sesi, menolak sesi `running` dengan `[BUSY]`, lalu menulis satu baris `di <path>:<line>: <text>` (tanpa Enter) ke PTY sesi lewat satu `Manager.Write`. Tanpa membaca isi file, tanpa jaringan, tanpa unit test.

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `internal/review/comment.go` (berkas baru)
- `cmd/shikigamid/wire_review.go` (satu blok tambahan + import; jangan ubah registrasi lain, hook listener, debouncer, event, atau fungsi helper yang sudah ada)

Jangan sentuh: `internal/review/{store,capture,ipc,types,diff,diff_ipc,actions,actions_ipc}.go`, `internal/session/`, `internal/status/`, `internal/ipc/`, `go.mod`, apa pun di `frontend/`, dan berkas wiring lain. Jangan menjalankan `git commit`.

## Kontrak

Disediakan (persis seperti overview 2.3), semuanya di paket `review`:
- `const IPCMethodComment = "review.comment"`, `const commentMaxRunes = 2000`
- `var ErrSessionGone = errors.New("sesi tidak ada atau sudah berakhir")`
- `type StateReader func(sessionID string) (running bool, known bool)`
- `type CommentDeps struct { Cwd func(string) (string, error); Write func(string, []byte) error; State StateReader }`
- `type CommentParams struct { SessionID string \`json:"sessionId"\`; Path string \`json:"path"\`; Line int \`json:"line"\`; Text string \`json:"text"\` }`
- `type CommentResult struct { OK bool \`json:"ok"\`; Sent string \`json:"sent"\`; Enter bool \`json:"enter"\` }`
- `func FormatComment(path string, line int, text string) (string, error)`
- `func Comment(ctx context.Context, deps CommentDeps, p CommentParams) (CommentResult, error)`
- `func RegisterCommentIPC(srv *ipc.Server, deps CommentDeps)`

Dipakai (sudah ada, fakta kode terverifikasi):
- `ipc.NewError(code, msg)`, `ipc.Errorf`, konstanta `ipc.ErrInvalidParams`, `ipc.ErrInvalid`, `ipc.ErrNotFound`, `ipc.ErrConflict` dan `(*ipc.Server).Register(method, func(ctx context.Context, params json.RawMessage) (any, error))` (pola sama dengan `internal/review/diff_ipc.go`).
- `fsx.ResolveUnder(root, rel string) (string, error)` dari `github.com/NooBeeID/shikigami/internal/fsx` (hanya validasi; hasil tidak dibaca isinya; root harus absolut).
- Di `wire_review.go`: `session.Manager` (`Get(id) (*Session, error)` dengan `Session.Cwd`; `Write(id string, data []byte) error`; `session.ErrNotRunning`, `session.ErrNotFound`), `status.Engine.Get(id) (SessionState, bool)`, `status.StatusRunning`, `d.Lookup("session.manager")`.
- Perhatian: paket `session` juga punya konstanta `session.StatusRunning`; di wiring pakai `status.StatusRunning` dari paket `status`, bukan `session.*`.
- Paket `review` TIDAK boleh mengimpor `session`, `status`, atau `cmd`.

## Langkah

1. **Baca dulu** `internal/review/diff_ipc.go` (pola registrasi), `internal/ipc/errors.go`, `internal/fsx/readwrite.go` (signature `ResolveUnder`), dan `cmd/shikigamid/wire_review.go`. Bila kode nyata beda dari uraian plan, ikuti kode dan catat di laporan.

2. **Buat `internal/review/comment.go`** (`package review`). Import: `context`, `encoding/json`, `errors`, `strings`, `unicode/utf8`, `.../internal/fsx`, `.../internal/ipc`.

   a. Konstanta, `ErrSessionGone`, `StateReader`, `CommentDeps`, `CommentParams`, `CommentResult` sesuai bagian Kontrak. Tambahkan konstanta privat pesan sibuk: `commentBusyMsg = "[BUSY] agent sedang bekerja, tunggu selesai lalu kirim lagi"`.

   b. `func commentStripControl(s string) string`: buang rune < 0x20 dan 0x7f (`strings.Map` mengembalikan -1). Dipakai untuk `path`.

   c. `func commentSanitizeText(s string) string`:
      - `strings.TrimSpace`.
      - Ganti setiap rangkaian `\r`/`\n` berurutan (boleh campur) dengan SATU spasi (lintasi rune; bila rune adalah `\r` atau `\n`, tulis spasi hanya jika rune sebelumnya bukan bagian rangkaian newline).
      - Buang karakter kontrol lain (< 0x20 dan 0x7f, termasuk ESC dan `\t`) agar tidak ada escape sequence. Catatan: spasi dari langkah sebelumnya (0x20) tidak termasuk kontrol dan dipertahankan.
      - `strings.TrimSpace` lagi.

   d. `func FormatComment(path string, line int, text string) (string, error)`:
      - `p := commentStripControl(strings.TrimSpace(path))`; bila kosong -> `ipc.NewError(ipc.ErrInvalid, "path wajib diisi")`.
      - `line < 1` -> `ErrInvalid` ("line harus >= 1").
      - `t := commentSanitizeText(text)`; kosong -> `ErrInvalid` ("text wajib diisi").
      - Kembalikan `fmt.Sprintf("di %s:%d: %s", p, line, t)` (tambahkan import `fmt`). Tanpa `\r`/`\n`/Enter di akhir.
      - Kembalikan error bertipe hasil `ipc.NewError` agar handler bisa meneruskannya langsung.

   e. `func Comment(ctx context.Context, deps CommentDeps, p CommentParams) (CommentResult, error)` dengan urutan persis overview 2.2 (setelah params berhasil di-unmarshal di handler):
      1. Trim `SessionID` dan `Path`. `sessionId` kosong, `path` kosong, `line < 1`, `strings.TrimSpace(text) == ""`, `utf8.RuneCountInString(text) > commentMaxRunes` (hitung pada teks yang di-trim) -> `ipc.NewError(ipc.ErrInvalid, ...)` dengan pesan Indonesia spesifik.
      2. Bila `deps.Cwd == nil || deps.Write == nil` -> `ipc.NewError(ipc.ErrInvalid, "review.comment belum dikonfigurasi")` (pengaman; jangan panic).
      3. `cwd, err := deps.Cwd(id)`; `err != nil` atau `cwd == ""` -> `ipc.NewError(ipc.ErrNotFound, "sesi tidak ditemukan")`.
      4. `_, err = fsx.ResolveUnder(cwd, path)`; gagal -> `ipc.NewError(ipc.ErrInvalid, "path tidak valid: "+err.Error())`. Jangan membuka atau membaca file.
      5. Cek state: bila `deps.State != nil`, `running, _ := deps.State(id)`; `running` -> `ipc.NewError(ipc.ErrConflict, commentBusyMsg)`. `known=false` diperlakukan tidak running.
      6. `sent, err := FormatComment(path, p.Line, p.Text)`; teruskan error bila ada.
      7. SATU panggilan `deps.Write(id, []byte(sent))`. Bila `errors.Is(err, ErrSessionGone)` -> `ipc.NewError(ipc.ErrNotFound, "sesi sudah berakhir")`; error lain -> `ipc.NewError(ipc.ErrInvalid, "gagal menulis ke sesi: "+err.Error())`. Jangan retry, jangan write kedua.
      8. Sukses: `CommentResult{OK: true, Sent: sent, Enter: false}`.
      - `ctx` boleh tidak dipakai selain pemeriksaan awal `ctx.Err()` (kembalikan `ctx.Err()` bila sudah dibatalkan).

   f. `func RegisterCommentIPC(srv *ipc.Server, deps CommentDeps)`:
      ```go
      srv.Register(IPCMethodComment, func(ctx context.Context, params json.RawMessage) (any, error) {
          var p CommentParams
          if err := json.Unmarshal(params, &p); err != nil {
              return nil, ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh {\"sessionId\": string, \"path\": string, \"line\": number, \"text\": string}")
          }
          return Comment(ctx, deps, p)
      })
      ```
      Perhatikan: kembalikan `(any, error)` dengan benar agar `CommentResult` nol tidak terbungkus pada jalur error (pada error kembalikan `nil, err`).

   g. Tanpa pembacaan isi file (tidak ada `os.ReadFile`/`os.Open`), tanpa paket `net`/`net/http`.

3. **Edit `cmd/shikigamid/wire_review.go`**:
   - Tambah import `errors`, `fmt`, dan `"github.com/NooBeeID/shikigami/internal/session"` (urut sesuai gofmt; jangan hapus import lama).
   - Di dalam `d.OnReady`, tepat setelah baris `review.RegisterActions(srv, store, reviewWireSupported(eng))` dan SEBELUM `log := d.Logger`, sisipkan blok:
     ```go
     if v, ok := d.Lookup("session.manager"); ok {
         if mgr, ok2 := v.(*session.Manager); ok2 && mgr != nil {
             review.RegisterCommentIPC(srv, reviewWireCommentDeps(mgr, eng))
         } else {
             reviewWireWarnNoManager(d)
         }
     } else {
         reviewWireWarnNoManager(d)
     }
     ```
     Jangan `return` dari callback; registrasi dan hook listener di bawahnya harus tetap jalan. Boleh disederhanakan menjadi satu variabel `mgr, mgrOK` selama perilaku sama.
   - Tambah helper privat berawalan `reviewWire` di berkas yang sama (di luar `init`, mis. setelah `reviewWireSupported`):
     - `func reviewWireWarnNoManager(d *Deps)`: bila `d.Logger != nil` -> `d.Logger.Warn("review: session.manager tidak tersedia, review.comment dilewati")`.
     - `func reviewWireCommentDeps(mgr *session.Manager, eng *status.Engine) review.CommentDeps` yang mengembalikan:
       - `Cwd`: `s, err := mgr.Get(id); if err != nil { return "", err }; return s.Cwd, nil`
       - `Write`: `err := mgr.Write(id, data); if errors.Is(err, session.ErrNotRunning) || errors.Is(err, session.ErrNotFound) { return fmt.Errorf("%w", review.ErrSessionGone) }; return err`
       - `State`: `st, ok := eng.Get(id); if !ok { return false, false }; return st.Status == status.StatusRunning, true`
   - Jangan ubah isi `reviewWireToolPath`, `reviewWireCwd`, `reviewWireSupported`, debouncer, `AddHookListener`, `AddExitListener`, atau goroutine `Prune`.

4. **Format dan build**: `gofmt -l internal/review/comment.go cmd/shikigamid/wire_review.go` (harus kosong; bila tidak, `gofmt -w` pada dua berkas itu saja), lalu `go build ./...` dari root repo. Perbaiki galat hanya di dua berkas milik fitur ini. Bila build gagal karena berkas di luar ownedPaths, laporkan tanpa memperbaikinya. `go vet ./internal/review/ ./cmd/shikigamid/` opsional.

## Perubahan

- Baru: `internal/review/comment.go`
- Diubah: `cmd/shikigamid/wire_review.go` (import `errors`, `fmt`, `session`; dua helper `reviewWire...` baru; satu blok registrasi setelah `review.RegisterActions`)

## Wiring

Fitur ini sudah menyambungkan registrasi `review.comment` lewat blok milik sendiri di `wire_review.go` (bagian dari ownedPaths, sesuai kontrak). Yang diekspos untuk langkah wiring akhir (dikerjakan agen lain, JANGAN dikerjakan di sini):
- Method IPC `review.comment` dengan params/result/error persis overview 2.2, termasuk pesan `[BUSY] agent sedang bekerja, tunggu selesai lalu kirim lagi` pada `ErrConflict`, yang dikenali UI dengan `includes('[BUSY]')`.
- Tidak ada perubahan frontend, `ReviewTab.svelte`, atau berkas lain; sambungan `sessionId` ke `DiffView` di `ReviewTab.svelte` adalah tugas langkah wiring.

## Selesai bila

- `internal/review/comment.go` berisi semua simbol di bagian Kontrak dengan signature persis, tanpa impor `session`/`status`/`cmd`, tanpa pembacaan isi file atau I/O jaringan.
- `FormatComment` menghasilkan satu baris `di <path>:<line>: <text>` tanpa newline, `\r`, ESC, atau kontrol lain; `Comment` menulis tepat satu `Write` dan mengembalikan `{ok:true, sent, enter:false}`.
- Urutan galat sesuai overview 2.2: params rusak `ErrInvalidParams`; validasi `ErrInvalid`; sesi tak ada `ErrNotFound`; path keluar cwd `ErrInvalid`; `running` `ErrConflict` `[BUSY] ...` tanpa byte ditulis; sesi berakhir saat tulis `ErrNotFound`.
- `wire_review.go` memuat blok `review.comment` setelah `RegisterActions`, log warning bila `session.manager` tidak ada tanpa menghentikan callback, dan tidak ada baris lain yang berubah.
- Guard: `go build ./...` dari root repo sukses (dan `gofmt -l` pada dua berkas kosong). Tanpa unit test, tanpa dependency baru, tanpa `git commit`.
