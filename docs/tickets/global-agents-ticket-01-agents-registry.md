---
id: global-agents-ticket-01-agents-registry
title: Agent Global: Registry dan Settings
spec: docs/superpowers/specs/2026-11-22-global-agents-design.md
status: implemented
wave: 1
depends_on: []
---
# Agent Global: Registry dan Settings

## Tujuan

Saat ini provider (`claude`, `codex`, `shell`) tidak mengenal model, dan agent di thread atau mention selalu "provider polos". Epic ini membangun fondasi agent global: registry kategori dan agent yang tersimpan persisten di `<DataDir>/agents.json`, flag model pada peluncuran sesi (`--model` untuk claude, `-m` untuk codex), dan halaman Settings "Agent" untuk mengelolanya. Epic ini adalah fondasi bagi tiket mention spawn (wave 2) dan orchestrator di thread (wave 3), yang memakai `Store.Resolve`, IPC `agents.*`, dan `SessionSpec.Model`.

## Hasil yang bisa dijalankan

Setelah tiket ini selesai (`go build ./...` dan build frontend lulus, aplikasi jalan):

- Settings punya tab baru "Agent" (urutan 15, di antara Connector dan MCP).
- Empat kategori bawaan tampil: Orchestrator, Researcher, Planner, Executor, masing-masing dengan teks peran awal berbahasa Indonesia yang bisa diedit. Kategori bawaan tidak bisa dihapus.
- Pengguna bisa membuat kategori sendiri, mengubah/menghapusnya, dan membuat agent (nama, provider, model, default) di kategori mana pun. Contoh: agent `planner-x` di kategori Planner, provider claude, model X, ditandai default.
- Data persisten: tutup lalu buka ulang aplikasi, kategori dan agent tetap ada (`agents.json` mode 0600).
- Orchestrator tidak bisa diberi provider codex (dropdown menyaring, backend menolak).
- Sesi yang diluncurkan dengan `SessionSpec.Model = "X"` berjalan dengan `--model X` (claude) atau `-m X` (codex); model kosong berperilaku seperti sekarang. Belum ada UI yang mengisi `Spec.Model` selain lewat tiket berikutnya; tiket ini cukup menyediakan jalurnya dan memverifikasi lewat build.
- Belum ada (tiket berikutnya): `@agent` di mention men-spawn sesi (tiket 2), master thread memakai agent Orchestrator dan `thread_spawn` menerima `agent`/`category` (tiket 3), `SetDeleteGuard` terpasang (tiket 3).

## Batasan

In scope:
- Paket `internal/agents`: tipe, penyimpanan, seed, invarian, validasi, resolver, IPC, event.
- Pemasangan modul daemon `cmd/shikigamid/wire_agents.go`.
- Field `Model` di `SessionSpec`, `LaunchContext`, dan flag di `claudeProvider.Command` / `codexProvider.Command`.
- Modul frontend `frontend/src/modules/settings/agents/` dan registrasi halaman Settings.

Out of scope:
- Spawn lewat mention, autocomplete `@`, thread/orchestrator, brief peran (tiket 2 dan 3).
- Prompt, batasan tool, izin per agent; agent per workspace/project; sinkronisasi antar mesin; daftar model otomatis dari provider.
- Fallback diam-diam ke provider lain bila binary hilang (anti-tujuan: harus error jelas).
- Mengubah `appconfig` atau `config.json` (agent tidak disimpan di sana).
- Mengubah `SettingsModal`.
- Unit test.

## Kontrak

Disediakan epic ini (dipakai tiket 2 dan 3):

Tipe (JSON camelCase):
```
Category { id, name, role string; builtin bool }
Agent    { id, name, categoryId, providerId, model string; isDefault bool }
```
- Id kategori bawaan: `orchestrator`, `researcher`, `planner`, `executor`. Kategori buatan pengguna: id dibangkitkan (slug nama + sufiks unik bila bentrok).
- Nama agent: slug huruf kecil (`[a-z0-9-]`), unik global. Input dinormalisasi (lowercase, spasi menjadi `-`); hasil kosong atau karakter ilegal = galat.

