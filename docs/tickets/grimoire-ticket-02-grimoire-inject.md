---
id: grimoire-ticket-02-grimoire-inject
title: Grimoire: suntik index + laws ke agent
spec: docs/superpowers/specs/2026-11-22-grimoire-design.md
status: implemented
wave: 2
depends_on: [grimoire-ticket-01-grimoire-build]
---
# Grimoire: suntik index + laws ke agent

## Tujuan
Grimoire yang sudah dibangun tiket 01 baru berguna bila agent benar-benar menerimanya. Epic ini menyusun satu teks suntikan (header aturan konstan + `index.md` + `laws.md`), membekukannya per sesi sebagai snapshot, lalu memasangnya ke sesi baru Claude dan Codex lewat `provider.RegisterLaunchContribution` dengan teks yang byte-identik. Epic ini juga memberi pengguna cara menulis `laws.md` dari panel dan mengukur berapa besar index yang disuntikkan, supaya klaim "hemat token" bisa diverifikasi dan bukan diasumsikan.

## Hasil yang bisa dijalankan
Di atas tiket 01 (generate kerangka statik + panel sidebar):
- Membuat sesi baru `claude` pada project yang sudah punya grimoire: proses diluncurkan dengan tambahan `--append-system-prompt-file <DataDir>/run/grimoire-<hex>.md`. Agent bisa ditanya "apa isi index grimoire yang kamu terima?" dan menjawab dari teks itu.
- Membuat sesi baru `codex` pada project yang sama: diluncurkan dengan tambahan `-c developer_instructions=<string TOML>` yang isinya identik dengan snapshot Claude.
- Sesi `shell` tidak berubah. Project tanpa grimoire (belum pernah Generate) tidak mendapat suntikan, sesi berjalan seperti sebelumnya.
- Di panel Grimoire ada entri "Laws". Klik membuka `LawsEditor` (textarea); menyimpan menulis `laws.md` di data dir. Sesi yang dibuat setelahnya memuat isi laws baru; sesi yang sudah hidup tidak berubah.
- Panel menampilkan baris "Disuntik ke sesi baru: N byte, ~M token". Angka yang sama muncul di `shikigamictl grimoire status` (indexBytes, estTokens, pages, lawsBytes).
- Snapshot lama di `<DataDir>/run/` yang tidak dipakai sesi hidup dibersihkan.
- `go build ./...` dan `make build` lulus; aplikasi jalan.

## Batasan
In scope:
- Komposisi teks suntikan, snapshot per sesi, escape string TOML, pembersihan snapshot lama.
- `LaunchContribution` untuk provider `claude` dan `codex`.
- Method IPC `grimoire.laws.get` / `grimoire.laws.set`, estimasi token, perluasan payload `grimoire.status`, tampilan di `shikigamictl grimoire status`.
- UI: `LawsEditor`, entri "Laws", baris info ukuran suntikan, slot aditif kecil di `GrimoirePanel.svelte`.

Out of scope:
- Mengubah paket `internal/session` atau menggandakan flag izin agent (`--dangerously-skip-permissions`, `--dangerously-bypass-approvals-and-sandbox` sudah ditambahkan provider).
- Menulis ke repo proyek (`AGENTS.md`, `CLAUDE.md`); memakai `model_instructions_file` Codex (itu mengganti instruksi dasar).
- Mengubah teks di tengah sesi; memicu build (itu tiket 03), ringkasan LLM (tiket 04), pelipatan index (milik tiket 01 saat membangun `index.md`).
- Laws otomatis, MCP grimoire, integrasi grimoire eksternal (`GrimoireURL/Key`), Windows.
- Unit test (guard hanya build); dependency baru.

## Kontrak
Dipakai dari tiket 01 (jangan diubah artinya):
- `Store` dan layout `<DataDir>/grimoire/<projectId>/` (`index.md`, `laws.md`, `state.json`, `books/...`) dari `internal/grimoire/store.go`. Sebelum menulis kode, baca API `Store` yang ada (pembacaan index, path direktori project, deteksi "grimoire ada") dan pakai ulang.
- Method `grimoire.status` dan event `grimoire.changed {projectId}`; konstanta header aturan konstan (bila tiket 01 sudah mendefinisikannya untuk `index.md`, pakai yang sama dan jangan menduplikasi teksnya).
- `provider.RegisterLaunchContribution(fn func(*provider.LaunchContext))`; `LaunchContext{Session session.SessionSpec, Provider, Env, Args, MCPServers}`; `SessionSpec{WorkspaceID, ProjectID, Cwd, ProviderID, ...}`; `ctx.Provider.ID()` bernilai `provider.IDClaude` / codex / shell.
- `paths.DataDir()`, `internal/fsx`.

