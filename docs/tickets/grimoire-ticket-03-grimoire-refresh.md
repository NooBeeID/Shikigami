---
id: grimoire-ticket-03-grimoire-refresh
title: Grimoire: deteksi perubahan, stale, dan build latar belakang
spec: docs/superpowers/specs/2026-11-22-grimoire-design.md
status: implemented
wave: 3
depends_on: [grimoire-ticket-02-grimoire-inject]
---
# Grimoire: deteksi perubahan, stale, dan build latar belakang

## Tujuan
Build grimoire dari tiket 1 selalu menulis ulang semua modul. Epic ini membuatnya inkremental: hash isi working tree per modul dibandingkan dengan `state.json`, hanya modul baru/berubah yang ditulis ulang, modul yang hilang dihapus halamannya. Sesi baru memicu build di latar belakang tanpa pernah menunda peluncuran agent (sesi memakai snapshot terakhir yang dibekukan tiket 2), sehingga grimoire tidak tertinggal dari kode tanpa biaya waktu di jalur peluncuran. Panel Grimoire menandai bab/buku stale dan menampilkan progres build.

## Hasil yang bisa dijalankan
Di atas tiket 1 (build statik + panel) dan tiket 2 (suntikan index + laws):
- Mengedit file kode di sebuah modul (tanpa commit), lalu membuat sesi baru: peluncuran agent langsung berjalan seperti biasa, sesi itu memakai snapshot index lama. Beberapa saat kemudian panel Grimoire berubah sendiri (lewat event `grimoire.changed`): hanya bab modul yang diedit yang ditulis ulang, modul yang folder-nya dihapus hilang dari daftar, modul baru muncul. Sesi berikutnya menyuntikkan index terbaru.
- Selama build berjalan, panel menampilkan indikator "sedang build". Bab/buku yang stale (mis. build modul itu gagal) bertanda stale; tombol "Perbarui" menjalankan build inkremental, dan `--full` lewat CLI memaksa build penuh.
- `shikigamictl grimoire build [--project <id>] [--full]` memakai jalur inkremental; `shikigamictl grimoire status` menampilkan `staleCount` dan `building`.
- `go build ./...` dan `make build` lulus, aplikasi berjalan.

## Batasan
In scope:
- Diff hash modul vs `state.json`, build inkremental di atas `Build` penuh dari tiket 1.
- Hook per-modul `Summarizer` opsional (default statik) sebagai titik masuk tiket 4.
- Pemicu build latar belakang saat sesi baru lewat `provider.RegisterLaunchContribution`, single-flight per project, tanpa polling/timer.
- Flag `--full` pada `grimoire build` dan field `full` pada IPC `grimoire.build`.
- Field aditif `stale` per bab di `grimoire.list`, `staleCount` dan `building` di `grimoire.status`.
- Penanda stale, indikator build, tombol Perbarui di panel.

Out of scope:
- Ringkasan LLM (tiket 4); epic ini hanya menyediakan hook `Summarizer`, implementasi default statik.
- Mengubah paket `session`, mengubah inti `Build`/`Scan`/`Store` tiket 1 selain yang disebut di Wiring, mengubah `ComposeInjection`/snapshot tiket 2.
- Polling, timer, `setInterval`, file watcher. Pemicu hanya sesi baru dan perintah eksplisit.
- Memperbarui snapshot sesi yang sedang berjalan (merusak prompt cache; anti-tujuan spec).
- Unit test, dependency baru, Windows, integrasi grimoire eksternal, MCP grimoire.

## Kontrak

Dipakai dari tiket sebelumnya (nama persis mengikuti kode tiket 1/2; bila nama aktual berbeda sedikit, sesuaikan tanpa mengubah arti):
- `grimoire.Build(ctx, ...)`, `Store` (baca/tulis `index.md`, `state.json`, `books/...`), `State{ProjectID, BuiltAt, Modules map[string]ModuleState}` dengan `ModuleState{Hash string, Sources []string, Stale bool}`, fungsi pindai/kelompok modul yang menghasilkan daftar modul + hash (`Scan`), pembangun index dari daftar bab (`BuildIndex`/setara) dan penulis bab statik per modul. Tiket 1 sudah menyimpan `Hash` per modul di `state.json`.
- `ComposeInjection` dan snapshot beku per sesi (tiket 2); epic ini tidak mengubahnya, hanya memastikan snapshot dibekukan sebelum build latar belakang dimulai (urutan dijelaskan di fitur `session-trigger`).
- `provider.RegisterLaunchContribution` (kontribusi tiket 2 sudah memakainya untuk suntikan; epic ini mendaftarkan kontribusi kedua yang tidak menambah argumen apa pun).
- Event `grimoire.changed {projectId}` dan `GrimoirePanel.svelte` (tiket 1).
- Method IPC tiket 1: `grimoire.build`, `grimoire.status`, `grimoire.list`.

