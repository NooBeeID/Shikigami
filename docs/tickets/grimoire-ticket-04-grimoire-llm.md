---
id: grimoire-ticket-04-grimoire-llm
title: Grimoire: ringkasan LLM opt-in
spec: docs/superpowers/specs/2026-11-22-grimoire-design.md
status: ready
wave: 4
depends_on: [grimoire-ticket-03-grimoire-refresh]
---
# Grimoire: ringkasan LLM opt-in

## Tujuan
Grimoire hasil tiket 1-3 hanya berisi kerangka statik (struktur modul, path, hash). Tiket ini menambahkan lapisan ringkasan oleh agent headless (`claude -p`) yang menjelaskan "mengapa" di tiap modul, tetapi hanya sebagai opt-in per project karena berbiaya. Biaya dijaga lewat tiga pagar: flag `llm` default mati, dry-run estimasi sebelum flag dinyalakan, dan LLM hanya dipanggil untuk modul yang hash-nya berubah (opsional dibatasi `maxModules`). Kegagalan LLM tidak boleh merusak build maupun peluncuran sesi.

## Hasil yang bisa dijalankan
Di atas tiket 1-3, setelah tiket ini selesai:
- Panel Grimoire punya kontrol "Ringkasan LLM" (toggle, default mati). Sebelum dinyalakan, panel menampilkan pratinjau estimasi: jumlah modul antrean dan estimasi token (dari `grimoire.estimate`, tanpa memanggil LLM). Ada input opsional `maxModules` (kosong = tanpa batas).
- Dengan flag menyala dan `claude` terpasang serta login, build (manual lewat tombol/`shikigamictl grimoire build`, atau latar belakang saat sesi baru) meringkas modul yang hash-nya berubah. Bab hasilnya berisi ringkasan dan baris `Sumber:` di baris pertama. Panel menunjukkan progres per modul.
- Modul yang gagal/timeout, atau melewati `maxModules`, ditandai `warning: stale` di index dan panel; build tetap selesai.
- Bila `claude` tidak terpasang atau belum login, build jatuh ke mode statik tanpa error fatal.
- `shikigamictl grimoire status` menampilkan `llmEnabled` dan biaya build terakhir; `shikigamictl grimoire status --dry-run` menampilkan estimasi modul dan token. Panel menampilkan baris biaya build terakhir.
- Sesi pertama tetap memakai index statik; ringkasan menyusul lewat build latar belakang untuk sesi berikutnya.
- `go build ./...` dan `make build` lulus; aplikasi berjalan.

## Batasan
In scope:
- Settings per project `{llm, maxModules}` di `<DataDir>/grimoire/<projectId>/settings.json`.
- Method IPC `grimoire.settings.get/set` dan `grimoire.estimate`.
- Implementasi `Summarizer` berbasis `claude -p`, prompt tetap, penanganan gagal/timeout/maxModules/ketiadaan claude.
- Pencatatan `lastBuildCost` dan penambahan field aditif `llmEnabled`, `lastBuildCost` pada `grimoire.status`.
- Perluasan CLI: flag `--dry-run` pada `shikigamictl grimoire status`, dan tampilan field baru.
- UI di `frontend/src/modules/grimoire/llm/` dan slot aditif pada `GrimoirePanel.svelte`.

Out of scope:
- Integrasi grimoire eksternal (`GrimoireURL/Key`), MCP grimoire, pembuatan laws otomatis, Windows.
- Mengubah format bab/index/state.json milik tiket 1-3 (field baru hanya aditif).
- Ringkasan untuk provider selain `claude` (Codex tidak dipakai sebagai peringkas).
- Penyuntikan ke agent (tiket 2), deteksi perubahan/pemicu (tiket 3): hanya dipakai lewat antarmuka.
- Dependency baru, unit test.
- Mengedit berkas milik tiket lain, kecuali satu baris panggilan pendaftaran bila terpaksa (sebutkan di Wiring) dan slot aditif di `GrimoirePanel.svelte`.

