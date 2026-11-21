---
id: ai-terminal-ticket-02-session-pty
title: Session PTY
spec: docs/superpowers/specs/2026-11-21-ai-terminal-design.md
status: implemented
wave: 2
depends_on: [ai-terminal-ticket-01-app-base]
---
# Session PTY

## Tujuan

Epic ini mengurus sesi terminal dari ujung ke ujung: pembungkus PTY tipis, manajer sesi, scrollback terbatas, method IPC `session.*`, subcommand `aitermctl session ...`, SATU tab terminal minimal di UI (xterm.js) dengan tombol pembuat sesi, dan terakhir spike pengukuran RAM/CPU. Setelah epic ini selesai, daemon bisa menjalankan banyak sesi shell, sesi tetap hidup saat klien (UI atau CLI) putus, klien yang tersambung kembali menerima scrollback, dan pengguna bisa spawn sesi shell dari aplikasi Wails lalu melihat dan mengetik di dalamnya (bukan hanya lewat CLI debug). Spike pengukuran (spec §9 fase 0, gerbang §9b) menutup epic ini dengan laporan `docs/perf/spike-report.md`. Epic ini berdiri sendiri: begitu digabung ke hasil ticket-01 (app-base), `make build` tetap hijau dan tidak ada epic lain yang dibutuhkan untuk menjalankannya.

Alasan desain (spec §1, §4, §5): ringan, tanpa polling. Output PTY di-push lewat event IPC, tidak ada loop polling, tidak ada proses tambahan per sesi selain proses shell itu sendiri, dan akses PTY dibungkus satu interface agar port Windows (ConPTY) nanti hanya mengganti satu implementasi.

## Batasan

In scope:
- Interface `PTY` dan implementasi `creack/pty` (macOS).
- Tipe `Session`, `SessionSpec`, `Manager`, ring buffer scrollback, interface `Launcher` dengan default login shell.
- Method IPC `session.create|list|kill|write|resize|attach` dan event `session.output.<id>`, `session.exit`.
- Subcommand `aitermctl session new|list|attach|kill`.
- Pembuat ID sesi (ULID) tertanam di paket `session`.
- Tab terminal minimal di UI: modul frontend `frontend/src/modules/terminal/` (komponen `TerminalTab` berbasis xterm.js, panel kecil dengan tombol "Sesi baru"/"Tutup sesi", pendaftaran lewat `registerTabType` dan `registerPanel`). Hanya SATU tab/sesi yang ditampilkan sekaligus.
- Spike pengukuran RAM/CPU dengan 2-3 sesi PTY dan laporan `docs/perf/spike-report.md` (fitur terakhir, gerbang lanjut/tidak).
- Dependency baru: `github.com/creack/pty` (Go), `@xterm/xterm` dan `@xterm/addon-fit` (npm). Konflik `go.mod`/`package.json` antar tiket diselesaikan manual saat merge dari git worktree; cukup dicatat, tidak perlu dicegah.

Out of scope (jangan dikerjakan, milik epic lain):
- Workspace/project nyata. `WorkspaceID` dan `ProjectID` hanyalah string opak; epic ini tidak memvalidasi keberadaannya (ticket-04).
- Provider claude/codex dan `RegisterLaunchContribution` (ticket-05). Epic ini hanya menyediakan `Launcher` dan default login shell.
- Status running/waiting/done/idle dan `IsIdle/OnIdle` (ticket-08), mention antar sesi (ticket-09).
- UI shell lengkap (ticket-06): tata letak sidebar/main-tab/statusbar, sidebar workspace dan project, banyak tab sesi, daftar sesi, pemilih provider, store workspace/sesi/tab, tema gelap/terang, auto-start daemon. Epic ini hanya membuat komponen terminal dasar dan satu tab (lihat fitur `terminal-tab-ui`).
- Persistensi sesi ke disk atau bertahan saat daemon restart/reboot (spec §9b: tidak wajib).
- Windows/ConPTY, tmux.
- Unit test (guard hanya build).
- Skrip pengukuran permanen, benchmark otomatis, dan pengukuran 6 sesi penuh di UI (spike hanya manual dengan `ps`, 2-3 sesi).
- Mengedit file milik epic lain, termasuk `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go`, `internal/ipc/`, `internal/paths/`, `frontend/src/lib/registry.ts`, `frontend/src/lib/ipc.ts`, `frontend/src/main.ts`, `app/`. Satu pengecualian terbatas: `frontend/src/App.svelte` (lihat Path yang dimiliki dan Wiring).