Disediakan epic ini (paket `internal/grimoire`):
```go
// inject.go
const InjectionHeader = "..."               // konstan; hanya bila tiket 01 belum punya header untuk disuntik
func ComposeInjection(projectID string) (text string, ok bool, err error)
// ok=false bila project tidak punya grimoire (tanpa suntikan).
// text = header aturan + "\n" + index.md + "\n" + laws.md (laws apa adanya; tanpa laws = bagian dilewati).
// Deterministik: input berkas sama menghasilkan byte sama.

// snapshot.go
func WriteSnapshot(text string) (path string, err error)       // <DataDir>/run/grimoire-<16 hex>.md, mode 0600
func PruneSnapshots(inUse func(path string) bool) error        // hapus grimoire-*.md yang tidak dipakai sesi hidup
func TOMLString(s string) string                               // string TOML basic ter-quote, escape \\ \" \n \r \t dan kontrol lain (\u00XX)

// laws.go
const MaxLawsBytes = 16 * 1024
func (s *Store) ReadLaws(projectID string) (string, error)
func (s *Store) WriteLaws(projectID, content string) error     // atomik via fsx; error bila > MaxLawsBytes

// measure.go
type Measure struct{ IndexBytes, EstTokens, Pages, LawsBytes int }
func MeasureInjection(projectID string) (Measure, error)       // EstTokens = ceil(len(teks suntikan penuh)/4)
func Register(srv *ipc.Server, d ...)                           // mendaftarkan grimoire.laws.get/set dan memperluas grimoire.status
```

IPC (error mengikuti `internal/ipc/errors.go`; `projectId` kosong/tidak dikenal = error masukan tidak valid):
- `grimoire.laws.get {projectId}` -> `{content: string}` (string kosong bila belum ada berkas).
- `grimoire.laws.set {projectId, content}` -> `{lawsBytes: int}`; menolak > `MaxLawsBytes`; setelah sukses menerbitkan event `grimoire.changed {projectId}`.
- `grimoire.status {projectId}` ditambah field aditif `indexBytes`, `estTokens`, `pages`, `lawsBytes` (field lama dan artinya tidak berubah). Bila tiket 01 sudah mengisi sebagian field ini, tiket ini hanya menyamakan sumbernya ke `MeasureInjection` tanpa mengubah arti. `indexBytes` = ukuran teks suntikan penuh (header + index + laws), karena itulah yang dibayar tiap sesi; `pages` = jumlah bab.

Argumen launch:
- Claude: `ctx.Args = append(ctx.Args, "--append-system-prompt-file", snapshotPath)`.
- Codex: `ctx.Args = append(ctx.Args, "-c", "developer_instructions="+TOMLString(text))`.

Frontend (`frontend/src/modules/grimoire/laws/`):
- `LawsEditor.svelte` (props `projectId`), `lawsStore.ts` (get/set via klien IPC yang sudah dipakai modul grimoire), `InjectionInfo.svelte` (props `projectId`, membaca `indexBytes`, `estTokens` dari store status modul grimoire).
- Konsumsi event `grimoire.changed` untuk memuat ulang; tanpa `setInterval`/polling.

## Fitur

