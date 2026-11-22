# Monitor Resource Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tampilkan total CPU dan memori semua proses Shikigami di bilah atas, dengan dropdown kecil berisi rincian per proses.

**Architecture:** Paket baru `internal/resource` di daemon menghitung snapshot dari satu panggilan `ps`, dengan CPU dari selisih CPU time antar-sampel. Daemon membuka IPC `resource.snapshot`; UI memanggilnya tiap 5 detik hanya saat jendela terlihat. UI menampilkan item bilah atas dengan dropdown.

**Tech Stack:** Go 1.26 (stdlib saja), Svelte 5 + TypeScript, Wails v2 (cgo di macOS).

**Spec:** `docs/superpowers/specs/2026-11-22-resource-monitor-design.md`

## Global Constraints

- Tanpa dependency baru (tidak memakai gopsutil); `ps` atau `libproc` dari Go.
- Interval sampling 5 detik; berhenti saat jendela tersembunyi atau UI tidak tersambung.
- CPU = selisih CPU time dibagi selisih waktu dinding × 100, bukan kolom `%cpu` dari `ps`.
- Memori = RSS (bias ke atas karena halaman shared).
- Memakai IPC daemon–UI yang sudah ada (`ipc.Server.Register`).
- Teks UI berbahasa Indonesia; angka memakai koma desimal ("1,4 GB", "< 0,1%").
- Tampilan: ringkasan di bilah atas, klik membuka dropdown kecil (bukan panel). Mengikuti pola `frontend/src/modules/topbar/SettingsButton.svelte`.

## Review Focus

- Sesi berakhir di antara pendaftaran PID dan hasil `ps`: tidak ada baris untuknya, tidak ada panic.
- PID dipakai ulang atau CPU time lebih kecil dari sampel sebelumnya: delta dibatasi ke 0, tidak pernah negatif.
- `ps` gagal: IPC mengembalikan error; UI mempertahankan angka terakhir, tidak crash.
- Tanpa satu pun sesi: snapshot tetap berisi daemon dan app.
- Aplikasi dijalankan di luar Wails (`inWails` false, mis. `vite dev`): tidak ada panggilan, item tersembunyi.

---

### Task 1: Paket sampler `internal/resource`

**Files:**
- Create: `internal/resource/types.go`, `internal/resource/sampler.go`, `internal/resource/ps.go`
- Test: `internal/resource/sampler_test.go`, `internal/resource/ps_test.go`

**Interfaces:**
- Produces:
  - `type RawProc struct { PID, PPID int; RSSKB uint64; CPUSeconds float64; Command string }`
  - `type Lister func() ([]RawProc, error)`
  - `type Root struct { Label, Kind, SessionID string; PID int; Extra []int }` — `Kind` ∈ `"daemon" | "app" | "session"`; `Extra` adalah PID tambahan yang dihitung ke root ini beserta turunannya.
  - `type Proc struct { Label, Kind, SessionID string; PID, Count int; RSSBytes uint64; CPUPercent float64 }` dengan tag JSON `label, kind, sessionId, pid, count, rssBytes, cpuPercent`.
  - `type Snapshot struct { At int64; TotalRSSBytes uint64; TotalCPUPercent float64; Procs []Proc }` dengan tag JSON `at` (unix ms), `totalRssBytes`, `totalCpuPercent`, `procs`.
  - `func NewSampler(list Lister, now func() time.Time) *Sampler`
  - `func (s *Sampler) Sample(roots []Root, procs []RawProc) Snapshot`
  - `func PSLister() ([]RawProc, error)`
  - `func ParsePSTime(s string) (float64, error)`