## Kontrak

### Yang dipakai dari ticket-01 (app-base), sudah ada saat epic ini mulai

- `internal/ipc`: `Server.Register(method string, func(ctx context.Context, params json.RawMessage) (any, error))`, `Server.Publish(topic string, payload any)`, client `Call(method, params, &out)` dan `Subscribe(topicPrefix, fn)`. Kode error: `-32000 ERR_BINARY`, `-32001 ERR_CONFLICT`, `-32002 ERR_NOT_FOUND`, `-32003 ERR_INVALID`. Pakai konstanta/helper error yang diekspor `internal/ipc` (cek namanya di kode ticket-01 sebelum menulis; bila hanya ada tipe error dengan field kode, buat error dengan kode yang sesuai lewat tipe itu).
- `cmd/aitermd/modules.go`: `RegisterModule(name string, func(srv *ipc.Server, d *Deps))`. Modul sesi hanya memakai `srv`; jangan bergantung pada isi `Deps` selain yang terbukti ada di kode ticket-01.
- `cmd/aitermctl/registry.go`: `RegisterCommand(...)`. Baca signature persisnya dari kode ticket-01 dan ikuti. Cara CLI mendapat koneksi client IPC (socket `<DataDir>/aitermd.sock` lewat `internal/paths`) mengikuti helper yang disediakan registry/main aitermctl.
- Method bawaan `ping` dan `daemon.info` sudah ada.
- Frontend: `registerPanel({id, area, component, order})` dan `registerTabType({type, component})` dari `frontend/src/lib/registry.ts`; `call(method, params)` dan `onEvent(topicPrefix, fn)` (mengembalikan fungsi unsubscribe) dari `frontend/src/lib/ipc.ts`; modul dimuat otomatis dari `frontend/src/modules/<modul>/index.ts` lewat `import.meta.glob` di `main.ts`. Baca nama ekspor persis dari kode ticket-01 sebelum menulis.

### Tipe Go (paket `aiterm/internal/session`, file `types.go`)

```go
type Status string // nilai awal epic ini: "running" | "exited"

type Session struct {
    ID          string    `json:"id"`          // ULID
    WorkspaceID string    `json:"workspaceId"` // string opak, boleh kosong
    ProjectID   string    `json:"projectId"`   // string opak, boleh kosong
    Cwd         string    `json:"cwd"`
    ProviderID  string    `json:"providerId"`  // default "shell"
    Status      Status    `json:"status"`
    CreatedAt   time.Time `json:"createdAt"`   // RFC3339 di JSON
    ExitCode    *int      `json:"exitCode,omitempty"` // terisi saat exited
}

type SessionSpec struct { // masukan ke Create dan Launcher.Command
    WorkspaceID, ProjectID, Cwd, ProviderID string
    Cols, Rows uint16 // default 80x24 bila 0
    Env []string      // tambahan env dari pemanggil, format KEY=VAL
}
```

Catatan: `Status` di JSON memakai nilai `running` / `exited` milik epic ini. Nilai status agent (`running|waiting|done|idle`) adalah milik event `session.status` di ticket-08 dan tidak dicampur di sini. Nilai `running` sengaja sama dengan salah satu nilai itu; konsumen membedakan lewat nama method/event.

### Launcher (file `launcher.go`)

```go
type Launcher interface {
    Command(s SessionSpec) (cmd string, args []string, env []string)
}
func DefaultLauncher() Launcher // login shell
func (m *Manager) SetLauncher(l Launcher)
```

- Default: `cmd = $SHELL` (fallback `/bin/zsh`), `args = ["-l"]`, `env = os.Environ()` ditambah `TERM=xterm-256color`, `COLORTERM=truecolor`, lalu `s.Env`.
- `SetLauncher` aman dipanggil sebelum sesi pertama dibuat; ticket-05 akan memanggilnya dari wiring-nya sendiri. Epic ini tidak mengatur siapa yang memanggil.

### Antarmuka PTY (paket `aiterm/internal/session/pty`)

```go
type Size struct{ Cols, Rows uint16 }
type PTY interface {
    Start(cmd string, args []string, env []string, cwd string, size Size) error
    Read(p []byte) (int, error)
    Write(p []byte) (int, error)
    Resize(size Size) error
    Close() error          // menutup master fd dan membunuh proses bila masih hidup; idempoten
    Wait() (exitCode int, err error) // blok sampai proses selesai
}
func New() PTY // implementasi creack/pty (pty_darwin.go atau pty_unix.go)
```

### Manager (file `manager.go`)