### compose-snapshot: Susun teks suntikan dan snapshot per sesi
Deskripsi: `internal/grimoire/inject.go` dan `snapshot.go`.
- `ComposeInjection(projectID)`: bila `index.md` tidak ada, kembalikan `ok=false` (project tanpa grimoire = tanpa suntikan). Selain itu gabungkan header aturan konstan, isi `index.md`, dan isi `laws.md` apa adanya (tidak di-trim, tidak ditambah tanggal atau waktu apa pun) dengan pemisah tetap. Header berisi aturan pakai: baca index, buka halaman yang relevan sebelum membaca source, path sumber tertera di tiap bab, lokasi folder grimoire project (path absolut `<DataDir>/grimoire/<projectId>/books`), dan peringatan bahwa baris bertanda `warning: stale` mungkin sudah usang. Karena path itu stabil per project, teks tetap byte-identik antar sesi.
- `WriteSnapshot(text)`: pola `writeMCPConfig` (`internal/provider/claude.go`): `os.MkdirAll(<DataDir>/run, 0o700)`, nama `grimoire-<16 hex acak>.md`, `WriteFile` 0600 lalu `Chmod` 0600.
- `PruneSnapshots(inUse)`: hapus berkas `grimoire-*.md` di `run/` yang `inUse` mengatakan tidak dipakai. Sumber "sesi hidup": bila `session.Manager` tidak membuka cara memeriksa argumen sesi, pakai kebijakan aman: hapus hanya snapshot yang lebih tua dari 24 jam dan bukan salah satu dari snapshot yang dibuat proses daemon ini (daftar in-memory yang dicatat `WriteSnapshot`). Pruning dijalankan sekali saat modul siap dan setiap kali snapshot baru ditulis; kegagalan pruning hanya dicatat log, tidak menggagalkan peluncuran.
- `TOMLString(s)`: menghasilkan string TOML basic yang valid, termasuk karakter kontrol dan Unicode; hasilnya dipakai apa adanya sebagai nilai `-c`.
Acceptance criteria:
- [ ] `ComposeInjection` dua kali berturut-turut tanpa perubahan berkas menghasilkan byte yang sama.
- [ ] Project tanpa `index.md` menghasilkan `ok=false` tanpa error.
- [ ] Snapshot ada di `<DataDir>/run/grimoire-<hex>.md` dengan mode 0600 dan isi sama persis dengan teks yang dikembalikan.
- [ ] `TOMLString` aman untuk newline, tanda kutip, backslash, dan karakter kontrol (nilai dapat di-parse sebagai string TOML).
- [ ] Snapshot yang tidak dipakai dibersihkan tanpa pernah menghapus snapshot milik sesi yang masih hidup.
- [ ] `go build ./...` lulus.
ownedPaths: `internal/grimoire/inject.go`, `internal/grimoire/snapshot.go`
Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini (laws-measure-ipc hanya memanggil `ComposeInjection`; sepakati tanda tangan di atas sehingga kedua sisi bisa ditulis bersamaan). `launch-contribution` memakai fungsi ini, jadi penyambungan akhirnya menunggu fitur ini selesai.

### launch-contribution: Pasang LaunchContribution untuk claude dan codex
Deskripsi: `cmd/shikigamid/wire_grimoire_inject.go`. Modul `RegisterModule("grimoire-inject", ...)` (nama unik; modul dipasang alfabetis, jadi layanan lain diambil di dalam `d.OnReady` lewat `d.Lookup`). Di `d.OnReady` daftarkan satu `provider.RegisterLaunchContribution` (sekali saja, `sync.Once`, mengikuti pola `internal/mention/install.go`). Logika contribution:
1. `ctx == nil || ctx.Provider == nil` -> keluar. Provider `shell` (dan id selain claude/codex) -> keluar tanpa perubahan.
2. Tentukan `projectId`: pakai `ctx.Session.ProjectID`; bila kosong, cocokkan `ctx.Session.Cwd` dengan `Project.Path` lewat layanan workspace (`workspace.list` atau layanan workspace dari `d.Lookup`, dengan cache pendek seperti `workspaceDirectory` di `wire_mention.go`). Bila tidak ketemu -> keluar tanpa suntikan.
3. `ComposeInjection(projectId)`; `ok=false` atau error -> keluar tanpa suntikan (catat log pada error; peluncuran tidak boleh gagal karena grimoire).
4. Claude: `WriteSnapshot(text)` lalu tambahkan `--append-system-prompt-file <path>` ke `ctx.Args`. Codex: tambahkan `-c`, `developer_instructions=`+`TOMLString(text)`; untuk Codex snapshot berkas tetap ditulis agar satu sumber kebenaran (opsional, dipakai untuk audit), tetapi argumen memakai teks yang sama.
5. Tidak menyentuh `internal/session`, tidak menambah flag izin, tidak menghapus atau menggandakan `Args` yang sudah ada (selalu `append`; jangan menambah bila flag yang sama sudah ada).
Acceptance criteria:
- [ ] Sesi claude pada project ber-grimoire: argumen proses memuat `--append-system-prompt-file` ke berkas snapshot yang ada.
- [ ] Sesi codex: argumen memuat `-c developer_instructions=...` dengan isi yang, setelah di-unescape, byte-identik dengan snapshot Claude untuk project yang sama.
- [ ] Sesi shell dan sesi pada project tanpa grimoire tidak mendapat argumen tambahan.
- [ ] Kegagalan membaca grimoire atau menulis snapshot tidak menggagalkan pembuatan sesi.
- [ ] Flag izin agent tidak muncul dua kali.
- [ ] `go build ./...` lulus; paket `internal/session` tidak berubah.
ownedPaths: `cmd/shikigamid/wire_grimoire_inject.go`
Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini; compile akhir memerlukan `compose-snapshot`.