- [ ] **Step 1: Tulis tes `ps_test.go`** — `TestParsePSTime` dengan kasus `"0:01.50"→1.5`, `"1:02.25"→62.25`, `"1:02:03.00"→3723`, `"2-01:00:00"→176400`, dan `"abc"` mengembalikan error. `TestParsePSLine` memastikan baris `"  123   1  2048  0:01.50 /usr/bin/zsh -l"` menjadi `RawProc{PID:123, PPID:1, RSSKB:2048, CPUSeconds:1.5, Command:"/usr/bin/zsh -l"}`.
- [ ] **Step 2: Jalankan, pastikan gagal.** `go test ./internal/resource/ -run 'PS' -v` → FAIL (undefined).
- [ ] **Step 3: Implementasi `ParsePSTime` dan `PSLister`** di `ps.go`. `PSLister` menjalankan `ps -axo pid=,ppid=,rss=,time=,command=` lewat `exec.Command` dan mem-parse tiap baris dengan fungsi `parsePSLine(line string) (RawProc, bool)`; baris tak valid dilewati.
- [ ] **Step 4: Jalankan tes PS, pastikan lulus.**
- [ ] **Step 5: Tulis tes `sampler_test.go`** dengan `Lister` palsu dan jam palsu:
  - `TestSampleAggregatesChildrenIntoRoot`: root PID 10 punya anak 11 dan cucu 12 → `Count == 3`, `RSSBytes` = jumlah RSSKB × 1024.
  - `TestSampleAttributesToNearestRoot`: daemon (PID 1) mengandung anak 20 yang juga root sesi; proses 20 dan turunannya dihitung ke sesi, bukan ke daemon, dan total tidak menghitung ganda.
  - `TestSampleCPUFromDelta`: sampel pertama `CPUPercent == 0`; sampel kedua 10 detik kemudian dengan CPU time naik 5 detik → `50`.
  - `TestSampleCPUNeverNegative`: CPU time turun (PID dipakai ulang) → `0`.
  - `TestSampleMissingRootPID`: root yang PID-nya tidak ada di tabel proses tidak menghasilkan baris.
  - `TestSampleExtraPIDs`: PID di `Root.Extra` beserta turunannya masuk ke root tersebut.
  - `TestSampleTotals`: `TotalRSSBytes` dan `TotalCPUPercent` sama dengan jumlah baris.
- [ ] **Step 6: Jalankan, pastikan gagal.** `go test ./internal/resource/ -v` → FAIL.
- [ ] **Step 7: Implementasi `Sampler`** di `sampler.go`. Bangun peta anak dari `PPID`; tetapkan tiap proses ke root terdekat (leluhur-atau-diri) lewat telusur naik; simpan `prev map[int]struct{cpu float64; at time.Time}` antar pemanggilan; delta CPU per proses dijumlah per root, dibatasi ≥ 0, dibagi delta waktu dinding (lindungi pembagian nol). Hapus entri `prev` untuk PID yang sudah hilang. Mutex melindungi `prev`. Hasil `Procs` diurutkan: daemon, app, lalu sesi menurun menurut `RSSBytes`.
- [ ] **Step 8: Jalankan semua tes paket, pastikan lulus.** `go test ./internal/resource/ -v` → PASS.
- [ ] **Step 9: Commit.** `git add internal/resource && git commit -m "feat(resource): sampler CPU dan memori per proses"`

### Task 2: PID sesi dan IPC `resource.snapshot`

**Files:**
- Modify: `internal/session/pty/pty.go` (interface `PTY`), `internal/session/pty/pty_unix.go`, `internal/session/manager.go`
- Create: `internal/resource/ipc.go`, `cmd/shikigamid/wire_resource.go`

**Interfaces:**
- Consumes: `resource.Sampler`, `resource.Root`, `resource.PSLister` (Task 1).
- Produces:
  - `PTY.Pid() int` — 0 bila belum start.
  - `func (m *Manager) RunningPids() map[string]int` — ID sesi → PID, hanya sesi `running`.
  - `type SessionSource interface { RunningPids() map[string]int }` di paket `resource`.
  - `func RegisterIPC(srv *ipc.Server, s *Sampler, src SessionSource, extra func(appPid int, procs []RawProc) []int)`; method `resource.snapshot`, params `{"appPid": number?}` (opsional), hasil `Snapshot`.