```go
func NewManager(pub func(topic string, payload any)) *Manager // pub = srv.Publish
func (m *Manager) Create(spec SessionSpec) (*Session, error)
func (m *Manager) List() []Session           // urut CreatedAt naik
func (m *Manager) Get(id string) (*Session, error)
func (m *Manager) Kill(id string) error
func (m *Manager) Write(id string, data []byte) error
func (m *Manager) Resize(id string, cols, rows uint16) error
func (m *Manager) Attach(id string) (AttachResult, error)
type AttachResult struct { Session Session; Scrollback []byte; Offset uint64 }
```

Error domain: sesi tidak ada -> `ErrNotFound`; masukan salah (cwd tidak ada, size nol sesudah default, dst.) -> `ErrInvalid`; menulis/resize ke sesi yang sudah exited -> `ErrNotRunning`. Lapisan IPC memetakan ke kode `-32002`, `-32003`, dan `-32001` (ERR_CONFLICT) berturut-turut.

### Scrollback (file `scrollback.go`)

Ring buffer byte berkapasitas tetap 256 KiB (`const ScrollbackSize = 256 * 1024`), menyimpan byte mentah terbaru. Menyimpan penghitung `total uint64` (jumlah byte kumulatif yang pernah masuk). `Snapshot() (data []byte, offset uint64)` mengembalikan salinan isi saat ini dan `total` pada saat itu, atomik terhadap `Append`. Aman dipakai konkuren (mutex).

### Method IPC

Semua `id` berupa string ULID. Nama field JSON camelCase.

| Method | Params | Result |
|---|---|---|
| `session.create` | `{workspaceId?, projectId?, cwd?, providerId?, cols?, rows?, env?: string[]}` | `Session` |
| `session.list` | `{}` atau kosong | `{sessions: Session[]}` |
| `session.kill` | `{id}` | `{ok: true}` |
| `session.write` | `{id, data}` (`data` = base64 byte mentah) | `{ok: true}` |
| `session.resize` | `{id, cols, rows}` | `{ok: true}` |
| `session.attach` | `{id}` | `{session: Session, scrollback: string (base64), offset: number}` |

Perilaku `cwd`: kosong -> home user. Path harus ada dan berupa direktori, kalau tidak `ERR_INVALID`.
`session.kill` pada sesi yang sudah exited: sukses dan sesi dihapus dari daftar; pada sesi hidup: kirim SIGHUP lalu SIGKILL bila belum selesai dalam 2 detik, lalu tunggu `Wait`. Sesi yang berakhir sendiri tetap ada di `list` berstatus `exited` sampai di-`kill` (agar scrollback terakhir masih bisa dibaca); batasi maksimal 64 sesi exited yang disimpan, yang terlama dibuang.

### Event IPC (lewat `srv.Publish`, dikirim ke semua klien yang subscribe, tanpa polling)

- Topik `session.output.<id>`: payload `{"data": "<base64>", "offset": <uint64>}`. `offset` = total byte kumulatif SESUDAH potongan ini. Dipush dari goroutine pembaca PTY per sesi, tiap hasil `Read` (buffer baca 32 KiB). Tidak ada timer/polling.
- Topik `session.exit`: payload `{"id": string, "exitCode": number}`, dipush sekali ketika proses selesai (baik berakhir sendiri maupun di-kill).

Alur klien yang sambung kembali: panggil `session.attach` untuk dapat `scrollback` + `offset`, lalu proses event `session.output.<id>` dan buang event yang `offset` <= offset attach (sudah tercakup scrollback). Subscribe sebaiknya dipasang SEBELUM memanggil attach, supaya tidak ada celah. `attach` tidak mengubah state sesi (tidak ada konsep "terlampir" di sisi daemon), jadi banyak klien boleh attach bersamaan.

### Kontrak frontend (modul `terminal`)