IPC daemon (method request/response):
- `agents.list` -> `{categories: Category[], agents: Agent[]}`
- `agents.categoryUpsert` `{id?, name, role}` -> `Category` (tanpa id = buat baru, `builtin=false`; dengan id = ubah nama+peran, termasuk kategori bawaan)
- `agents.categoryDelete` `{id, agents?: 'move'|'delete', moveTo?: categoryId}` -> `{}`. Kategori bawaan ditolak. Bila kategori punya agent dan `agents` kosong = galat; `move` butuh `moveTo` valid dan memindahkan agent (cek ulang aturan orchestrator/MCP di tujuan, dan default di tujuan: agent yang dipindah tidak merebut default yang sudah ada); `delete` menghapus agent-nya.
- `agents.upsert` `{id?, name, categoryId, providerId, model, isDefault}` -> `Agent`
- `agents.delete` `{id}` -> `{}` (memanggil delete guard bila terpasang; bila agent default dihapus dan masih ada agent lain di kategori, default tidak dialihkan otomatis kecuali tersisa tepat satu agent, lalu agent itu jadi default; aturan ini didokumentasikan di komentar kode)
- `agents.setDefault` `{id}` -> `{}` (mencabut default lama di kategori yang sama)
- Event `agents.changed` (payload bebas, minimal `{}`) dipancarkan setelah tiap mutasi sukses.

Go (`internal/agents`):
- `type Store struct`; `func Open(path string, reg ProviderLookup) *Store` atau pola setara yang meniru `appconfig.Open`; `DefaultPath(dataDir)` -> `<DataDir>/agents.json`.
- `func (s *Store) List() ([]Category, []Agent)` (salinan, urutan stabil: kategori menurut urutan dibuat, agent menurut nama).
- `func (s *Store) Resolve(ref string) (Agent, Category, error)`: satu-satunya tempat aturan resolusi. Urutan: (1) nama agent persis (setelah normalisasi slug); (2) id kategori; (3) nama kategori (case-insensitive). Kategori dipetakan ke agent default-nya; kategori tanpa default atau tanpa agent = galat jelas ("kategori <nama> belum punya agent default"), tidak menebak. Ref tak dikenal = galat `ErrNotFound`-style yang bisa dibedakan (`errors.Is`).
- `func (s *Store) SetDeleteGuard(fn func(agentID string) error)`: dipasang tiket 3; bila `fn` mengembalikan error, `agents.delete` dan `categoryDelete` mode `delete`/`move` yang menghapus/mengubah agent tersebut ditolak dengan galat itu. Tanpa guard = tidak ada penolakan.
- Registrasi IPC lewat fungsi `RegisterIPC(srv *ipc.Server, s *Store)` mengikuti pola paket lain (mis. `provider/ipc.go`).

Layanan di Deps: `d.Provide("agents.store", *agents.Store)`.

Sesi/provider:
- `session.SessionSpec.Model string` (kosong = default provider); `provider.LaunchContext.Model string`; claude `--model <m>`, codex `-m <m>`; diteruskan oleh `Launcher`. Fitur lain hanya mengisi `Spec.Model`.

Frontend:
- `registerSettingsPage({ id: 'agents', title: 'Agent', order: 15, component: AgentsPage })`.
- Modul lain tidak boleh mengimpor dari `modules/settings/agents`; mereka memanggil `agents.list` lewat klien sendiri.

Dipakai epic ini (sudah ada): `provider.Get(id)` / registry provider dan `Capabilities.MCP`; IPC `provider.list` (frontend: `loadProviders` di `frontend/src/lib/stores/providers.ts`); `RegisterModule`, `Deps.Provide`, `Deps.DataDir` di `cmd/shikigamid/modules.go`; pola tulis atomik 0600 di `internal/appconfig/appconfig.go`; `registerSettingsPage` di `frontend/src/modules/settings/pages.ts`; pola `connectors` (`client.ts`, `store.ts`, `ConnectorsPage.svelte`).