Disediakan epic ini (Go, paket `internal/grimoire`):
```go
// diff.go
type ModuleChange struct {
    Name string
    Kind ChangeKind // ChangeAdded | ChangeModified | ChangeRemoved | ChangeUnchanged
    OldHash, NewHash string
}
type Diff struct {
    Added, Modified, Removed, Unchanged []ModuleChange
    // Pending = Added + Modified + modul dengan State.Modules[x].Stale == true
}
func ComputeDiff(scanned []Module, st *State) Diff  // murni, tanpa I/O
func (d Diff) Pending() []ModuleChange
func (d Diff) Empty() bool                          // tidak ada Added/Modified/Removed dan tidak ada stale

// incremental.go
type Summarizer interface {                         // hook untuk tiket 4; nil = statik
    Summarize(ctx context.Context, m Module, static string) (chapters map[string]string, err error)
}
type IncrementalOptions struct {
    Full       bool        // true = abaikan state, proses semua modul (jalur Build penuh)
    Summarizer Summarizer  // opsional
}
type Result struct {
    Rebuilt, Removed, Failed, Skipped int
    Duration time.Duration
}
func (b *Builder) BuildIncremental(ctx context.Context, projectID string, opts IncrementalOptions) (Result, error)

// trigger.go
type Trigger struct { /* single-flight per projectId */ }
func NewTrigger(b *Builder, publish func(event string, payload any)) *Trigger
func (t *Trigger) Kick(projectID, projectPath string)   // non-blocking, aman dipanggil dari jalur launch
func (t *Trigger) Building(projectID string) bool
func RegisterRefresh(d RefreshDeps)                     // dipanggil dari wire_grimoire_refresh.go
```

Perilaku kontrak:
- Modul dianggap berubah bila `hash(scan)` != `State.Modules[name].Hash`. Hash adalah isi working tree (bukan git HEAD), sehingga edit belum di-commit terdeteksi; folder tanpa git didukung.
- Modul Added/Modified: bab-bab modul ditulis ulang utuh (tidak di-patch), `Hash` dan `Sources` diperbarui, `Stale=false`. Modul Removed: direktori `books/<nama_snake>/` dihapus dan entrinya dibuang dari `State.Modules`. Modul Unchanged tidak disentuh (kecuali `Stale=true`, maka dicoba ulang).
- Modul yang gagal diproses: halaman lama dipertahankan, `Stale=true` di state, baris index buku itu memakai akhiran ` warning: stale, jalankan grimoire build`. Kegagalan satu modul tidak membatalkan modul lain.
- Setelah semua modul: `index.md` dibangun ulang, `state.json` disimpan (tulis atomik lewat `internal/fsx` seperti tiket 1), `builtAt` diperbarui.
- `Full=true` sama dengan semua modul diperlakukan Modified.
- Pembaruan aditif IPC (tidak mengubah arti field lama):
  - `grimoire.build {projectId, full?}` -> mengembalikan hasil build (`rebuilt`, `removed`, `failed`) dan menjalankan jalur inkremental; bila build untuk project itu sudah berjalan, mengembalikan `building: true` tanpa memulai yang kedua (single-flight).
  - `grimoire.list` -> `chapters[].stale: bool` (true bila modul induk stale).
  - `grimoire.status` -> `staleCount` (jumlah modul stale) dan `building` (build berjalan untuk project itu).
  - Event `grimoire.changed {projectId}` dipublish saat build selesai (sukses atau sebagian gagal) dan saat build mulai (agar panel menampilkan indikator `building`).

## Fitur