## Kontrak
Dipakai (dari tiket sebelumnya, nama persis ikuti kode yang ada; periksa saat implementasi):
- Interface `Summarizer` dan jalur build inkremental dari `grimoire-refresh` (`incremental.go`); titik tempat build memilih Summarizer (statik default) dan antrean modul stale/berubah dari `diff.go`.
- Kerangka statik per modul (`scan.go`, tiket 1): nama modul, daftar sumber, hash, tag.
- Pemicu build latar belakang sesi baru (`trigger.go`), yang wajib tetap tidak menunggu LLM.
- `grimoire.status` / pengukuran dari tiket 2 (`measure.go`); tiket ini menambah field.
- `GrimoirePanel.svelte` (slot pengaturan), store per project aktif, event `grimoire.changed`.
- `internal/fsx`, `internal/paths`, `internal/ipc/errors.go` untuk penulisan berkas dan error.

Disediakan:
- `settings.json`: `{"llm": false, "maxModules": null}`. `llm` default `false`; `maxModules` `null` atau tidak ada = tanpa batas; nilai bukan `null` harus integer >= 1. Berkas tidak ada = default. Penulisan atomik lewat `fsx`.
- Go (paket `internal/grimoire`):
  - `type Settings struct { LLM bool \`json:"llm"\`; MaxModules *int \`json:"maxModules"\` }`
  - `LoadSettings(projectId string) (Settings, error)`, `SaveSettings(projectId string, s Settings) error`.
  - `type Estimate struct { Modules int \`json:"modules"\`; EstTokens int \`json:"estTokens"\` }` dan `EstimateBuild(projectId string) (Estimate, error)`.
  - `type BuildCost struct { Modules int \`json:"modules"\`; EstTokens int \`json:"estTokens"\`; DurationMs int64 \`json:"durationMs"\`; Failed int \`json:"failed"\`; At string \`json:"at"\` }` disimpan di `state.json` sebagai field aditif `lastBuildCost` (atau berkas pendamping bila lebih aman terhadap pemilik `state.json`; putuskan saat plan, jangan mengubah arti field yang ada).
  - `NewLLMSummarizer(...)` yang memenuhi interface `Summarizer`.
- IPC (JSON, error mengikuti `internal/ipc/errors.go`):
  - `grimoire.settings.get {projectId}` -> `{llm:boolean, maxModules:int|null}`.
  - `grimoire.settings.set {projectId, llm?, maxModules?}` -> settings terbaru. Field yang tidak dikirim tidak diubah; `maxModules: null` eksplisit menghapus batas. Validasi: `maxModules` < 1 -> error parameter tidak valid. Setelah set, publikasikan `grimoire.changed {projectId}`.
  - `grimoire.estimate {projectId}` -> `{modules:int, estTokens:int}`. Tidak memanggil LLM, tidak menulis berkas grimoire selain tidak ada.
  - `grimoire.status` ditambah field aditif: `llmEnabled:boolean`, `lastBuildCost:{modules,estTokens,durationMs,failed,at}|null`.
- Event `grimoire.changed {projectId}` dipublikasikan juga saat settings berubah dan di akhir modul yang selesai diringkas (agar progres tampil tanpa polling).
- CLI: `shikigamictl grimoire status --dry-run` memanggil `grimoire.estimate`; output `status` biasa menampilkan `llmEnabled` dan `lastBuildCost`.

## Fitur

### settings-estimate: Settings project dan dry-run estimasi
Deskripsi: `internal/grimoire/settings.go` mengelola `settings.json` per project (default `llm=false`, `maxModules=null`). `internal/grimoire/estimate.go` menghitung antrean modul (modul baru atau hash berubah dibanding `state.json`, memakai fungsi diff milik tiket 3, ditambah modul bertanda stale) dan estimasi token: ukuran byte berkas sumber modul + panjang prompt tetap + perkiraan keluaran per modul, dibagi konstanta byte per token yang sama dengan yang dipakai `measure.go` (gunakan ulang, jangan menduplikasi heuristik). Bila `maxModules` terisi, estimasi hanya menghitung modul sampai batas dan melaporkan sisanya sebagai di atas batas (cukup di `modules` yang dipangkas; field tambahan opsional aditif). Daftarkan method IPC `grimoire.settings.get/set` dan `grimoire.estimate` lewat fungsi `RegisterLLM(...)` di paket `internal/grimoire` (dipanggil dari `wire_grimoire_llm.go`). Perluas `cmd/shikigamictl/cmd_grimoire.go` secara aditif: flag `--dry-run` pada `status`, serta baris `llmEnabled`/biaya terakhir.