## Fitur

Ketiga fitur dapat dikerjakan paralel. `agents-settings-page` hanya bergantung pada kontrak IPC di atas (bukan pada implementasinya), jadi bisa ditulis bersamaan; verifikasi akhirnya baru bermakna setelah `agents-store-ipc` ada (lihat Wiring).

### agents-store-ipc: Penyimpanan, validasi, resolver, dan IPC agents

Deskripsi:
Buat paket `internal/agents` di sisi Go.
- Penyimpanan: berkas `<DataDir>/agents.json` berisi `{categories: [...], agents: [...]}` (tambahkan field `version: 1` untuk migrasi di masa depan). Tulis atomik (berkas sementara di direktori sama, `fsync`/rename) dengan mode 0600, meniru `appconfig.write`. Berkas hilang atau kosong = seed. Berkas rusak (JSON tidak valid) = jangan menimpa diam-diam: kembalikan galat jelas saat operasi, atau cadangkan ke `agents.json.bak` lalu seed ulang dan catat log; pilih salah satu dan dokumentasikan di komentar. Akses dilindungi mutex.
- Seed: empat kategori bawaan (`orchestrator`, `researcher`, `planner`, `executor`; `builtin=true`) dengan teks peran awal bahasa Indonesia, tanpa agent. Contoh peran: Orchestrator "Mengoordinasi pekerjaan: memecah tujuan, memilih agent yang tepat, dan memantau hasilnya; tidak mengerjakan implementasi sendiri."; Researcher "Menyelidiki dan mengumpulkan informasi; hanya membaca, tidak mengubah kode."; Planner "Hanya merencanakan: menyusun rencana langkah demi langkah; tidak mengubah kode."; Executor "Mengerjakan perubahan kode sesuai rencana dan melaporkan hasilnya." Seed hanya mengisi kategori bawaan yang belum ada (agar kategori bawaan yang hilang dari berkas tetap pulih), tidak menimpa nama/peran yang sudah diubah pengguna.
- Invarian: nama agent unik; paling banyak satu default per kategori (menandai default mencabut yang lama); agent pertama di kategori menjadi default otomatis walau `isDefault=false`; upsert dengan `isDefault=true` merebut default; mengubah `categoryId` agent default ke kategori lain tidak boleh meninggalkan kategori asal dengan lebih dari satu default dan harus mengikuti aturan default di kategori tujuan.
- Validasi pada upsert agent: `providerId` terdaftar di provider registry (`provider.Get`) dan bukan `shell`; kategori ada; bila `categoryId == "orchestrator"` provider harus `Capabilities().MCP == true`; nama valid dan unik (kecuali dirinya sendiri). Galat berbahasa Indonesia dan spesifik (mis. "provider codex tidak mendukung MCP, tidak bisa jadi Orchestrator"). Untuk menghindari impor siklus dan memudahkan uji manual, Store menerima antarmuka kecil pencari provider (`Get(id) (provider.Provider, bool)`) yang default-nya registry global.
- Kategori: nama wajib dan unik (case-insensitive); `categoryDelete` ikuti kontrak (bawaan ditolak, punya agent ditolak kecuali opsi `move`/`delete`).
- Resolver `Resolve(ref)` sesuai kontrak. Tidak boleh ada jalur lain yang menebak agent.
- `SetDeleteGuard(func(agentID string) error)` sesuai kontrak; default nil.
- IPC: `RegisterIPC` mendaftarkan enam method dan memancarkan event `agents.changed` lewat mekanisme event `ipc.Server` yang sudah ada (lihat cara modul lain memancarkan event). Komentar dan galat bahasa Indonesia.
- Modul daemon `cmd/shikigamid/wire_agents.go`: `RegisterModule("agents", func(srv *ipc.Server, d *Deps){ store := agents.Open(...); d.Provide("agents.store", store); agents.RegisterIPC(srv, store) })` mengikuti pola `wire_*.go` lain.