- Komponen `frontend/src/modules/terminal/TerminalTab.svelte`: prop tunggal `sessionId: string`. Saat mount: `new Terminal` + `FitAddon`; subscribe `session.output.<id>` dan `session.exit` SEBELUM `session.attach`; tahan event ke buffer; tulis scrollback (base64 -> bytes); alirkan buffer dengan membuang event ber-`offset` <= offset attach; `term.onData` -> `session.write(id, base64)`; `ResizeObserver` + fit -> `session.resize` (debounce pendek). Saat destroy: unsubscribe dan `term.dispose()` TANPA membunuh sesi. Saat `session.exit`: tulis baris "[sesi berakhir]" dan matikan input.
- Pewarnaan: membaca CSS variables `--term-bg`, `--term-fg`, `--term-cursor` dengan fallback bawaan (gelap) bila belum didefinisikan. Ticket-06 hanya mendefinisikan variabel itu; ia tidak mengubah komponen.
- Helper `frontend/src/modules/terminal/terminal-client.ts` (attach/subscribe/write/resize dan konversi base64) dipisah dari komponen agar ticket-06 bisa memakainya ulang.
- Pendaftaran di `frontend/src/modules/terminal/index.ts`: `registerTabType({type: 'session', component: TerminalTab})` dan `registerPanel({id: 'terminal-quick', area: 'main-tab', component: QuickTerminal, order: 50})`. `QuickTerminal.svelte` memuat tombol "Sesi baru" (`session.create` tanpa parameter: shell login, cwd home) dan "Tutup sesi" (`session.kill`), serta menampilkan `TerminalTab` untuk satu sesi aktif. Saat mount ia memanggil `session.list` dan, bila ada sesi `running`, meng-attach yang terbaru (membuktikan sesi tetap hidup saat window ditutup). "Sesi baru" menggantikan tab yang tampil; sesi lama tetap hidup di daemon.

## Fitur

Fitur `pty-wrapper`, `session-manager`, dan `session-ipc-ctl` bisa dikerjakan paralel setelah kontrak di atas disepakati, dengan urutan ketergantungan kompilasi: `pty-wrapper` dan `session-manager` hanya bergantung pada kontrak (interface `pty.PTY` bisa dipakai manager lewat interface saja); `session-ipc-ctl` memakai API `Manager`. Fitur `terminal-tab-ui` hanya bergantung pada kontrak method/event IPC dan kontrak frontend di atas, jadi juga bisa paralel dengan fitur Go; uji end-to-end-nya baru bisa dilakukan setelah `session-ipc-ctl` digabung. Fitur `spike-measure` WAJIB dikerjakan terakhir, setelah keempat fitur lain tergabung. Bila dikerjakan paralel, tiap agent berpegang pada tanda tangan di bagian Kontrak. Penggabungan terakhir harus lolos `make build`.

### pty-wrapper: Antarmuka PTY tipis

Deskripsi: paket `internal/session/pty` berisi interface `PTY` (lihat Kontrak) dan satu implementasi memakai `github.com/creack/pty`.

- `Start`: buat `exec.Cmd` dari `cmd,args`, set `Env`, `Dir = cwd`, mulai dengan `pty.StartWithSize` memakai `Size`. Set `SysProcAttr{Setsid: true, Setctty: true}` bila belum dilakukan oleh library, agar kill bisa menarget group.
- `Read`/`Write`: delegasi ke master fd.
- `Resize`: `pty.Setsize`.
- `Close`: tutup master fd, kirim SIGHUP ke process group, idempoten (aman dipanggil dua kali, tanpa panic).
- `Wait`: `cmd.Wait()` dan mengembalikan exit code (bila di-signal, kode `128+signal`).
- Konstruktor `New() PTY`. Tidak ada tipe creack/pty yang bocor ke luar paket ini.
- Tambahkan dependency `github.com/creack/pty` ke `go.mod`/`go.sum` (lihat bagian Path yang dimiliki untuk pengecualian).

Acceptance criteria:
- [ ] `go build ./internal/session/pty/...` berhasil.
- [ ] Interface `PTY` memiliki tepat metode `Start, Read, Write, Resize, Close, Wait` dengan tanda tangan di Kontrak.
- [ ] Tidak ada import `creack/pty` di luar `internal/session/pty/`.
- [ ] `Close` idempoten dan tidak memblokir selamanya bila proses sudah selesai.
- [ ] Tidak ada goroutine bocor di dalam paket ini (pembacaan dilakukan pemanggil).

ownedPaths: `internal/session/pty/`. Paralel dengan fitur lain.

### session-manager: Manajer sesi dan scrollback

Deskripsi: paket `internal/session` (file `types.go`, `launcher.go`, `scrollback.go`, `manager.go`) berisi tipe, `Launcher`, ring buffer, dan `Manager`.