Acceptance criteria:
- [ ] `settings.json` dibuat hanya saat `settings.set` pertama; tanpa berkas, `get` mengembalikan `{llm:false, maxModules:null}`.
- [ ] `set` dengan `maxModules: 0` atau negatif ditolak dengan error ipc yang sesuai; `maxModules: null` menghapus batas; field yang tidak dikirim tidak berubah.
- [ ] `grimoire.estimate` pada project tanpa grimoire mengembalikan jumlah modul seluruh project (semua dianggap baru) tanpa menulis apa pun dan tanpa menjalankan `claude`.
- [ ] `grimoire.estimate` setelah build penuh tanpa perubahan mengembalikan `modules: 0`.
- [ ] `shikigamictl grimoire status --dry-run [--project <id>]` mencetak jumlah modul dan estimasi token.
- [ ] Penulisan settings atomik dan berizin sama dengan berkas grimoire lain; tidak ada berkas ditulis di dalam repo proyek.

ownedPaths:
- `internal/grimoire/settings.go`
- `internal/grimoire/estimate.go`
- (aditif, disebut di Wiring) `cmd/shikigamictl/cmd_grimoire.go`

Fitur ini bisa dikerjakan paralel dengan `llm-summarizer` dan `llm-ui`; kontrak tipe `Settings`/`Estimate` di atas yang menjadi pegangan bersama. Kecuali `llm-summarizer` membaca `Settings` lewat `LoadSettings`, yang cukup dipasang sebagai stub sementara bila fitur ini belum selesai.

### llm-summarizer: Summarizer claude -p
Deskripsi: `internal/grimoire/summarize.go` dan `prompt.go` mengimplementasikan `Summarizer` yang, per modul dalam antrean, menjalankan `claude -p` secara headless (model murah sebagai default, dapat diubah lewat konstanta/env yang terdokumentasi di kode; jangan menggandakan flag izin sesi interaktif, cukup flag yang diperlukan mode `-p`) dengan cwd project. `prompt.go` memuat prompt tetap (template konstan): kerangka statik modul + isi berkas sumber modul (dipotong ke batas ukuran per modul yang eksplisit; modul terlalu besar dipotong dengan penanda) dan instruksi: tulis ringkasan snapshot fakta tanpa tanggal/changelog/narasi proses, jelaskan tanggung jawab modul, hubungan antarmodul, dan "mengapa" yang terlihat dari kode, baris pertama wajib `Sumber: <path relatif repo>, ...`. Keluaran divalidasi: baris pertama harus diawali `Sumber:`; bila tidak, prefiks baris `Sumber:` dibangun dari daftar sumber modul di state. Bab ditulis ulang utuh (bukan patch) ke `books/<nama_snake>/<bab_snake>.md` lewat `fsx`.

Perilaku wajib:
- Hanya modul dalam antrean (hash berubah/baru/stale) diringkas; modul tak berubah tidak pernah memanggil LLM.
- `maxModules` terisi: proses sampai batas, modul sisanya ditandai `stale: true` di `state.json` (index memberi `warning: stale, jalankan grimoire build`) dan bab lama dipertahankan.
- Timeout per modul (konstanta, mis. 120 detik, dengan context) dan kegagalan proses/keluaran kosong: modul ditandai stale, build lanjut ke modul berikutnya dan tidak dibatalkan.
- Deteksi `claude` (`exec.LookPath`) dan login (jalankan pemeriksaan ringan atau tangani kegagalan autentikasi dari panggilan pertama): bila tidak ada/gagal login, seluruh build jatuh ke mode statik (kerangka statik tetap ditulis), tanpa menandai error fatal; catat alasan di log daemon dan di `lastBuildCost.failed`/pesan status.
- Flag `llm=false` (default): Summarizer statik dipakai dan `claude` tidak pernah dijalankan.
- Build LLM berjalan di latar belakang dan tidak menahan peluncuran sesi (pemicu tiket 3 sudah melepas peluncuran; pastikan jalur ini tidak menambah blokir). Sesi pertama memakai index statik.
- Publikasikan `grimoire.changed` setelah tiap modul selesai agar UI menampilkan progres; state `building` dari tiket 3 dipertahankan.
- Catat `lastBuildCost` di akhir build: jumlah modul diringkas, token perkiraan (estimasi dari ukuran prompt dan keluaran, bukan angka tagihan), durasi total, jumlah gagal, waktu.
- Satu build LLM per project dalam satu waktu (pakai mekanisme serialisasi/`building` yang ada dari tiket 3; jangan membuat kunci baru yang bertabrakan).
- `cmd/shikigamid/wire_grimoire_llm.go`: `RegisterModule` dengan nama unik `grimoire-llm`; memasang Summarizer LLM ke jalur build lewat `d.OnReady`/`d.Lookup` (modul dipasang alfabetis, jangan mengandalkan urutan), dan mendaftarkan method IPC fitur `settings-estimate` lewat `RegisterLLM`.