- [ ] **Step 1: Tambah `Pid() int` ke interface `PTY`** dan implementasikan di `unixPTY` (kunci `mu`, kembalikan `cmd.Process.Pid` atau 0). Perbarui semua implementasi lain dari `PTY` yang ditemukan `go build ./...`.
- [ ] **Step 2: Tambah `Manager.RunningPids`** — iterasi `m.sessions` di bawah `RLock`, hanya entri `running()` dengan `Pid() > 0`.
- [ ] **Step 3: Tulis `internal/resource/ipc.go`.** Handler membangun `Root` daemon (`os.Getpid()`, label `"Daemon"`), satu `Root` per sesi (label `"Sesi <8 karakter pertama ID>"`, `SessionID` terisi), dan bila `appPid > 0` satu `Root` `"App"` dengan `Extra: extra(appPid, procs)`. Panggil `PSLister` sekali, lalu `Sampler.Sample`. Error `ps` dikembalikan sebagai `ipc.NewError(ipc.ErrInternal, ...)`; params tidak valid sebagai `ipc.ErrInvalidParams`. Gunakan pola `internal/status/ipc.go`.
- [ ] **Step 4: Tulis `wire_resource.go`** dengan `RegisterModule("resource", ...)`: buat `Sampler`, ambil `session.manager` lewat `d.OnReady` (pola `wire_status.go`), daftarkan IPC. `extra` sementara `nil` (diisi di Task 3).
- [ ] **Step 5: Verifikasi.** `go build ./... && go vet ./...` lulus. Jalankan daemon, lalu `shikigamictl` atau klien IPC yang ada memanggil `resource.snapshot`; hasil berisi baris `Daemon` dengan RSS > 0. (`cmd/shikigamictl` tidak punya perintah generik; bila tak ada, tambahkan `shikigamictl resource` yang mencetak JSON, mengikuti `cmd_status.go`.)
- [ ] **Step 6: Commit.** `git commit -m "feat(resource): IPC resource.snapshot dan PID sesi"`

### Task 3: Atribusi app dan proses WebKit

**Files:**
- Modify: `app/bridge.go`, `cmd/shikigamid/wire_resource.go`
- Create: `internal/resource/responsible_darwin.go`, `internal/resource/responsible_other.go`

**Interfaces:**
- Consumes: `RegisterIPC(..., extra)` (Task 2), `RawProc.Command` (Task 1).
- Produces:
  - `func (b *Bridge) AppPID() int` (dipanggil frontend sebagai `window.go.main.Bridge.AppPID()`).
  - `func WebKitPIDs(appPid int, procs []RawProc) []int` di paket `resource`: PID proses `com.apple.WebKit.*` yang bertanggung jawab ke `appPid`; di non-darwin mengembalikan `nil`.

Proses WebKit (GPU, WebContent, Networking) adalah layanan XPC, bukan anak proses app, sehingga pohon proses tidak cukup.

- [ ] **Step 1: Spike (buang setelah dipakai).** Program Go kecil bercgo di direktori scratchpad yang memanggil `responsibility_get_pid_responsible_for_pid(pid_t)` (simbol privat libSystem, deklarasi `extern`) untuk tiap PID yang `Command`-nya memuat `com.apple.WebKit`, saat `shikigami` berjalan. Verifikasi: tiga PID WebKit milik app melaporkan PID app; WebKit milik Safari atau app lain tidak.
- [ ] **Step 2: Putuskan.** Bila simbol tersedia dan hasilnya benar, lanjut Step 3. Bila tidak, **fallback**: `WebKitPIDs` mengembalikan `nil` di semua platform, dan dropdown (Task 4) menampilkan catatan "WebKit tidak termasuk". Laporkan hasil spike ke user sebelum lanjut.
- [ ] **Step 3: Implementasi `responsible_darwin.go`** (build tag `darwin`, cgo) dan `responsible_other.go` (`//go:build !darwin`).
- [ ] **Step 4: Tambah `Bridge.AppPID`** di `app/bridge.go` (`return os.Getpid()`), lalu isi argumen `extra` di `wire_resource.go` dengan `resource.WebKitPIDs`.
- [ ] **Step 5: Verifikasi.** `make build`. Jalankan app; `resource.snapshot` dengan `appPid` dari `Bridge.AppPID()` memberi baris `App` dengan RSS yang kira-kira sebanding dengan laporan `docs/perf/multipane-report.md` (ratusan MB, bukan belasan MB).
- [ ] **Step 6: Commit.** `git commit -m "feat(resource): hitung proses app dan WebKit"`

### Task 4: Store dan dropdown di bilah atas