- `Create`: validasi spec (cwd, default size 80x24, default `ProviderID` = `"shell"`), buat ID ULID sendiri (48-bit timestamp ms + 80-bit acak `crypto/rand`, encoding Crockford base32, 26 karakter; fungsi privat `newID()` di `types.go`), panggil `Launcher.Command(spec)`, jalankan `pty.New().Start(...)`, daftarkan sesi, lalu jalankan SATU goroutine pembaca per sesi: `Read` -> `scrollback.Append` -> `pub("session.output."+id, {data: base64, offset})`. Saat `Read` error/EOF: panggil `Wait`, set `Status=exited` dan `ExitCode`, `pub("session.exit", {id, exitCode})`, tutup PTY.
- Penulisan ke scrollback dan penerbitan event dilakukan berurutan di goroutine yang sama supaya urutan `offset` konsisten dan `Attach` (snapshot di bawah mutex yang sama dengan `Append`) tidak melewatkan atau menggandakan byte.
- `Write` meneruskan byte ke PTY (serialisasi dengan mutex per sesi). `Resize` memanggil `PTY.Resize` dan menyimpan ukuran.
- `Kill`: lihat perilaku di Kontrak. Sesi dihapus dari map setelah kill.
- `List` mengembalikan salinan (bukan pointer ke state internal), urut `CreatedAt`.
- Sesi bertahan saat tidak ada klien: Manager tidak bergantung pada koneksi IPC sama sekali.
- `Manager.Shutdown()` (publik) membunuh semua sesi hidup; dipanggil oleh wiring saat daemon berhenti bila ada hook yang tersedia, bila tidak ada, cukup disediakan.
- Tidak ada polling: tidak ada `time.Ticker` untuk membaca output atau memeriksa status.
- Batas memori: scrollback 256 KiB per sesi, buffer baca 32 KiB dipakai ulang.

Acceptance criteria:
- [ ] `go build ./internal/session/...` berhasil.
- [ ] `Manager` memiliki `Create/List/Get/Kill/Write/Resize/Attach` sesuai Kontrak.
- [ ] Ring buffer membuang byte terlama saat melewati 256 KiB, `Snapshot` mengembalikan `offset` kumulatif yang tidak pernah turun.
- [ ] `session.exit` terbit tepat sekali per sesi, termasuk saat di-kill.
- [ ] Default launcher memakai `$SHELL -l` dengan fallback `/bin/zsh` dan menyetel `TERM=xterm-256color`.
- [ ] Hanya ada satu goroutine pembaca per sesi dan tidak ada ticker/polling.
- [ ] Paket `session` tidak mengimpor `internal/ipc` (diberi `pub` lewat parameter), supaya tidak terjadi siklus dan tetap mudah diganti.
- [ ] ID berupa 26 karakter ULID, urut menurut waktu pembuatan.

ownedPaths: `internal/session/manager.go`, `internal/session/scrollback.go`, `internal/session/types.go`, `internal/session/launcher.go`. Memakai `pty.PTY` dari fitur pty-wrapper; bisa dikerjakan paralel selama tanda tangan interface di Kontrak dipatuhi (kompilasi penuh baru terjadi setelah pty-wrapper ada).

### session-ipc-ctl: Method IPC session dan CLI

Deskripsi: menyambungkan `Manager` ke IPC dan menyediakan perintah uji manual.

- `internal/session/ipc.go`: fungsi `RegisterIPC(srv *ipc.Server, m *Manager)` mendaftarkan enam method sesuai tabel Kontrak (decode params JSON, validasi, panggil Manager, petakan error domain ke kode `-32001/-32002/-32003`). Catatan: file ini boleh mengimpor `internal/ipc`; paket `session` selain file ini tidak.
- `cmd/aitermd/wire_session.go`: `init()` memanggil `RegisterModule("session", func(srv *ipc.Server, d *Deps){ m := session.NewManager(srv.Publish); session.RegisterIPC(srv, m) })`. Simpan instance Manager di variabel paket `cmd/aitermd` hanya bila ticket lain memerlukannya nanti; saat ini tidak perlu.
- `cmd/aitermctl/cmd_session.go`: `init()` memanggil `RegisterCommand(...)` untuk grup `session`:
  - `aitermctl session new [--cwd DIR] [--workspace ID] [--project ID] [--provider ID] [--cols N] [--rows N]` mencetak ID sesi dan JSON `Session`.
  - `aitermctl session list` mencetak tabel sederhana: `ID  STATUS  CWD  PROVIDER`.
  - `aitermctl session attach <id>`: subscribe `session.output.<id>` SEBELUM memanggil `session.attach`, tulis scrollback ke stdout, lanjutkan menulis output event (membuang yang `offset` <= offset attach), serta mengirim stdin (mode raw terminal via `golang.org/x/term` bila tersedia di go.mod; bila tidak, cukup mode baris dengan `session.write`) ke `session.write`. Menangani `session.exit` (keluar dengan kode exit sesi). Lepas dengan `Ctrl+]` tanpa mematikan sesi. Kirim `session.resize` pada ukuran terminal awal dan saat SIGWINCH.
  - `aitermctl session kill <id>`.
  - Bila `golang.org/x/term` belum ada di `go.mod`, jangan menambahnya; gunakan `syscall`/`stty` melalui `os/exec` untuk mengatur raw mode, atau mode baris. Tujuan hanya uji manual.