Acceptance criteria:
- [ ] Dengan `llm=true`, `claude` terpasang dan login: build mengganti bab modul yang berubah dengan ringkasan yang baris pertamanya `Sumber: ...`; modul yang tidak berubah tidak diubah dan tidak memicu panggilan.
- [ ] Dengan `llm=false`: tidak ada proses `claude` yang dijalankan.
- [ ] Dengan `maxModules=N` dan antrean lebih dari N: tepat N modul diringkas, sisanya `stale: true` dan tampil dengan `warning: stale` di index.
- [ ] Satu modul timeout atau gagal: modul itu stale, modul lain tetap diringkas, build selesai tanpa error.
- [ ] `claude` tidak ada di PATH atau belum login: build selesai dalam mode statik, tanpa panic, dan `grimoire.status` tetap valid.
- [ ] Peluncuran sesi tidak menunggu LLM (build latar belakang); sesi pertama memakai index statik.
- [ ] `lastBuildCost` terisi setelah build dan muncul pada `grimoire.status` (aditif, `null` bila belum pernah build LLM).
- [ ] Tidak ada berkas ditulis di dalam repo proyek; proses `claude` hanya dimatikan lewat context/PID miliknya sendiri.
- [ ] Format bab tetap: `Sumber:` di baris pertama, tanpa tanggal/changelog.

ownedPaths:
- `internal/grimoire/summarize.go`
- `internal/grimoire/prompt.go`
- `cmd/shikigamid/wire_grimoire_llm.go`

Fitur ini bisa dikerjakan paralel dengan `settings-estimate` dan `llm-ui`; ketergantungan pada `LoadSettings`/`Settings` dan `BuildCost` bersifat kontrak tipe (lihat Kontrak), sehingga cukup disepakati nama tipenya.

### llm-ui: Toggle LLM, estimasi, dan biaya di panel
Deskripsi: `frontend/src/modules/grimoire/llm/` berisi komponen Svelte (mis. `LlmSettings.svelte`, `llmStore.ts`) untuk dipasang di slot aditif `GrimoirePanel.svelte` (hanya tambahan, tidak mengubah logika panel milik tiket 1). Isi:
- Toggle "Ringkasan LLM" yang membaca `grimoire.settings.get`. Saat pengguna hendak menyalakan (flag masih mati), panggil `grimoire.estimate` dan tampilkan pratinjau "N modul, ~T token" dengan tombol konfirmasi; toggle baru menjalankan `grimoire.settings.set {llm:true}` setelah konfirmasi. Mematikan tidak perlu konfirmasi.
- Input angka opsional `maxModules` (kosong = tanpa batas, validasi >= 1), disimpan lewat `grimoire.settings.set`.
- Indikator progres per modul saat `building` benar: tampilkan modul yang sedang/sudah diringkas dari data status/list yang diperbarui oleh event `grimoire.changed`.
- Baris biaya build terakhir dari `grimoire.status.lastBuildCost` (modul diringkas, token perkiraan, durasi, jumlah gagal), dan pesan bila jatuh ke mode statik.
- Pembaruan hanya lewat event `grimoire.changed`; tanpa `setInterval`/polling. Store per project aktif mengikuti pola store modul grimoire. Gunakan token tema yang ada, tanpa dependency baru. Bila daemon lama belum mengirim field baru (`llmEnabled` undefined), UI menganggap mati dan tetap render.