Acceptance criteria:
- [ ] `go build ./...` lulus.
- [ ] Pada data dir kosong, `agents.list` mengembalikan empat kategori bawaan dengan peran terisi dan nol agent; `agents.json` terbentuk 0600.
- [ ] `agents.upsert` agent pertama di kategori otomatis `isDefault=true`; agent kedua dengan `isDefault=true` mencabut default pertama; `agents.setDefault` berfungsi sama.
- [ ] Upsert dengan provider tidak terdaftar atau `shell` ditolak; Orchestrator dengan `codex` ditolak, dengan `claude` diterima.
- [ ] Nama agent duplikat (setelah normalisasi) ditolak; nama dinormalisasi menjadi slug huruf kecil.
- [ ] `agents.categoryDelete` pada kategori bawaan ditolak; pada kategori dengan agent tanpa opsi ditolak; dengan `move` + `moveTo` memindahkan agent; dengan `delete` menghapus agent-nya.
- [ ] `Resolve` dengan nama agent, id kategori, dan nama kategori mengembalikan hasil benar; kategori tanpa default menghasilkan galat jelas; ref tak dikenal menghasilkan galat yang bisa dibedakan lewat `errors.Is`.
- [ ] `SetDeleteGuard` yang mengembalikan error menolak `agents.delete`.
- [ ] Setelah proses dimulai ulang, data tetap sama; event `agents.changed` terpancar setelah tiap mutasi sukses.
- [ ] Layanan `agents.store` tersedia lewat `d.Lookup`.

ownedPaths: `internal/agents/`, `cmd/shikigamid/wire_agents.go`

Paralel: bisa dikerjakan paralel dengan `provider-model-flag` dan `agents-settings-page`. Tidak menyentuh `internal/session` atau `internal/provider` (hanya memakai API yang sudah ada).

### provider-model-flag: Flag model di provider dan sesi

Deskripsi:
Tambah jalur model dari `SessionSpec` sampai baris perintah provider.
- `internal/session/types.go`: tambah `Model string` di `SessionSpec` (tag JSON `model,omitempty`, ikuti gaya field lain). Periksa apakah `SessionSpec` dipersist atau dipakai di pemulihan; field baru harus kompatibel dengan data lama (kosong).
- `internal/session/launcher.go`: teruskan `Spec.Model` ke `LaunchContext.Model` saat membangun konteks.
- `internal/provider/launch.go`: tambah `Model string` di `LaunchContext` dengan komentar; normalisasi (`trim`) bila ada helper normalisasi konteks.
- `claudeProvider.Command`: bila `ctx.Model != ""` tambahkan `--model <m>`; `codexProvider.Command`: bila tidak kosong tambahkan `-m <m>`. Jamin tidak duplikat: bila `ctx.Args` sudah memuat `--model`/`-m` (atau bentuk `--model=...`), jangan menambah lagi (dan jangan menimpa; dokumentasikan prioritas di komentar). Buat helper kecil di `launch.go` serupa `withFlag` yang menangani flag ber-nilai. Model kosong = perilaku sekarang, byte per byte.
- Kegagalan peluncuran karena model tidak valid harus terlihat: periksa bagaimana kegagalan proses cepat (exit awal) dilaporkan sekarang (status `done`/exit, output di terminal). Pastikan sesi tidak dihapus diam-diam saat proses keluar segera: output galat CLI tetap terbaca di terminal dan status mencerminkan keluar. Bila ada jalur yang menelan galat/exit awal, perbaiki secukupnya di file yang dimiliki; bila tidak, catat di komentar bahwa perilaku sudah memenuhi. Jangan memvalidasi nama model di sisi aplikasi (daftar model tidak dikenal).