- Event dari server diterima lewat `client.Subscribe("session.output."+id, fn)`.

Acceptance criteria:
- [ ] `go build ./...` berhasil pada gabungan ticket-01 + epic ini.
- [ ] `aitermd` mendaftarkan `session.create/list/kill/write/resize/attach` (muncul bila dipanggil; method tak dikenal tidak lagi untuk nama-nama ini).
- [ ] `session.attach` mengembalikan `scrollback` base64 dan `offset`; output berikutnya datang sebagai event tanpa polling.
- [ ] `session.write` dengan `data` bukan base64 valid -> `ERR_INVALID`; `id` tak dikenal -> `ERR_NOT_FOUND`; menulis ke sesi exited -> `ERR_CONFLICT`.
- [ ] `aitermctl session new` + `attach` memberi shell interaktif yang bisa menjalankan `ls`, dan melepas dengan Ctrl+] membuat sesi tetap hidup (`session list` masih menampilkannya `running`).
- [ ] Attach ulang setelah lepas menampilkan scrollback sebelumnya.
- [ ] Tidak ada file di luar ownedPaths yang diubah.

ownedPaths: `internal/session/ipc.go`, `cmd/aitermd/wire_session.go`, `cmd/aitermctl/cmd_session.go`. Bergantung pada API `Manager` dari session-manager untuk kompilasi; dapat dikerjakan paralel dengan tanda tangan di Kontrak.

### terminal-tab-ui: Tab terminal minimal di UI

Deskripsi: modul frontend `terminal` yang membuat sesi shell bisa dibuat dan dipakai dari aplikasi Wails. Berisi `TerminalTab.svelte`, `terminal-client.ts`, `QuickTerminal.svelte`, dan `index.ts` sesuai Kontrak frontend. Memakai `@xterm/xterm` dengan renderer bawaan (canvas/DOM; jangan menambah addon berat seperti webgl) dan `@xterm/addon-fit`; impor `@xterm/xterm/css/xterm.css` dari komponen. Menambah dependency `@xterm/xterm` dan `@xterm/addon-fit` ke `frontend/package.json` (dan lockfile) hanya sebanyak yang diperlukan, tanpa mengubah baris lain. Karena `frontend/src/App.svelte` placeholder ticket-01 hanya menghitung panel dan belum merender komponennya, fitur ini membuat satu perubahan minimal tambahan di file itu: merender komponen panel area `main-tab` dari registry (satu blok `{#each}`), tanpa menghapus isi placeholder lain. Ticket-06 menggantikan `App.svelte` seluruhnya, jadi perubahan ini sementara dan tidak boleh ditambah logika lain.

Batas dengan ticket-06: fitur ini membuat KOMPONEN terminal dan SATU tab saja. Ticket-06 tidak membuat ulang terminal; ia membungkus `TerminalTab` ke banyak tab, store, sidebar, dan layout, serta menghapus panel `terminal-quick` (lihat tiket 06). Tidak ada tab bar, daftar sesi, pemilih provider/project, atau store di fitur ini.

Acceptance criteria:
- [ ] `cd frontend && npm install && npm run build` sukses; `make build` hijau.
- [ ] Dengan `aitermd` berjalan, menekan "Sesi baru" di window Wails memunculkan terminal berisi prompt shell login; mengetik `ls` menampilkan hasilnya (input dan output berjalan).
- [ ] Mengubah ukuran window mengubah ukuran PTY (`session.resize`); `stty size` mencerminkan ukuran baru.
- [ ] Menutup lalu membuka kembali window (daemon tetap hidup) menampilkan sesi terbaru yang masih `running` dengan scrollback-nya.
- [ ] `session.exit` (mis. mengetik `exit`) menampilkan "[sesi berakhir]" tanpa error dan input mati.
- [ ] Subscription dan instance `Terminal` dibersihkan saat komponen di-destroy; sesi tidak terbunuh.
- [ ] Tipe tab `session` dan panel `terminal-quick` terdaftar lewat registry tanpa mengedit `lib/registry.ts`, `lib/ipc.ts`, `main.ts`.
- [ ] Tanpa `setInterval`/polling; hanya event IPC.