### diff-incremental: Diff hash dan build inkremental
Deskripsi:
Buat `internal/grimoire/diff.go` dan `internal/grimoire/incremental.go`.
- `ComputeDiff` membandingkan modul hasil `Scan` tiket 1 (nama + hash) dengan `State.Modules`. Hasilnya mengelompokkan modul menjadi Added, Modified, Removed, Unchanged; modul dengan `Stale=true` di state selalu masuk `Pending()`.
- `BuildIncremental` mengambil lock per project (shared dengan `Build` penuh bila tiket 1 sudah punya; bila belum, mutex per projectId di `Builder`), memuat state (tanpa state = semua Added, setara build penuh), menghitung diff, lalu untuk tiap modul Pending menulis bab lewat generator statik tiket 1. Jika `opts.Summarizer != nil`, panggil `Summarize(ctx, module, staticSkeleton)` dan pakai hasilnya bila sukses; bila error, jatuh ke hasil statik dan tandai modul `Stale=true` (karena ringkasan belum ada). Tanpa Summarizer (default saat tiket ini selesai) semua modul statik dan tidak ada yang stale kecuali gagal tulis.
- Modul Removed: hapus direktori bukunya, buang dari state.
- Setelah itu bangun ulang index (termasuk aturan batas ukuran dan pelipatan buku jarang berubah dari tiket 1) dan simpan state. Bila `Diff.Empty()` dan `Full=false`, jangan menulis ulang apa pun (tidak menyentuh mtime, tidak publish event) dan kembalikan `Result` kosong.
- `Full=true` melewati diff dan memproses semua modul.
- `Build` penuh dari tiket 1 tidak diubah; `BuildIncremental(Full:true)` boleh mendelegasikan ke jalur yang sama dengan memakai fungsi tulis-bab publik tiket 1.
- Hormati `context`: bila dibatalkan, hentikan di antara modul, simpan state parsial yang konsisten (modul yang belum selesai tetap hash lama), dan jangan menulis index setengah jadi.

Acceptance criteria:
- [ ] `ComputeDiff` murni (tanpa I/O) dan menghasilkan Added/Modified/Removed/Unchanged yang benar untuk state kosong, state sama, satu modul berubah, satu modul hilang.
- [ ] Build kedua tanpa perubahan kode tidak menulis ulang bab apa pun dan tidak mengubah `index.md`.
- [ ] Mengedit satu file di satu modul (tanpa commit) menulis ulang hanya bab modul itu; hash di `state.json` ikut berubah.
- [ ] Menghapus folder modul menghapus `books/<nama>/` dan entri state serta baris index.
- [ ] Modul yang gagal diproses mempertahankan halaman lama, `stale: true` di state, dan baris index berakhiran ` warning: stale, jalankan grimoire build`; modul lain tetap diproses.
- [ ] Interface `Summarizer` ada, nil = statik; kegagalan Summarizer jatuh ke statik + stale.
- [ ] Penulisan state dan bab atomik; tidak ada file ditulis di dalam repo proyek.
- [ ] `go build ./...` lulus.

ownedPaths:
- `internal/grimoire/diff.go`
- `internal/grimoire/incremental.go`

Paralel: bisa dikerjakan paralel dengan `stale-ui`. `session-trigger` bergantung pada signature `BuildIncremental` di atas; kerjakan setelah (atau bersamaan dengan) `diff-incremental` dengan memegang signature kontrak.

### session-trigger: Pemicu build saat sesi baru
Deskripsi:
Buat `internal/grimoire/trigger.go` dan `cmd/shikigamid/wire_grimoire_refresh.go`.
- `Trigger.Kick(projectID, projectPath)`: bila build untuk projectId sudah berjalan, kembali segera (single-flight, `map[projectId]struct{}` di bawah mutex). Jika tidak, tandai building, publish `grimoire.changed`, lalu jalankan goroutine latar belakang yang menghitung diff (`Scan` + `ComputeDiff`); bila `Diff.Empty()` selesai tanpa menulis dan tanpa event tambahan selain membersihkan status building (publish sekali agar panel menghilangkan indikator); bila ada perubahan atau stale, jalankan `BuildIncremental`, lalu publish `grimoire.changed`. Goroutine memakai `context` turunan dari context daemon (dibatalkan saat daemon berhenti), bukan context peluncuran sesi.
- Kontribusi launch: dalam `wire_grimoire_refresh.go` daftarkan modul daemon baru bernama `grimoire-refresh` lewat `RegisterModule` dan di dalamnya `provider.RegisterLaunchContribution`. Kontribusi ini hanya memanggil `Kick` dan mengembalikan tanpa menambah `LaunchContext.Args`. Provider `shell` dilewati. Project tanpa grimoire (belum pernah di-generate) tidak dipicu: pembuatan pertama tetap lewat tombol Generate / `grimoire build` (hindari kejutan menulis data di project yang user belum aktifkan).
- Urutan penting: modul dipasang alfabetis (`grimoire`, `grimoire-inject`, `grimoire-refresh`), jadi kontribusi inject tiket 2 sudah terdaftar dan berjalan lebih dulu atau setidaknya snapshot dibekukan dari `index.md` yang ada saat ini, tanpa menunggu `Kick`. Kontribusi refresh tidak boleh menahan, memblokir, atau gagalkan peluncuran: tidak ada `Wait`, tidak ada channel yang ditunggu, error dicatat ke log saja. Urutan eksekusi kontribusi tidak boleh diandalkan. Penulisan `index.md`, `state.json`, dan bab oleh build wajib atomik (tulis ke berkas sementara lalu rename, lewat `internal/fsx`), sehingga snapshot yang sedang dibekukan tiket 2 selalu membaca `index.md` yang utuh (versi lama atau baru, tidak pernah setengah tertulis). Teks yang dibekukan per sesi tetap satu dan byte-identik untuk Claude dan Codex karena dibaca sekali oleh `ComposeInjection`.
- Layanan lain (`Builder`, publisher event IPC, daftar project untuk memetakan projectId dari path launch) diambil lewat `d.OnReady`/`d.Lookup`, bukan saat `RegisterModule` dipanggil.
- Perluas handler IPC `grimoire.build` (tanpa mengubah berkasnya: lewat modul ini, daftarkan handler pengganti hanya bila registry IPC mengizinkan override; bila tidak, ubah satu baris pemanggilan di `wire_grimoire.go`, lihat Wiring) agar memanggil `BuildIncremental` dengan `full` dari payload, serta menambah `staleCount` dan `building` ke `grimoire.status` dan `stale` ke `grimoire.list`.
- CLI: `shikigamictl grimoire build [--project <id>] [--full]` mengirim `grimoire.build {projectId, full}`; tanpa `--full` inkremental. Perubahan flag ini dilakukan sekecil mungkin di `cmd/shikigamictl/cmd_grimoire.go` (satu flag + payload; sebut di Wiring karena berkas itu milik tiket 1).
- Tanpa polling, tanpa timer, tanpa file watcher.