Acceptance criteria:
- [ ] `go build ./...` lulus.
- [ ] `SessionSpec.Model` dan `LaunchContext.Model` ada; `Launcher` meneruskannya.
- [ ] Command claude dengan Model "X" memuat tepat satu `--model X`; codex memuat tepat satu `-m X`; Model kosong menghasilkan args identik dengan sebelumnya.
- [ ] Args yang sudah memuat flag model tidak digandakan.
- [ ] Sesi dengan model tidak valid tidak mati senyap: keluar awal terlihat sebagai status/ output di sesi (diverifikasi dengan membaca kode jalur exit).
- [ ] Tidak ada perubahan di `internal/agents`.

ownedPaths: `internal/session/types.go`, `internal/session/launcher.go`, `internal/provider/launch.go`, `internal/provider/claude.go`, `internal/provider/codex.go`

Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini; tidak saling bergantung (agents-store-ipc hanya memakai `Capabilities` dan `Get` yang sudah ada).

### agents-settings-page: Halaman Settings Agent

Deskripsi:
Modul `frontend/src/modules/settings/agents/`:
- `client.ts`: pembungkus `call(...)` (seperti `connectors/client.ts`) untuk `agents.list`, `agents.categoryUpsert`, `agents.categoryDelete`, `agents.upsert`, `agents.delete`, `agents.setDefault`, dengan tipe `Category`, `Agent`, dan `AgentsSnapshot`.
- `store.ts`: store `categories`, `agents`, `agentsLoading`, `agentsError`, `selectedCategoryId`; `loadAgents()` dengan `loadSeq` untuk mengabaikan respons basi (pola `connectors/store.ts`); langganan event `agents.changed` yang memicu `loadAgents()` (gunakan helper langganan event frontend yang sudah dipakai modul lain; berhenti berlangganan bila perlu agar tidak bocor). Seleksi kategori dijaga valid setelah muat ulang (jatuh ke kategori pertama bila yang terpilih hilang).
- `AgentsPage.svelte`: tata letak mengikuti `ConnectorsPage`. Kiri: daftar kategori (peran tablist/listbox, klik/Enter memilih) dan tombol "Kategori baru". Kanan: judul kategori, tombol ubah nama dan hapus (hapus disembunyikan/nonaktif untuk bawaan), textarea "Peran" yang disimpan saat blur atau tombol "Simpan" (tampilkan status simpan/galat), lalu daftar agent kategori itu: nama, provider, model (atau "default provider" bila kosong), penanda default berupa teks "Default" (lencana berteks, bukan hanya warna; boleh disertai ikon), tombol edit, hapus, dan "Jadikan default" untuk agent non-default; tombol "Agent baru". Keadaan kosong: "Belum ada agent di kategori ini." Penghapusan kategori yang punya agent menampilkan dialog dengan pilihan "Pindahkan agent ke kategori lain" (dropdown tujuan) atau "Hapus agent juga"; penghapusan agent meminta konfirmasi. Galat backend (mis. guard, validasi) ditampilkan apa adanya di UI.
- Form agent (dialog/modal, ikuti pola `ConnectDialog.svelte` dan `Modal.svelte` untuk fokus, Escape, `aria-modal`, label): nama, provider (select dari store provider yang ada via `provider.list`/`loadProviders`: hanya yang tersedia, selain `shell`; bila kategori `orchestrator`, hanya yang `capabilities.mcp === true`; jika agent yang diedit punya provider yang kini tidak valid, tampilkan tetapi beri peringatan), model (input teks bebas, placeholder "default provider"), checkbox "Jadikan default". Validasi sisi klien ringan (nama wajib), sisanya dari backend.
- `index.ts`: `registerSettingsPage({ id: 'agents', title: 'Agent', order: 15, component: AgentsPage })`, memuat data saat halaman pertama dibuka (bukan saat startup).
- Teks UI bahasa Indonesia; identifier bahasa Inggris. Pastikan tidak ada mutasi langsung pada store dari komponen selain lewat fungsi store/klien.