ownedPaths: `frontend/src/modules/terminal/`. Pengecualian terbatas: `frontend/package.json` (+ lockfile) hanya untuk menambah dua dependency xterm, dan `frontend/src/App.svelte` hanya untuk merender panel `main-tab`. Tumpang tindih dengan ticket-01/06 pada file-file itu dicatat dan diselesaikan manual saat merge.

### spike-measure: Spike pengukuran RAM/CPU dan gerbang (fitur TERAKHIR)

Deskripsi: setelah keempat fitur lain tergabung dan `make build` hijau, ukur biaya sumber daya nyata lalu tulis `docs/perf/spike-report.md` (spec §9 fase 0, §9b). Hanya pengukuran manual dengan `ps`; tidak ada skrip atau kode permanen, dan tidak ada file aplikasi yang diubah.

Langkah:
1. Jalankan `build/bin/aitermd` dan catat baseline idle (0 sesi): RSS (`ps -o rss= -p PID`, KB dibagi 1024) dan CPU.
2. Buat 2 lalu 3 sesi shell (`aitermctl session new` dan/atau tombol "Sesi baru" di app), tunggu idle beberapa detik, catat RSS daemon di tiap titik (0, 2, 3 sesi) lalu hitung tambahan per sesi.
3. Jalankan aplikasi Wails (`build/bin/aiterm`) dengan satu tab terminal terbuka. Ukur RSS proses app ditambah proses WebKit anaknya (`com.apple.WebKit.WebContent`/`Networking` milik app, diidentifikasi lewat pohon proses `ps -o pid,ppid,rss,command`).
4. CPU idle: gunakan CPU time kumulatif (`ps -o time=`) pada dua sampel berjarak minimal 5 detik, hitung `delta_cpu / delta_wall * 100`; jangan memakai `%cpu` (rata-rata sejak start) sebagai nilai idle. Sampel sekali per titik, bukan pemantauan berkelanjutan.
5. Tulis laporan: metadata mesin (`sw_vers`, `sysctl -n hw.memsize`, `sysctl -n machdep.cpu.brand_string`), perintah yang dipakai, tabel angka mentah, perbandingan dengan target spec §7, catatan keterbatasan, dan keputusan gerbang. Perbandingan dengan Orca opsional dan manual.

Target (spec §7, semuanya **[call]**, perlu diverifikasi): daemon idle < 30MB RSS; tambahan < ~10MB per sesi di luar proses agent; UI < 200MB dengan 6 sesi; CPU idle ~0% (lulus bila < 1,0%).

Keterbatasan yang wajib dinyatakan di laporan: spike ini hanya 2-3 sesi PTY dan satu tab terminal di UI. Target "UI < 200MB dengan 6 sesi" tidak bisa diukur langsung; laporkan angka UI yang terukur (satu tab terminal) sebagai indikatif, jelaskan ekstrapolasinya, dan tandai bahwa pengukuran 6 sesi diulang setelah ticket-06 bila hasilnya mendekati batas.

Keputusan gerbang (spec §9b) harus dinyatakan eksplisit dalam bentuk checklist dengan alasan dan tindak lanjut: (a) semua target lulus atau mendekati: lanjut dengan Wails; (b) satu atau dua target meleset tipis (kurang dari kira-kira 1,5x): lanjut dengan daftar optimasi (mis. buang instance terminal tab tak aktif, kecilkan scrollback) lalu ukur ulang; (c) UI jauh meleset (lebih dari 1,5x target atau CPU idle terus naik): pertimbangkan UI Swift native + SwiftTerm dengan daemon Go tidak berubah.

Acceptance criteria:
- [ ] `docs/perf/spike-report.md` ada dan memuat metadata mesin, perintah ukur, dan tabel angka hasil pengukuran sungguhan (tidak ada angka karangan atau placeholder yang tersisa).
- [ ] Tabel membandingkan daemon idle, tambahan per sesi, UI, dan CPU idle terhadap target spec §7 dengan status lulus/meleset per baris.
- [ ] Keterbatasan (2-3 sesi, satu tab UI, ekstrapolasi ke 6 sesi) tertulis.
- [ ] Keputusan gerbang lanjut/tidak dinyatakan dengan satu pilihan terpilih (a/b/c), alasan, dan tindak lanjut.
- [ ] Tidak ada file di luar `docs/perf/spike-report.md` yang dibuat atau diubah; `make build` tetap hijau.

