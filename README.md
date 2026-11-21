# aiterm

Aplikasi desktop macOS untuk menjalankan banyak sesi AI agent (Claude Code, Codex, atau shell biasa) dalam terminal, dikelompokkan per **workspace** yang berisi beberapa project. Dibuat ringan: tanpa Electron, tanpa polling.

Terdiri dari tiga biner:

- `aitermd`: daemon Go. Mengelola workspace, sesi PTY, file, status sesi, dan mention antar sesi. Berjalan terpisah dari UI, jadi sesi tetap hidup walau window ditutup.
- `aiterm`: aplikasi Wails (WKWebView, bukan Chromium) dengan frontend Svelte + xterm.js. Menyalakan daemon otomatis bila belum berjalan.
- `aitermctl`: CLI debug untuk daemon.

Status: sembilan tiket pertama (fase 0–3 di spec) sudah ter-merge dan build lulus. Belum ada pengujian dengan agent sungguhan untuk deteksi status dari hooks dan mention antar agent; lihat [Keterbatasan](#keterbatasan-yang-diketahui).

## Fitur

- **Workspace**: satu workspace berisi banyak project. Daftar sesi milik workspace, bukan project, jadi tidak berubah saat berpindah project.
- **Sesi terminal**: banyak terminal per workspace (xterm.js), tiap sesi punya project (working directory) dan provider sendiri: `claude`, `codex`, atau `shell`. Scrollback 256 KiB per sesi dikirim ulang saat UI tersambung kembali.
- **Baca dan tulis file**: tree project (dimuat per folder), tab viewer, dan editor teks polos dengan simpan Cmd+S. `.txt` dan `.md` tampil dengan markdown view; jenis lain sebagai teks. File biner ditolak. Ada peringatan bila file berubah di disk saat tab terbuka.
- **Status sesi**: berjalan / menunggu / selesai / idle, berbasis event hooks provider (bukan polling), dengan notifikasi di UI.
- **Mention antar sesi**: dari user lewat UI (`@nama-sesi`, kirim dengan Cmd/Ctrl+Enter), dan dari agent lewat server MCP lokal (`list_sessions`, `send_message`, `read_output`). Pesan masuk antrean dan dikirim saat sesi target idle. Mode pengiriman dari agent: `auto`, `confirm_agent` (default), `user_only`.

Belum ada: knowledge/Grimoire dan penataan project otomatis (fase 4 dan 5 di spec), Windows, syntax highlighting.

## Prasyarat

- macOS (Apple Silicon diuji)
- Go (versi sesuai `go.mod`)
- Node.js dan npm
- Xcode Command Line Tools (cgo/WebKit untuk `build-app`)
- Opsional untuk menjalankan agent: `claude` dan/atau `codex` ada di `PATH`

Wails CLI tidak wajib; `build-app` memakai `go build -tags desktop,production` dengan `CGO_LDFLAGS="-framework UniformTypeIdentifiers"` (sudah diatur di Makefile; wajib bila membuild manual).

## Build

```
make build
# atau
scripts/build.sh
```

Hasil ada di `build/bin/` (`aitermd`, `aitermctl`, `aiterm`). Target lain:

- `make build-ui`: `npm install` dan build frontend (`frontend/dist`)
- `make build-daemon`: build `aitermd` dan `aitermctl`
- `make build-app`: salin `frontend/dist` ke `app/dist` lalu build app Wails
- `make run-daemon`: build dan jalankan daemon
- `make clean`: hapus artefak build
- `make check`: `go build ./...`

Tidak ada unit test; satu-satunya guard adalah build yang lulus.

## Menjalankan

```
pkill -x aitermd          # hentikan daemon lama (penting setelah build ulang)
open build/bin/aiterm     # atau: build/bin/aiterm
```

App mencari `aitermd` di folder yang sama dengan `aiterm` (lalu di `PATH`) dan menyalakannya bila belum berjalan. Daemon tetap hidup setelah window ditutup; hentikan dengan `pkill -x aitermd`. Setelah build ulang, hentikan dulu daemon lama, karena app akan tersambung ke daemon yang sedang berjalan.

Langkah pertama di window: buat workspace di sidebar, tambahkan project dengan mengetik path folder (dialog pilih folder native belum tersedia dari Wails v2), lalu buat sesi.

### Data

- Direktori data default: `~/Library/Application Support/aiterm` (mode 0700), berisi `workspaces.json`, `config.json`, socket `aitermd.sock`, dan log `aitermd.log`.
- Override dengan `AITERM_DATA_DIR` (path socket maksimal sekitar 100 byte). Untuk mencoba tanpa menyentuh data nyata: `AITERM_DATA_DIR=$(mktemp -d) build/bin/aiterm`, dijalankan langsung dari shell (bukan lewat `open`).

### Daemon dan CLI saja

```
build/bin/aitermd                 # atau: make run-daemon
build/bin/aitermctl ping          # mencetak pong
build/bin/aitermctl info
build/bin/aitermctl help
```

Perintah `aitermctl`: `fs`, `info`, `mention`, `ping`, `provider`, `session`, `status`, `workspace`. Flag `--socket <path>` mengganti socket default. Contoh: `aitermctl session new`, `aitermctl session list`, `aitermctl workspace list`.

## Arsitektur

```
 Wails UI (Svelte + xterm.js) ──JSON-RPC 2.0 lewat unix socket──┐
                                                                ▼
                                          Daemon Go (aitermd)
                                          - workspace, sesi, PTY
                                          - file service
                                          - status sesi (hooks)
                                          - mention (server MCP lokal)
                                                  │
                                                  ▼
                              claude / codex / shell (proses di PTY)
```

- IPC: JSON-RPC 2.0, satu pesan JSON per baris, di `<DataDir>/aitermd.sock`. Event server ke klien berupa notifikasi `event` dengan `topic`. Tidak ada polling.
- App Wails meneruskan semua event daemon ke UI lewat event Wails `daemon:event`.
- Modul daemon mendaftarkan diri lewat `init()` di `cmd/aitermd/wire_<modul>.go`; subcommand CLI lewat `cmd/aitermctl/cmd_<modul>.go`. Frontend memuat `frontend/src/modules/*/index.ts` otomatis dan mendaftarkan panel/tab lewat `lib/registry.ts`.

## Struktur

- `cmd/aitermd`, `cmd/aitermctl`: daemon dan CLI debug
- `internal/ipc`: server dan klien JSON-RPC
- `internal/paths`: direktori data dan path socket
- `internal/session`: manajer sesi, scrollback, antarmuka PTY (`session/pty`)
- `internal/provider`: adapter claude, codex, shell
- `internal/workspace`: workspace, project, dan konfigurasi
- `internal/fsx`: file service (tree, baca/tulis, watch)
- `internal/status`: status sesi dan Idle API
- `internal/mention`: server MCP lokal dan antrean mention
- `app/`: app Wails dan jembatan ke daemon
- `frontend/`: UI Svelte (`modules/` per kemampuan, `lib/` registry dan klien IPC)
- `scripts/`: `build.sh` dan `ticket-status.sh`
- `docs/`: spec, tiket, plan, dan laporan perf (lihat di bawah)

## Dokumen dan alur kerja

- Spec: `docs/superpowers/specs/2026-11-21-ai-terminal-design.md`
- Laporan spike RAM/CPU: `docs/perf/spike-report.md`
- Tiket: `docs/tickets/` (satu tiket per epic); plan implementasi: `docs/plans/<tiket>/`

Alur kerja memakai custom command di `.claude/commands/`:

| Command | Fungsi |
|---|---|
| `/create-ticket-from-spec` | Pecah spec menjadi tiket epic di `docs/tickets/` |
| `/plan-from-tiket <tiket>` | Buat `overview.md` dan satu plan per fitur di `docs/plans/<tiket>/` |
| `/implement-feature <tiket>` | Implementasi di git worktree, lalu squash merge ke `main` bila build lulus |
| `/ticket-status` | Tabel status semua tiket beserta dependensinya |

## Keterbatasan yang diketahui

- Hanya macOS; Windows dan Linux belum.
- Belum diuji dengan agent sungguhan: deteksi status dari hooks (skema hooks Claude Code diasumsikan dari dokumentasi) serta antrean dan deteksi idle untuk mention.
- Mention dari agent lewat MCP hanya tersambung untuk sesi `claude`; provider `codex` dan `shell` belum menerjemahkan konfigurasi MCP.
- Token MCP terikat ke sesi pada request pertama, bukan saat peluncuran.
- Daemon yang menggantung bisa menahan startup app (ping tanpa timeout di klien IPC).
- Target UI "di bawah 200 MB dengan 6 sesi" belum terukur; estimasi 189–212 MB (lihat laporan spike). Ukur ulang dengan banyak tab.
- Tampilan UI belum diverifikasi secara visual oleh pengembang otomatis; coba langsung di window.