**Files:**
- Create: `frontend/src/modules/resource/format.ts`, `frontend/src/modules/resource/store.ts`, `frontend/src/modules/resource/ResourceMeter.svelte`, `frontend/src/modules/resource/index.ts`
- Modify: `frontend/src/lib/ipc.ts` (helper `appPid`), `frontend/src/main.ts` atau berkas tempat modul lain diimpor (ikuti cara `modules/notifications` dimuat)

**Interfaces:**
- Consumes: `call`, `inWails` dari `lib/ipc.ts`; `registerTopbarItem` dari `modules/shell`; store `sessions` dari `lib/stores`.
- Produces:
  - `formatBytes(n: number): string` — `< 1 GiB` → `"330 MB"` (bulat); `≥ 1 GiB` → `"1,4 GB"`.
  - `formatCpu(p: number): string` — `< 0.1` → `"< 0,1%"`; `< 10` → `"8,0%"`; selain itu `"12%"`.
  - `resourceStore: Readable<{ snapshot: Snapshot | null; error: string }>`
  - `startResourceSync(): () => void` — idempoten; polling 5000 ms hanya saat `document.visibilityState === 'visible'`; saat kembali terlihat langsung ambil sampel lalu lanjut berinterval; saat error mempertahankan `snapshot` terakhir dan mengisi `error`.
  - `appPid(): Promise<number>` di `ipc.ts`, memanggil `window.go.main.Bridge.AppPID()`.

- [ ] **Step 1: Tulis `format.ts`** sesuai signature di atas.
- [ ] **Step 2: Tulis `store.ts`.** Params panggilan: `{ appPid }` (dari `appPid()`, di-cache). Jika `!inWails`, `startResourceSync` tidak melakukan apa-apa.
- [ ] **Step 3: Tulis `ResourceMeter.svelte`.** Tombol bilah atas berisi `CPU {formatCpu} · {formatBytes}` (total); klik membuka dropdown dengan pola buka/tutup, klik-di-luar, dan Escape dari `SettingsButton.svelte`. Isi dropdown: satu baris per `Proc` (nama, memori, CPU); untuk `kind === 'session'` nama dicari dari store `sessions` berdasarkan `sessionId` dan jatuh ke `label` bila tak ada. Footer kecil: "Diperbarui tiap 5 detik · RSS termasuk memori shared". Bila `snapshot` null tampilkan "–". Item disembunyikan bila `!inWails`.
- [ ] **Step 4: Tulis `index.ts`**: `registerTopbarItem({ id: 'resource', component: ResourceMeter, align: 'right', order: 20 })` dan `startResourceSync()`; impor modul di tempat modul lain diimpor.
- [ ] **Step 5: Verifikasi tipe dan build.** `cd frontend && npm run check && npm run build` lulus tanpa error baru.
- [ ] **Step 6: Commit.** `git commit -m "feat(ui): meter CPU dan memori di bilah atas"`

### Task 5: Verifikasi di app asli dan dokumentasi

**Files:**
- Modify: `README.md` (daftar Fitur), `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md` (baris 85 dan 190: meter resource kini masuk dalam bentuk terbatas)

- [ ] **Step 1: Build dan jalankan** `make build`, buka app dengan satu sesi `shell` dan satu sesi `claude`.
- [ ] **Step 2: Cek manual:** (a) ringkasan muncul dan angka berubah saat output deras; (b) dropdown menampilkan Daemon, App, dan dua sesi, menutup dengan klik luar dan Escape; (c) tutup satu sesi, barisnya hilang dalam ≤ 5 detik tanpa error di konsol; (d) minimize atau sembunyikan jendela, lalu pantau `shikigamid` dengan `ps`/log: tidak ada panggilan `resource.snapshot` selama tersembunyi, dan satu panggilan segera setelah ditampilkan lagi; (e) bandingkan RSS "App" dengan `ps` untuk PID yang sama, selisih kecil.
- [ ] **Step 3: Ukur overhead.** CPU idle daemon dan app dengan meter aktif tetap < 1% (metode `docs/perf/spike-report.md`); catat angkanya di pesan commit.
- [ ] **Step 4: Perbarui README dan spec v2** (satu baris tiap bagian).
- [ ] **Step 5: Commit.** `git commit -m "docs: catat fitur monitor resource"`