Acceptance criteria:
- [ ] Toggle tampil di panel; default mati untuk project baru.
- [ ] Sebelum menyalakan, pratinjau estimasi (modul dan token) tampil dan baru setelah konfirmasi flag disimpan.
- [ ] Input `maxModules` menolak nilai < 1, kosong berarti tanpa batas, dan tersimpan di `settings.json`.
- [ ] Progres per modul tampil saat build LLM berjalan dan hilang saat selesai, hanya digerakkan oleh `grimoire.changed`.
- [ ] Baris biaya build terakhir tampil setelah build dengan data dari `grimoire.status`; tidak tampil bila `lastBuildCost` null.
- [ ] Pergantian project aktif memuat settings dan biaya project itu; tidak ada polling.
- [ ] Tampilan memakai token tema yang ada; build frontend lulus.

ownedPaths:
- `frontend/src/modules/grimoire/llm/`

Fitur ini bisa dikerjakan paralel dengan dua fitur lain; kontrak IPC di atas yang menjadi pegangan (boleh di-mock sementara). Titik sambungnya ke `GrimoirePanel.svelte` dilakukan di Wiring.

## Path yang dimiliki
Hanya path ini yang boleh disentuh:
- `internal/grimoire/summarize.go`
- `internal/grimoire/prompt.go`
- `internal/grimoire/settings.go`
- `internal/grimoire/estimate.go`
- `cmd/shikigamid/wire_grimoire_llm.go`
- `frontend/src/modules/grimoire/llm/`

Sentuhan aditif minimum di berkas milik tiket lain, hanya sebagaimana dijelaskan di Wiring: `cmd/shikigamictl/cmd_grimoire.go` (flag `--dry-run` dan tampilan field baru), `frontend/src/modules/grimoire/GrimoirePanel.svelte` (satu slot), dan field aditif pada tipe status/`state.json` di `internal/grimoire/`.

## Wiring
Langkah penutup tiket ini, memakai titik ekstensi tiket 1-3 sehingga hasilnya terlihat dan jalan:
1. Daemon: `cmd/shikigamid/wire_grimoire_llm.go` mendaftarkan modul `grimoire-llm` via `RegisterModule`. Di dalamnya, ambil layanan grimoire/build/IPC lewat `d.OnReady`/`d.Lookup` (jangan mengandalkan urutan alfabetis), panggil `grimoire.RegisterLLM(...)` untuk method `grimoire.settings.get/set` dan `grimoire.estimate`, dan pasang Summarizer LLM ke jalur build inkremental (dipilih saat `llm=true` dan `claude` tersedia, statik selain itu). Jangan mengedit `wire_grimoire.go`; bila pemasangan Summarizer mutlak butuh satu hook di berkas tiket 3, batasi pada satu baris panggilan dan catat di deskripsi commit.
2. Status: tambahkan field `llmEnabled` dan `lastBuildCost` ke respons `grimoire.status` secara aditif (tidak mengubah field lama).
3. CLI: di `cmd/shikigamictl/cmd_grimoire.go` tambahkan flag `--dry-run` pada `status` (memanggil `grimoire.estimate`) dan tampilkan `llmEnabled`/`lastBuildCost`, tanpa mengubah perintah lain.
4. Frontend: pasang komponen dari `frontend/src/modules/grimoire/llm/` pada slot pengaturan aditif di `GrimoirePanel.svelte` (satu impor dan satu elemen).
5. Verifikasi manual: jalankan aplikasi; nyalakan toggle (lihat pratinjau estimasi), jalankan Generate, pastikan bab yang berubah berisi ringkasan dengan `Sumber:`, modul gagal/di atas batas tampil stale, dan baris biaya muncul. Matikan `claude` dari PATH untuk memeriksa fallback statik. Tiket ini tidak butuh tiket lain selain tiket 3 yang sudah selesai (wave sebelumnya) untuk dijalankan.

## Guard
Cukup build berhasil, tanpa unit test:
- `go build ./...`
- `make build` (termasuk build frontend)