Acceptance criteria:
- [ ] Build frontend lulus (termasuk `svelte-check`/typecheck bila ada di skrip build).
- [ ] Halaman menampilkan kategori dan agent dari `agents.list`; memilih kategori mengganti panel kanan.
- [ ] Kategori baru/ubah/hapus dan agent baru/ubah/hapus/jadikan default memanggil IPC yang benar dan UI ter-refresh (lewat event atau muat ulang setelah mutasi).
- [ ] Penanda default berteks "Default", bukan hanya warna.
- [ ] Dropdown provider tidak memuat `shell`, hanya yang tersedia; untuk Orchestrator hanya yang MCP true.
- [ ] Model kosong tampil sebagai "default provider"; input model memakai placeholder yang sama.
- [ ] Dialog dapat dioperasikan keyboard (fokus masuk, Escape menutup, label terhubung).
- [ ] Respons `agents.list` basi tidak menimpa yang lebih baru (`loadSeq`).
- [ ] Tidak ada perubahan pada `SettingsModal.svelte`.

ownedPaths: `frontend/src/modules/settings/agents/`

Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini; hanya bergantung pada kontrak IPC di atas.

## Path yang dimiliki

Hanya ini yang boleh disentuh:
- `internal/agents/`
- `cmd/shikigamid/wire_agents.go`
- `frontend/src/modules/settings/agents/`
- `internal/session/types.go`
- `internal/session/launcher.go`
- `internal/provider/launch.go`
- `internal/provider/claude.go`
- `internal/provider/codex.go`
- Pengecualian wiring (lihat bagian Wiring): satu baris impor sisi-efek di `frontend/src/modules/settings/index.ts`, tidak ada perubahan lain di berkas itu.

## Wiring

Langkah penutup tiket ini, sehingga hasilnya terlihat dan jalan tanpa tiket lain:
1. Backend: `cmd/shikigamid/wire_agents.go` mendaftarkan modul `agents` lewat `init()` + `RegisterModule` (berkas `wire_*.go` di package `main` otomatis ikut build; tidak perlu mengubah `modules.go`). Pastikan `d.Provide("agents.store", store)` dan `agents.RegisterIPC` terpanggil, dan path penyimpanan memakai `d.DataDir`.
2. Frontend: `main.ts` hanya memuat `./modules/*/index.ts` (glob satu tingkat), sehingga `settings/agents/index.ts` tidak termuat otomatis. Tambahkan tepat satu baris `import './agents';` di `frontend/src/modules/settings/index.ts` (setelah impor yang ada, sebelum registrasi lain bila urutan tidak berpengaruh; registrasi memakai `order: 15` sehingga posisi tab ditentukan oleh `order`). Ini satu-satunya perubahan yang diizinkan di berkas itu dan tidak mengubah `SettingsModal`. Bila ada titik registrasi yang lebih tepat yang sudah ada saat pengerjaan, boleh dipakai selama tidak mengubah `SettingsModal`.
3. Peluncuran sesi: pastikan `Launcher` meneruskan `Spec.Model` sehingga fitur tiket lain cukup mengisi `Spec.Model`.
4. Jalankan aplikasi: buka Settings, tab "Agent" tampil di antara Connector dan MCP; buat agent Planner (claude, model X) dan tandai default; restart aplikasi dan pastikan data tetap ada; coba membuat agent Orchestrator dengan codex dan pastikan tidak bisa (dropdown tidak menawarkan; backend menolak).

## Guard

Cukup build berhasil, tanpa unit test:
- `go build ./...`
- Build frontend (perintah build/typecheck di `frontend/`, mis. `npm run build` dan `npm run check` bila ada).