Acceptance criteria:
- [ ] Membuat sesi baru (Claude/Codex) di project yang sudah punya grimoire tidak tertunda oleh build, meskipun build lama; argumen launch tidak berubah oleh kontribusi ini.
- [ ] Bila hash working tree berubah atau ada modul stale, build inkremental berjalan di goroutine latar belakang dan `grimoire.changed` dipublish saat mulai dan selesai.
- [ ] Dua sesi baru berurutan cepat di project yang sama hanya menghasilkan satu build (single-flight); project berbeda tidak saling menghalangi.
- [ ] Bila tidak ada perubahan dan tidak ada stale, tidak ada penulisan file dan tidak ada build.
- [ ] Sesi yang diluncurkan memakai snapshot index sebelum build; teks suntikan sesi itu tidak berubah selama sesi hidup.
- [ ] Provider `shell` dan project tanpa grimoire tidak memicu build.
- [ ] `grimoire.status` memuat `staleCount` dan `building`; `grimoire.list` memuat `chapters[].stale`; `grimoire.build` menerima `full`.
- [ ] `shikigamictl grimoire build --full` memaksa build penuh; tanpa flag inkremental.
- [ ] Tidak ada `time.Ticker`/`setInterval`/watcher; goroutine berhenti saat daemon berhenti.
- [ ] Paket `session` tidak diubah. `go build ./...` lulus.

ownedPaths:
- `internal/grimoire/trigger.go`
- `cmd/shikigamid/wire_grimoire_refresh.go`

Paralel: bisa dikerjakan paralel dengan `stale-ui`. Bergantung pada signature `BuildIncremental` dari `diff-incremental` (kontrak di atas), jadi integrasi akhir dilakukan setelah keduanya ada; keduanya dapat ditulis bersamaan dengan memegang kontrak.

### stale-ui: Penanda stale dan progres di panel
Deskripsi:
Buat subfolder `frontend/src/modules/grimoire/stale/` berisi komponen Svelte kecil dan store turunan, tanpa menambah dependency:
- `StaleBadge.svelte`: penanda kecil (titik/label "stale") dengan token tema yang ada; dipakai di baris bab dan header buku (buku stale bila ada bab stale).
- `BuildIndicator.svelte`: indikator "sedang build" (teks + animasi halus CSS, tanpa timer JS) dari nilai `building` di status.
- `UpdateButton.svelte`: tombol "Perbarui" yang memanggil `grimoire.build {projectId}` (inkremental), nonaktif dan berlabel "Membangun..." saat `building`; menampilkan jumlah stale (`staleCount`) bila > 0.
- `staleStore.ts`: store per project aktif yang membaca `grimoire.status` dan `grimoire.list`; diperbarui hanya saat event `grimoire.changed` untuk projectId yang sesuai (dan saat project aktif berganti). Tanpa `setInterval`/polling.
- Slot aditif di `GrimoirePanel.svelte` (milik tiket 1, perubahan hanya aditif dan disebut di Wiring): header panel memuat `BuildIndicator` dan `UpdateButton`; baris bab/buku memuat `StaleBadge` bila `chapters[].stale`.
- Tombol Generate tiket 1 (belum ada grimoire) tidak diubah; Perbarui hanya muncul bila grimoire sudah ada.
- Teks UI bahasa Indonesia; identifier bahasa Inggris. Warna lewat token tema yang ada (tidak ada warna keras).