### laws-measure-ipc: Laws dan pengukuran ukuran index
Deskripsi: `internal/grimoire/laws.go` dan `measure.go`.
- `ReadLaws`/`WriteLaws` pada `Store`: file `laws.md` di `<DataDir>/grimoire/<projectId>/`, tulis atomik (temp + rename lewat `internal/fsx` bila tersedia), buat direktori project bila belum ada, tolak konten lebih dari `MaxLawsBytes` dengan error masukan tidak valid. Konten disimpan apa adanya.
- `MeasureInjection`: memanggil `ComposeInjection`; `IndexBytes = len(text)`, `EstTokens = ceil(len/4)` (estimasi kasar, diberi label "~" di UI), `Pages` = jumlah bab dari daftar buku, `LawsBytes` = ukuran `laws.md`. Project tanpa grimoire: semua nol.
- `Register(...)` mendaftarkan `grimoire.laws.get`, `grimoire.laws.set`, dan memperluas payload `grimoire.status` dengan `indexBytes`, `estTokens`, `pages`, `lawsBytes` secara aditif (bungkus/komposisikan handler status dari tiket 01 tanpa mengubah field lamanya; bila API tiket 01 tidak bisa dibungkus, tambahkan hook kecil yang dijelaskan di Wiring). `laws.set` menerbitkan `grimoire.changed`.
- `shikigamictl grimoire status` menampilkan baris tambahan "index: N byte, ~M token, P halaman, laws: L byte". Karena `cmd/shikigamictl/cmd_grimoire.go` milik tiket 01, tampilan tambahan ini dilakukan dengan membaca field baru dari output status yang sudah dicetak tiket 01 (bila tiket 01 mencetak JSON mentah, field baru otomatis ikut); bila perlu perubahan berkas itu, batasi pada satu blok cetak kecil dan sebutkan di Wiring.
Acceptance criteria:
- [ ] `grimoire.laws.set` lalu `grimoire.laws.get` mengembalikan konten yang sama; berkas `laws.md` ada di data dir dan tidak ada berkas ditulis di repo proyek.
- [ ] Konten melebihi `MaxLawsBytes` ditolak dengan error IPC yang konsisten.
- [ ] `grimoire.status` memuat `indexBytes`, `estTokens`, `pages`, `lawsBytes` dan field lama tidak berubah.
- [ ] Mengubah laws mengubah `indexBytes`/`estTokens` pada status berikutnya.
- [ ] `shikigamictl grimoire status` menampilkan ukuran index dan estimasi token.
- [ ] `go build ./...` lulus.
ownedPaths: `internal/grimoire/laws.go`, `internal/grimoire/measure.go`
Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini; bergantung pada tanda tangan `ComposeInjection` di atas.

