# Plan: Flag model di provider dan sesi

## Metadata

- Tiket: `docs/tickets/global-agents-ticket-01-agents-registry.md` (fitur `provider-model-flag`)
- Overview: `docs/plans/global-agents-ticket-01-agents-registry/overview.md` (bagian 2.4 dan 6)
- Spec: `docs/superpowers/specs/2026-11-22-global-agents-design.md`
- Module Go: `github.com/NooBeeID/shikigami`
- ownedPaths: `internal/session/types.go`, `internal/session/launcher.go`, `internal/provider/launch.go`, `internal/provider/claude.go`, `internal/provider/codex.go`
- Bergantung pada foundation: tidak (overview bagian 4: tidak ada foundation; fitur ini tidak mengimpor kode fitur lain).

## Tujuan

Menyediakan jalur model dari `session.SessionSpec` sampai baris perintah provider: `SessionSpec.Model` -> `LaunchContext.Model` -> `--model <m>` (claude) atau `-m <m>` (codex). Model kosong harus menghasilkan args byte per byte sama dengan sekarang. Tidak ada validasi nama model di aplikasi. Keluar awal proses (mis. model tidak valid) harus terlihat sebagai status `exited` + output di sesi; sesuai pembacaan kode, jalur ini sudah memenuhi (lihat langkah 6), jadi tidak perlu perubahan perilaku.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `internal/session/types.go`
- `internal/session/launcher.go`
- `internal/provider/launch.go`
- `internal/provider/claude.go`
- `internal/provider/codex.go`

Jangan menyentuh `internal/agents/`, `cmd/shikigamid/` (termasuk `wire_*.go`, `modules.go`), `internal/session/manager.go`, `frontend/`, `appconfig`, atau berkas lain. Jangan menambah unit test. Kebutuhan di luar path ini dicatat di laporan, bukan diedit.

## Kontrak

Disediakan (dipakai tiket 02 dan 03 yang hanya mengisi `Spec.Model`; lihat overview 2.4):

- `session.SessionSpec.Model string` (kosong = default provider).
- `provider.LaunchContext.Model string`.
- Claude: `--model <m>`; codex: `-m <m>`. Tanpa duplikat; `ctx.Args` yang sudah memuat `--model`, `-m`, atau `--model=...` / `-m=...` menang (tidak ditimpa, tidak ditambah).
- Helper di `internal/provider/launch.go`: `func withValueFlag(args []string, flag, alias, value string) []string` (serupa `withFlag`).

Dipakai (sudah ada, tidak diubah bentuknya): `provider.Provider.Command(ctx *LaunchContext)`, `withFlag`, `Launcher.Command(s session.SessionSpec)`, `claudeSkipPermissionsFlag`, `codexBypassFlag`.

Tidak ada pesan IPC baru, tidak ada tipe di `internal/agents`.

## Langkah

1. `internal/session/types.go`: pada `SessionSpec` (saat ini tanpa tag JSON, field satu baris `WorkspaceID, ProjectID, Cwd, ProviderID string`), tambahkan field baru terpisah, ikuti gaya tanpa tag:
   ```go
   // Model adalah nama model untuk provider; kosong = default provider.
   // Nilai tidak divalidasi aplikasi (daftar model tidak dikenal); CLI provider
   // yang menolaknya, dan keluar awal itu terlihat sebagai sesi exited.
   Model string
   ```
   `SessionSpec` tidak dipersist (`Session` yang dikembalikan tidak memuat spec; `Manager.Create` hanya membaca field spec saat membuat sesi), jadi tidak ada isu kompatibilitas data lama. Jangan menambah `Model` ke struct `Session`. Jalankan `gofmt` agar perataan field benar.

2. `internal/provider/launch.go`:
   - Di `LaunchContext` tambahkan, setelah `Args`:
     ```go
     // Model adalah nama model yang diminta (kosong = default provider). Diisi
     // Launcher dari Session.Model; Command tiap provider menerjemahkannya ke flag.
     Model string
     ```
   - Tambahkan helper setelah `withFlag`:
     ```go
     // withValueFlag mengembalikan args dengan "<flag> <value>" ditambahkan di akhir
     // bila value tidak kosong dan args belum memuat flag tersebut. Args yang sudah
     // ada MENANG: bila args memuat flag, alias (bila tidak kosong), atau bentuk
     // "flag=..." / "alias=...", args dikembalikan apa adanya (tidak digandakan,
     // tidak ditimpa). value kosong (setelah strings.TrimSpace) = args apa adanya.
     func withValueFlag(args []string, flag, alias, value string) []string
     ```
     Implementasi: `value = strings.TrimSpace(value)`; bila kosong kembalikan `args`; iterasi `args`, cocokkan `a == flag`, `alias != "" && a == alias`, `strings.HasPrefix(a, flag+"=")`, `alias != "" && strings.HasPrefix(a, alias+"=")`; bila cocok kembalikan `args`; jika tidak `return append(args, flag, value)`. Tambahkan impor `strings`. Pemanggil selalu memberi salinan `args` (lihat langkah 3-4), jadi `append` tidak mengubah `ctx.Args`.
   - Di `Launcher.Command`, ke literal `&LaunchContext{...}` tambahkan `Model: s.Model,` (trim dilakukan helper; tidak ada helper normalisasi konteks lain).