Acceptance criteria:
- [ ] Bab stale menampilkan penanda stale; buku yang memuat bab stale juga ditandai.
- [ ] Saat `building` true panel menampilkan indikator build dan tombol Perbarui nonaktif.
- [ ] Menekan Perbarui memanggil `grimoire.build` tanpa `full`, panel menyegarkan diri setelah event `grimoire.changed`.
- [ ] Store hanya refresh lewat event `grimoire.changed`; tidak ada `setInterval`/polling.
- [ ] Mengganti project aktif memuat status project itu, event project lain diabaikan.
- [ ] Gaya mengikuti token tema, terbaca di tema terang dan gelap.
- [ ] Build frontend lulus (`make build`).

ownedPaths:
- `frontend/src/modules/grimoire/stale/`

Paralel: bisa dikerjakan paralel dengan `diff-incremental` dan `session-trigger`. Bekerja terhadap kontrak IPC aditif (`stale`, `staleCount`, `building`); bila field belum dikirim backend, komponen memperlakukan sebagai `false`/`0` agar tetap jalan.

## Path yang dimiliki
Hanya path ini yang boleh disentuh oleh tiket ini:
- `internal/grimoire/diff.go`
- `internal/grimoire/incremental.go`
- `internal/grimoire/trigger.go`
- `cmd/shikigamid/wire_grimoire_refresh.go`
- `frontend/src/modules/grimoire/stale/`

Pengecualian sempit (titik ekstensi tiket 1, perubahan seminimal mungkin dan aditif, dilaporkan di PR): satu slot di `frontend/src/modules/grimoire/GrimoirePanel.svelte`, satu flag `--full` di `cmd/shikigamictl/cmd_grimoire.go`, dan bila registry IPC tidak mengizinkan override handler, satu baris panggilan di `cmd/shikigamid/wire_grimoire.go`. Tidak ada perubahan lain pada berkas tiket 1/2.

## Wiring
Langkah penutup yang dikerjakan setelah ketiga fitur selesai, supaya hasilnya terlihat dan jalan tanpa bergantung tiket 3/4 lain (hanya tiket 1 dan 2 yang sudah ada):
1. `cmd/shikigamid/wire_grimoire_refresh.go`: `RegisterModule("grimoire-refresh", ...)` (nama unik; terpasang alfabetis setelah `grimoire` dan `grimoire-inject`). Di `OnReady`, ambil `Builder`, publisher event IPC, dan daftar project lewat `d.Lookup`; buat `Trigger`; daftarkan `provider.RegisterLaunchContribution` yang memanggil `Trigger.Kick` tanpa mengubah `LaunchContext.Args` dan tanpa menunggu.
2. Sambungkan `BuildIncremental`, `staleCount`, `building`, dan `stale` ke handler IPC `grimoire.build`, `grimoire.status`, `grimoire.list`: lewat fungsi `RegisterRefresh` bila registry IPC memungkinkan penimpaan handler; bila tidak, ubah tepat satu baris di `wire_grimoire.go` agar handler tiket 1 memanggil jalur inkremental (sebut di deskripsi PR). Perubahan IPC hanya menambah field.
3. `cmd/shikigamictl/cmd_grimoire.go`: tambah flag `--full` pada `grimoire build`; `grimoire status` menampilkan `staleCount` dan `building`.
4. `GrimoirePanel.svelte`: tambah slot aditif yang me-render `BuildIndicator`, `UpdateButton` di header dan `StaleBadge` pada baris bab/buku dari `stale/`. Impor dari `./stale/...`. Tanpa polling; refresh lewat `grimoire.changed`.
5. Verifikasi manual: generate grimoire di sebuah project, edit satu file modul, buat sesi baru (launch cepat), amati indikator build lalu pembaruan bab; hapus satu folder modul dan pastikan halamannya hilang; jalankan build kedua tanpa perubahan dan pastikan tidak ada penulisan.

## Guard
Cukup build berhasil, tanpa unit test:
- `go build ./...`
- `make build` (termasuk build frontend)