### laws-ui: Entri Laws dan info suntikan di panel
Deskripsi: `frontend/src/modules/grimoire/laws/`.
- `LawsEditor.svelte`: textarea sederhana (font monospace, token tema yang ada), tombol Simpan dan indikator "tersimpan"/error, penghitung byte. Memuat isi lewat `grimoire.laws.get`, menyimpan lewat `grimoire.laws.set`, menampilkan pesan bahwa perubahan berlaku untuk sesi baru. Dibuka dari entri "Laws" di panel: tampil sebagai tampilan inline di dalam panel (bukan jenis pane baru) dengan tombol kembali ke daftar buku.
- `InjectionInfo.svelte`: baris "Disuntik ke sesi baru: N byte, ~M token" dari `grimoire.status` (`indexBytes`, `estTokens`); disembunyikan bila grimoire belum ada.
- `lawsStore.ts`: state per project aktif; memuat ulang saat event `grimoire.changed` untuk project itu; tanpa `setInterval`.
- Slot aditif di `GrimoirePanel.svelte` (milik tiket 01, hanya perluasan kecil seperti dijelaskan di Wiring): satu entri "Laws" di atas daftar buku dan satu baris `InjectionInfo` di bawahnya; daftar buku tidak dirombak.
Acceptance criteria:
- [ ] Entri "Laws" tampil di panel pada project ber-grimoire dan membuka `LawsEditor`.
- [ ] Menyimpan laws berhasil dan penyimpanan ulang muncul pada pembukaan berikutnya.
- [ ] Baris info menampilkan byte dan token yang sama dengan `shikigamictl grimoire status` dan berubah setelah laws disimpan, tanpa reload manual.
- [ ] Tidak ada polling; refresh hanya via `grimoire.changed`.
- [ ] Tidak ada dependency baru; build frontend lulus (`make build`).
ownedPaths: `frontend/src/modules/grimoire/laws/`
Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini; kontraknya hanya method IPC `grimoire.laws.get/set` dan field status di atas (boleh di-stub saat pengembangan).

## Path yang dimiliki
Hanya ini yang boleh disentuh oleh tiket ini:
- `internal/grimoire/inject.go`
- `internal/grimoire/laws.go`
- `internal/grimoire/snapshot.go`
- `internal/grimoire/measure.go`
- `cmd/shikigamid/wire_grimoire_inject.go`
- `frontend/src/modules/grimoire/laws/`

Pengecualian sempit milik tiket 01 yang boleh disentuh seminimal mungkin dan harus disebut di Wiring: satu baris panggilan `Register`/hook di `cmd/shikigamid/wire_grimoire.go` (hanya bila terpaksa), satu blok cetak di `cmd/shikigamictl/cmd_grimoire.go`, dan slot aditif di `frontend/src/modules/grimoire/GrimoirePanel.svelte`.

## Wiring
Langkah penutup tiket ini, memakai titik ekstensi tiket 01:
1. Daemon: `wire_grimoire_inject.go` mendaftarkan modul `grimoire-inject` lewat `RegisterModule`. Di `d.OnReady`: ambil Store/layanan grimoire dari tiket 01 lewat `d.Lookup`, panggil `grimoire.Register(srv, ...)` (method `grimoire.laws.get/set` dan perluasan `grimoire.status`), pasang `RegisterLaunchContribution` (satu kali), dan jalankan `PruneSnapshots` awal. Karena modul dipasang alfabetis (`grimoire` sebelum `grimoire-inject`), layanan tiket 01 sudah tersedia di `OnReady`. Bila `grimoire.status` milik tiket 01 tidak bisa diperluas dari luar, tambahkan satu baris hook di `wire_grimoire.go` dan catat di sini.
2. CLI: field baru status tampil di `shikigamictl grimoire status` (tanpa subcommand baru; tidak perlu `RegisterCommand` baru).
3. Frontend: impor `LawsEditor` dan `InjectionInfo` di `GrimoirePanel.svelte` (perluasan aditif: entri "Laws" + baris info). Tidak ada panel atau `registerPanel` baru.
4. Jalankan `make build`, luncurkan aplikasi, Generate grimoire pada sebuah project, buka sesi claude dan codex baru, dan periksa argumen proses (mis. `ps`) serta isi snapshot di `<DataDir>/run/`. Tiket ini tidak bergantung pada tiket 03/04 untuk berjalan.

## Guard
- `go build ./...` berhasil.
- `make build` berhasil (termasuk build frontend).
- Tanpa unit test (keputusan spec); verifikasi manual lewat Wiring langkah 4.