ownedPaths: `docs/perf/spike-report.md`. Dikerjakan terakhir, tidak paralel dengan fitur lain di epic ini.

## Path yang dimiliki

Hanya ini yang boleh disentuh:
- `internal/session/` (termasuk `internal/session/pty/`, `types.go`, `launcher.go`, `scrollback.go`, `manager.go`, `ipc.go`)
- `cmd/aitermd/wire_session.go`
- `cmd/aitermctl/cmd_session.go`
- `frontend/src/modules/terminal/` (TerminalTab, terminal-client, QuickTerminal, index.ts)
- `docs/perf/spike-report.md`

Satu pengecualian terbatas yang tak terhindarkan: `go.mod` dan `go.sum` (milik ticket-01) boleh diubah HANYA untuk menambah `require github.com/creack/pty` (versi stabil terbaru, mis. v1.1.x) beserta checksum-nya lewat `go get github.com/creack/pty` lalu `go mod tidy`. Dilarang mengubah baris lain. Epic wave 2 lain bisa juga menambah require sendiri; bila terjadi konflik penggabungan pada file ini, gabungkan kedua set baris `require` dan jalankan `go mod tidy`.

Pengecualian terbatas lain (frontend): `frontend/package.json` (dan lockfile `package-lock.json`, milik ticket-01) boleh diubah HANYA untuk menambah dependency `@xterm/xterm` dan `@xterm/addon-fit`; `frontend/src/App.svelte` (milik ticket-01) boleh diubah HANYA untuk merender panel area `main-tab` dari registry. Konflik penggabungan pada `go.mod`/`go.sum`/`package.json`/`package-lock.json`/`App.svelte` antar tiket tidak dicegah dan diselesaikan manual saat merge dari git worktree (gabungkan kedua sisi, jalankan `go mod tidy` dan `npm install`). Ticket-06 menggantikan `App.svelte` sepenuhnya, sehingga perubahan di sini otomatis tertimpa.

## Wiring

- Tidak ada edit ke file bersama. Daemon memuat modul lewat `init()` di `cmd/aitermd/wire_session.go` yang memanggil `RegisterModule("session", ...)` (dari `cmd/aitermd/modules.go`, ticket-01). CLI memuat lewat `init()` di `cmd/aitermctl/cmd_session.go` yang memanggil `RegisterCommand(...)` (ticket-01).
- Frontend: `frontend/src/modules/terminal/index.ts` ditemukan otomatis oleh `import.meta.glob` (ticket-01) dan memanggil `registerTabType({type:'session', ...})` serta `registerPanel({id:'terminal-quick', area:'main-tab', ...})`. Tidak ada edit ke `registry.ts`, `ipc.ts`, atau `main.ts`. Ticket-06 (ui-shell) memakai ulang `TerminalTab` dan `terminal-client.ts` dari modul ini (jangan dibuat ulang) dan menghapus panel `terminal-quick`; bentuk event `{data, offset}` dan alur attach-setelah-subscribe sudah diimplementasikan di sini.
- Ticket-05 (provider-adapters) akan memanggil `Manager.SetLauncher` dan memakai `SessionSpec`/`Launcher`; epic ini tidak melakukan apa pun untuk mereka selain mengekspos API tersebut. Konsumen lain yang butuh instance `Manager` (status, mention) mengambilnya lewat method IPC atau hook yang mereka tambahkan sendiri di wave berikutnya; epic ini tidak mengekspos singleton global.
- `make build` harus tetap lolos: paket Go baru ikut terkompilasi lewat `go build ./...`, dan modul frontend `terminal` ikut di-bundle oleh `vite build`.
- `spike-measure` tidak punya wiring; hanya menghasilkan dokumen.

## Guard

Cukup build berhasil, tanpa unit test:
- `go build ./...` (harus lolos pada gabungan ticket-01 + epic ini).
- `make build` (go build + build wails/vite dari ticket-01) harus tetap hijau, termasuk modul frontend `terminal` dan dependency xterm.
- `go vet ./internal/session/... ./cmd/aitermd/... ./cmd/aitermctl/...` disarankan sebagai pemeriksaan tambahan, bukan syarat.
- Uji manual opsional (di luar guard): jalankan `aitermd`, lalu `aitermctl session new`, `session list`, `session attach <id>`, `session kill <id>`; buka `build/bin/aiterm`, tekan "Sesi baru", ketik `ls`.
- `spike-measure` bukan guard build: hasilnya dokumen laporan dan keputusan gerbang manusia.