3. `internal/provider/claude.go`, `claudeProvider.Command`: setelah baris `args := withFlag(append([]string(nil), ctx.Args...), claudeSkipPermissionsFlag)` tambahkan
   ```go
   args = withValueFlag(args, "--model", "-m", ctx.Model)
   ```
   (alias `-m` ikut dideteksi agar Args pengguna yang memakai `-m` tetap menang). Letakkan sebelum blok `--mcp-config` agar urutan args lama untuk model kosong tidak berubah. Perbarui komentar doc `Command`: menyebut `--model <m>` bila `ctx.Model` terisi, dan bahwa `ctx.Args` yang sudah memuat `--model` menang.

4. `internal/provider/codex.go`, `codexProvider.Command`: ubah menjadi
   ```go
   args := withFlag(append([]string(nil), ctx.Args...), codexBypassFlag)
   args = withValueFlag(args, "-m", "--model", ctx.Model)
   return "codex", args, nil
   ```
   Perbarui komentar doc: codex memakai `-m <m>` bila `ctx.Model` terisi; `ctx.Args` yang sudah memuat `-m`/`--model` menang.

5. `internal/session/launcher.go`: `defaultLauncher` (login shell) tidak memakai model; tidak ada perubahan perilaku. Pada komentar `Launcher`/`defaultLauncher.Command` tambahkan satu kalimat bahwa `Spec.Model` sengaja diabaikan di launcher shell default dan diteruskan oleh `provider.Launcher` ke `LaunchContext.Model`. (Pengisian `LaunchContext.Model` ada di `provider/launch.go`, langkah 2; `session/launcher.go` hanya memuat antarmuka dan launcher default.) Bila implementasi aktual `session/launcher.go` saat dikerjakan sudah membangun `LaunchContext`, teruskan `Spec.Model` di sana juga.

6. Keluar awal (model tidak valid): jalur sudah diperiksa dan memenuhi syarat tanpa perubahan kode. `Manager.Create` (`internal/session/manager.go`, tidak dimiliki) memulai PTY lalu `readLoop` membaca output ke scrollback dan menerbitkan `session.output.<id>`; saat proses keluar (cepat sekalipun) `readLoop` memanggil `Wait`, menyetel `Status = StatusExited` dan `ExitCode`, menerbitkan `session.exit`, dan sesi tetap di `m.sessions` (baru dipangkas bila lebih dari `maxExitedKept = 64`). Jadi galat CLI tetap terbaca lewat scrollback/Attach dan status mencerminkan exit. Verifikasi ulang dengan membaca `readLoop` dan `pruneExited` sebelum menutup; bila ternyata ada jalur yang menelan exit awal, JANGAN edit `manager.go`, catat di laporan. Tambahkan komentar singkat di `types.go` (pada field `Model`, sudah tercakup di langkah 1) yang menyatakan perilaku ini. Jangan menambah validasi nama model.

7. Jalankan `gofmt -l internal/session internal/provider` (harus kosong) dan `go build ./...` dari root repo.

## Perubahan

Diubah (tidak ada berkas baru):

- `internal/session/types.go`: field `Model` di `SessionSpec`.
- `internal/session/launcher.go`: hanya komentar (kecuali ada pembangunan `LaunchContext` di sana).
- `internal/provider/launch.go`: field `LaunchContext.Model`, helper `withValueFlag`, `Model: s.Model` di `Launcher.Command`, impor `strings`.
- `internal/provider/claude.go`: `--model` lewat `withValueFlag`, komentar.
- `internal/provider/codex.go`: `-m` lewat `withValueFlag`, komentar.

## Wiring

Fitur ini tidak memerlukan wiring baru: `provider.Launcher` sudah menjadi launcher sesi, dan sekarang meneruskan `Spec.Model`. Yang diekspos untuk langkah wiring (overview bagian 5, butir 3, hanya verifikasi dengan membaca kode): `SessionSpec.Model` (diisi tiket 02/03), `LaunchContext.Model` (dibaca `Command` claude/codex), dan perilaku bahwa `Args` yang sudah memuat flag model menang. Jangan menyambungkan apa pun ke `internal/agents`.

## Selesai bila

- [ ] `go build ./...` dari root repo lulus (guard; tanpa unit test).
- [ ] `SessionSpec.Model` dan `LaunchContext.Model` ada; `Launcher.Command` meneruskan `s.Model`.
- [ ] Claude dengan Model "X" menghasilkan tepat satu `--model X`; codex tepat satu `-m X`; Model kosong (atau hanya spasi) menghasilkan args identik dengan sebelumnya (urutan: flag skip-permissions di depan, `--mcp-config` di akhir untuk claude).
- [ ] `ctx.Args` yang memuat `--model`, `-m`, `--model=...`, atau `-m=...` tidak digandakan dan tidak ditimpa; prioritas didokumentasikan di komentar.
- [ ] Jalur exit awal sesi diverifikasi lewat pembacaan kode dan didokumentasikan di komentar; tidak ada validasi nama model.
- [ ] `gofmt -l` pada path yang diubah kosong; tidak ada perubahan di luar lima ownedPaths (cek `git status`), khususnya tidak ada di `internal/agents`.
