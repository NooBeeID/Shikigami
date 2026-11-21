---
id: ai-terminal-ticket-09-session-mention
title: Session Mention
spec: docs/superpowers/specs/2026-11-21-ai-terminal-design.md
status: implemented
wave: 6
depends_on: [ai-terminal-ticket-08-session-status, ai-terminal-ticket-06-ui-shell, ai-terminal-ticket-05-provider-adapters]
---
# Session Mention

## Tujuan

Epic ini mewujudkan "mention antar sesi" dari spec (bagian 7, fase 2): sesi bisa saling bertukar pesan lintas project, lintas sesi, dan lintas provider dalam satu workspace. Ada dua jalur: (1) agent ke agent lewat server MCP lokal milik daemon yang dipasang ke tiap sesi (tool `list_sessions`, `send_message`, `read_output`), dan (2) user ke sesi lewat `@nama-sesi` di UI. Risiko utama spec adalah menyuntik teks ke agent yang sedang sibuk, jadi pesan masuk antrean per sesi target dan baru dikirim saat sesi target idle (memakai Idle API dari tiket 08), dengan fallback minimum: mention dari user saja, dikirim setelah konfirmasi atau saat idle, bila injeksi ke agent sibuk terbukti tidak andal.

Tiket ini berdiri sendiri dan tidak boleh merusak build. Tiket 01 sampai 08 sudah ada dan lolos `make build`; setelah tiket ini selesai `make build` tetap lolos. Tidak ada polling: pengiriman digerakkan oleh event idle, bukan timer berkala.

## Batasan

In scope:
- Server MCP lokal di daemon (`internal/mention/`) dengan tiga tool dan pemasangan konfigurasinya ke tiap sesi lewat `LaunchContribution`.
- Antrean pesan per sesi target, pengiriman lewat `provider.SendInput` saat `status.OnIdle` terpicu, dan mode konfirmasi/fallback.
- Method IPC `mention.send`, `mention.queue.list` (dan pendukungnya), event antrean, serta perintah debug `aitermctl mention ...`.
- Modul frontend `mention`: input `@nama-sesi` dengan autocomplete dari sesi workspace aktif, dan panel tampilan antrean tertunda, didaftarkan lewat registry.

Out of scope:
- Server MCP untuk knowledge/Grimoire (fase 4), penataan project (fase 5), Windows, syntax highlighting.
- Tombol "mention file ke sesi" dari file viewer (ide lanjutan di spec, bukan versi awal).
- Mengubah file milik epic lain: `internal/provider/`, `internal/status/`, `internal/session/`, `internal/workspace/`, `internal/ipc/`, `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go`, `frontend/src/App.svelte`, `frontend/src/lib/*`, `frontend/src/modules/<modul lain>/`. Epic ini hanya memakai API-nya.
- Menambah dependency Go atau npm di `go.mod`/`frontend/package.json` (milik tiket 01). Implementasi MCP memakai `net/http` dan `encoding/json` dari stdlib saja (JSON-RPC 2.0 sederhana sesuai spesifikasi MCP "streamable HTTP", cukup `initialize`, `tools/list`, `tools/call`). Bila ternyata butuh library, laporkan sebagai blocker, jangan edit `go.mod`.
- Unit test (guard hanya build).

## Kontrak

### Yang dipakai (dari epic lain)

Dari tiket 01 (`internal/ipc`, `internal/paths`, registri):
- `srv.Register(method string, func(ctx context.Context, params json.RawMessage) (any, error))`, `srv.Publish(topic string, payload any)`, `client.Call`, `client.Subscribe`. Kode error: -32000 ERR_BINARY, -32001 ERR_CONFLICT, -32002 ERR_NOT_FOUND, -32003 ERR_INVALID.
- `RegisterModule("mention", func(srv *ipc.Server, d *Deps){...})` (`cmd/aitermd/modules.go`) dan `RegisterCommand(...)` (`cmd/aitermctl/registry.go`). Baca kedua file itu untuk bentuk `Deps` dan signature persis sebelum menulis wiring.
- `internal/paths` untuk `DataDir` (`~/Library/Application Support/aiterm`); socket dan berkas runtime mention ditaruh di bawahnya.
- Frontend: `frontend/src/lib/ipc.ts` (`call`, `subscribe`) dan `frontend/src/lib/registry.ts` (`registerPanel`, `registerTabType`).

Dari tiket 02 (`internal/session`): akses ke sesi (daftar sesi, ambil `Session{id, workspaceId, projectId, cwd, providerId, status, createdAt}`) dan ke output sesi (scrollback/ring buffer terbatas untuk `read_output`). Baca `internal/session` untuk nama persis API Go-nya (mis. getter manager sesi dari `Deps`); jika tidak ada API pembacaan scrollback publik, gunakan method IPC `session.attach` lewat client internal sebagai cadangan, jangan edit `internal/session`.

Dari tiket 04 (`internal/workspace`): pencarian workspace dan project sebuah sesi, agar tool MCP dibatasi pada workspace sesi pemanggil dan agar nama project bisa ditampilkan. Gunakan API Go yang tersedia di `Deps` atau method `workspace.*`.

Dari tiket 05 (`internal/provider`):
- `RegisterLaunchContribution(func(*LaunchContext))` di `internal/provider/launch.go`. `LaunchContext{Session, Provider, Env map[string]string, Args []string, MCPServers map[string]any}`. Epic ini mengisi `MCPServers["aiterm-mention"]` (dan env bila perlu); adapter provider yang menerjemahkan `MCPServers` ke format tiap provider (mis. `--mcp-config` untuk Claude). Epic ini tidak menulis format per provider; bila provider tidak mendukung MCP (model kapabilitas, spec 9b), kontribusi diabaikan oleh adapter dan sesi tersebut tetap bisa menerima mention dari user lewat UI.
- `provider.SendInput(sessionID, text)`: cara provider mengirim input ke sesi (menangani format paste/Enter per provider). Nama dan signature persis mengikuti `internal/provider`; baca dulu. Bila yang tersedia berupa method pada adapter, ambil adapter lewat registry provider dan sesi.

Dari tiket 08 (`internal/status`):
- `status.IsIdle(sessionID string) bool` dan `status.OnIdle(sessionID string, fn func())`. Nilai status: `running|waiting|done|idle`. Cara membatalkan/meng-unregister callback mengikuti API tiket 08; bila `OnIdle` bersifat sekali-pakai, daftarkan ulang setelah tiap pengiriman selama antrean masih berisi.

Dari tiket 06 (frontend): area panel `sidebar`, `main-tab`, `statusbar` dirender dari registry; store di `frontend/src/lib/stores/` (`sessions`, `workspaces`, `tabs`). Modul ini boleh mengimpor store (hanya baca) tetapi lebih disarankan memuat daftar sesi lewat `call('session.list', {workspaceId})` agar tidak bergantung pada nama ekspor store; workspace aktif diambil dari store `workspaces` (`activeWorkspaceId`).

### Yang disediakan

#### Server MCP lokal

- Transport: HTTP di loopback, `127.0.0.1` pada port acak yang dipilih saat daemon start (bind port 0), alamat disimpan di memori dan di berkas `<DataDir>/mention.json` (`{"url":"http://127.0.0.1:PORT/mcp"}`) untuk debug. Satu server untuk semua sesi (bukan satu proses per sesi). Alternatif unix socket diperbolehkan bila pemasangan ke provider lebih mudah, tetapi pilih satu dan jadikan konsisten.
- Identitas pemanggil: tiap sesi mendapat token acak (disimpan di memori, per sesi) yang dipasang pada konfigurasi MCP sesi itu (header `Authorization: Bearer <token>` pada konfigurasi MCP, atau path `/mcp/<token>` bila provider tidak mendukung header). Server memetakan token ke `sessionID` pemanggil; token tak dikenal ditolak (HTTP 401). Dengan begitu `send_message` tahu pengirimnya dan tool dibatasi pada workspace yang sama dengan pemanggil.
- Tool (dengan input JSON schema):
  - `list_sessions()` -> array `{id, name, projectName, providerId, status}` untuk semua sesi dalam workspace pemanggil (lintas project dan provider), menandai sesi pemanggil dengan `self: true`. `name` adalah nama mention (lihat Penamaan).
  - `send_message(to: string, text: string)` -> `{queued: bool, delivered: bool, position: int}`. `to` boleh berupa nama mention atau id sesi. Pesan masuk antrean target (lihat Antrean); bila target idle, dikirim segera.
  - `read_output(to: string, last_n: int)` -> teks `last_n` baris terakhir output sesi target (default 50, maksimum 500), dengan escape ANSI dibuang. Hanya sesi dalam workspace yang sama.
- Error tool memakai `isError: true` pada hasil `tools/call` dengan pesan singkat (mis. "sesi tidak ditemukan", "bukan workspace yang sama").
- Pemasangan: `internal/mention/install.go` memanggil `provider.RegisterLaunchContribution` dari `init()` atau dari fungsi yang dipanggil `wire_mention.go`. Kontribusi membuat token untuk `LaunchContext.Session`, lalu mengisi `ctx.MCPServers["aiterm-mention"] = map[string]any{"type":"http","url":<url>,"headers":{"Authorization":"Bearer <token>"}}`. Token dilepas saat sesi berakhir (`session.exit`).

#### Penamaan sesi (`@nama-sesi`)

Nama mention diturunkan deterministik dari sesi: `<providerId>-<projectName>` dengan huruf kecil, spasi menjadi `-`; bila bentrok dalam workspace, ditambah akhiran `-2`, `-3`, dst. berdasar urutan `createdAt`. Daftar ini dihitung oleh paket `mention` (fungsi `ResolveName(workspaceID, name) (sessionID, error)` dan `NameOf(sessionID) string`) dan dipakai oleh MCP, IPC, dan UI (UI mengambilnya lewat `mention.names`).

#### Antrean

- Antrean FIFO per sesi target, di memori (tidak wajib bertahan saat daemon restart, sesuai spec 9b). Batas 50 pesan per target; melebihi itu `ERR_CONFLICT`.
- `Message{id (ULID), from ("user" atau sessionID pengirim), fromName, to (sessionID), text, createdAt, state ("queued"|"awaiting_confirm"|"delivered"|"failed"), source ("agent"|"user")}`.
- Pengiriman: saat pesan masuk, bila `status.IsIdle(to)` maka kirim segera dengan `provider.SendInput`; selain itu tetap di antrean dan daftarkan `status.OnIdle(to, drain)`. `drain` mengirim pesan berurutan satu per satu; setelah satu pesan terkirim, sesi kemungkinan berubah ke `running`, jadi pesan berikutnya menunggu idle berikutnya (`OnIdle` didaftar ulang bila antrean belum kosong). Tidak ada timer polling.
- Format teks yang disuntik: baris tunggal berawalan sumber agar agent tujuan paham asalnya, mis. `[pesan dari <fromName>] <text>`; teks multi-baris dikirim apa adanya lewat mekanisme paste milik `provider.SendInput`, lalu Enter. Pesan terkirim -> `state: delivered`.
- Mode (konfigurasi di memori, bisa diubah lewat `mention.config`, default `confirm_agent`):
  - `auto`: semua pesan (agent maupun user) dikirim otomatis saat idle.
  - `confirm_agent` (default, mencerminkan fallback spec 9b): pesan dari user dikirim otomatis saat idle; pesan dari agent (`source: "agent"`) berstatus `awaiting_confirm` dan baru dikirim setelah user mengonfirmasi lewat `mention.confirm`. Alasannya injeksi ke agent sibuk belum terbukti andal.
  - `user_only`: pesan dari agent ditolak (`send_message` mengembalikan error "mode user_only"); hanya mention user yang diproses. Ini fallback minimum spec 9b.
- Event antrean: `srv.Publish("mention.queue", {sessionId, queue: Message[]})` setiap antrean target berubah (tambah, kirim, batal, konfirmasi).

#### Method IPC

- `mention.send` params `{workspaceId, to, text, from?}` (`to` = nama mention atau id sesi; `from` kosong berarti user) -> `Message`. `ERR_NOT_FOUND` bila sesi tidak ada, `ERR_INVALID` bila `text` kosong atau lintas workspace.
- `mention.queue.list` params `{sessionId?}` (kosong = seluruh antrean dalam semua sesi) -> `Message[]`.
- `mention.queue.cancel` params `{id}` -> `{ok:true}`; hanya untuk state `queued`/`awaiting_confirm`.
- `mention.confirm` params `{id}` -> `Message`; mengubah `awaiting_confirm` menjadi `queued` dan memicu pengiriman bila target idle.
- `mention.names` params `{workspaceId}` -> `[{sessionId, name, projectName, providerId, status}]` (untuk autocomplete UI).
- `mention.config` params `{mode?}` -> `{mode}` (baca atau ubah mode).
- `mention.info` (tanpa params) -> `{url, sessions: n}` (alamat server MCP, untuk debug).

#### CLI debug (`aitermctl`)

`cmd/aitermctl/cmd_mention.go` mendaftarkan perintah `mention` dengan subperintah: `send <to> <text>`, `queue [sessionId]`, `cancel <id>`, `confirm <id>`, `names <workspaceId>`, `mode [auto|confirm_agent|user_only]`, `info`. Output JSON per baris.

#### Frontend

- `frontend/src/modules/mention/index.ts` mendaftarkan:
  - `registerPanel({id: 'mention-composer', area: 'sidebar', component: MentionComposer, order: 30})`: input teks dengan `@nama-sesi`.
  - `registerPanel({id: 'mention-queue', area: 'statusbar', component: QueueBadge, order: 20})`: lencana jumlah pesan tertunda; klik membuka daftar antrean (popover atau panel).
- Tidak mendaftar `registerTabType` baru.

## Fitur

Ketiga fitur dapat dikerjakan paralel. Antarmuka antar fitur disepakati dari bagian Kontrak: `mcp-server` memanggil fungsi antrean `Queue.Enqueue(ctx, Message) (Message, error)`, `message-queue` menyediakannya; keduanya mengikuti signature pada Kontrak. Agar build lolos di tiap tahap, `mcp-server` mendefinisikan interface kecil `Enqueuer` di `internal/mention/tools.go` (`Enqueue(ctx, from, to, text string, source string) (Message, error)`) dan memakainya; `message-queue` mengimplementasikan interface itu. Tipe `Message` dan fungsi penamaan didefinisikan di `internal/mention/types.go` yang dibuat oleh fitur `message-queue` (satu-satunya file tambahan di `internal/mention/`); bila `mcp-server` dikerjakan lebih dulu, buat file tersebut dengan isi sesuai Kontrak lalu `message-queue` melengkapinya (hindari definisi ganda, build tetap lolos). `mention-ui` hanya bergantung pada nama method IPC pada Kontrak.

### mcp-server: Server MCP lokal

Deskripsi: `internal/mention/mcp.go` menjalankan HTTP server loopback (stdlib `net/http`) dengan handler JSON-RPC 2.0 untuk metode MCP `initialize` (kembalikan `protocolVersion`, `capabilities: {tools: {}}`, `serverInfo`), `notifications/initialized` (balas 202), `tools/list`, dan `tools/call`. `internal/mention/tools.go` mendefinisikan tiga tool beserta JSON schema input dan implementasinya, dan menyelesaikan pemanggil dari token. `internal/mention/install.go` membuat/melepas token per sesi dan mendaftarkan `LaunchContribution` ke `internal/provider` seperti pada Kontrak. Server dimulai oleh fungsi `Start(ctx, deps)` yang dipanggil dari `wire_mention.go`; berhenti saat konteks daemon dibatalkan. `read_output` mengambil scrollback sesi lewat API `internal/session` (atau `session.attach` sebagai cadangan), memotong `last_n` baris, dan membuang escape ANSI. `send_message` hanya memvalidasi dan memanggil `Enqueuer`; logika antrean ada di fitur `message-queue`.

Acceptance criteria:
- [ ] `go build ./...` lolos dengan `internal/mention/mcp.go`, `tools.go`, `install.go` ada.
- [ ] Server bind ke `127.0.0.1` saja (bukan `0.0.0.0`) dan alamatnya tersedia lewat `mention.info`/berkas `mention.json`.
- [ ] `tools/list` mengembalikan tepat tiga tool: `list_sessions`, `send_message`, `read_output`, dengan input schema valid.
- [ ] Request tanpa token valid ditolak 401; token memetakan ke sesi pemanggil.
- [ ] `list_sessions` hanya memuat sesi workspace pemanggil, lintas project dan provider, dengan nama mention dan status.
- [ ] `read_output` membatasi `last_n` (default 50, maks 500) dan tidak memuat escape ANSI.
- [ ] `send_message` ke sesi di luar workspace pemanggil mengembalikan error tool yang jelas.
- [ ] `LaunchContribution` terdaftar dan mengisi `MCPServers["aiterm-mention"]` untuk tiap sesi baru; token dilepas saat sesi berakhir.
- [ ] Tidak ada goroutine polling; server hanya bereaksi pada request.
- [ ] Tidak mengedit `internal/provider`, `internal/session`, `internal/status`.

ownedPaths: `internal/mention/mcp.go`, `internal/mention/tools.go`, `internal/mention/install.go`. Paralel dengan fitur lain, dengan catatan antarmuka `Enqueuer` dan `types.go` di atas.

### message-queue: Antrean pesan dan pengiriman saat idle

Deskripsi: `internal/mention/queue.go` menyimpan antrean per sesi target, menerapkan aturan pengiriman dan mode (`auto`, `confirm_agent`, `user_only`) dari Kontrak, mengirim lewat `provider.SendInput`, dan memakai `status.IsIdle`/`status.OnIdle` agar pengiriman digerakkan event (tanpa timer). Menangani siklus hidup: antrean sesi dihapus saat `session.exit` (pesan tertunda diberi `state: failed`), pendaftaran `OnIdle` tidak dobel, dan kegagalan `SendInput` menandai pesan `failed` tanpa menghentikan pesan lain. `internal/mention/ipc.go` mendaftarkan method `mention.send`, `mention.queue.list`, `mention.queue.cancel`, `mention.confirm`, `mention.names`, `mention.config`, `mention.info`, dan menerbitkan event `mention.queue`. `cmd/aitermd/wire_mention.go` berisi `init(){ RegisterModule("mention", func(srv *ipc.Server, d *Deps){ ... }) }` yang membangun `Queue`, mendaftarkan method IPC, dan memanggil `mention.Start` dari fitur `mcp-server` bila sudah ada (sambungkan lewat fungsi `Start` di paket `mention`; bila `mcp-server` belum digabung, `wire_mention.go` hanya menyambungkan antrean sehingga build tetap lolos, dan sambungan MCP ditambahkan setelah fitur tersebut tersedia). `cmd/aitermctl/cmd_mention.go` mendaftarkan perintah debug dari Kontrak lewat `RegisterCommand`.

Acceptance criteria:
- [ ] `go build ./...` lolos dengan `queue.go`, `ipc.go`, `wire_mention.go`, `cmd_mention.go` ada.
- [ ] `mention.send` ke sesi idle mengirim segera lewat `provider.SendInput` dan `state` menjadi `delivered`.
- [ ] `mention.send` ke sesi tidak idle menyimpan pesan `queued` dan mengirimnya saat `status.OnIdle` terpicu; urutan FIFO terjaga.
- [ ] Setelah satu pesan terkirim, pesan berikutnya menunggu idle berikutnya (tidak menyuntik berturut-turut ke agent yang baru mulai bekerja).
- [ ] Mode `confirm_agent` menahan pesan agent di `awaiting_confirm` sampai `mention.confirm`; pesan user tetap otomatis saat idle.
- [ ] Mode `user_only` menolak pesan agent dengan pesan error jelas; pesan user tetap jalan.
- [ ] `mention.queue.list`, `mention.queue.cancel`, `mention.names`, `mention.config`, `mention.info` bekerja; event `mention.queue` terbit pada tiap perubahan.
- [ ] Antrean dibersihkan saat sesi berakhir; pesan tertunda menjadi `failed`.
- [ ] Batas 50 pesan per target ditegakkan dengan `ERR_CONFLICT`.
- [ ] Tidak ada `time.Ticker`/polling; hanya callback idle dan request IPC.
- [ ] `aitermctl mention send|queue|cancel|confirm|names|mode|info` terdaftar dan terhubung ke method IPC.
- [ ] Tidak mengedit `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go`, `internal/status`, `internal/provider`.

ownedPaths: `internal/mention/queue.go`, `internal/mention/ipc.go`, `internal/mention/types.go` (file tipe bersama, dibuat oleh fitur yang lebih dulu dikerjakan), `cmd/aitermd/wire_mention.go`, `cmd/aitermctl/cmd_mention.go`. Paralel dengan fitur lain, dengan catatan antarmuka `Enqueuer` dan `types.go` di atas.

### mention-ui: @nama-sesi di UI

Deskripsi: modul frontend `modules/mention/` dengan dua komponen Svelte. (1) `MentionComposer.svelte` (panel sidebar): kotak teks tempat user mengetik pesan dengan `@nama-sesi`; saat mengetik `@`, muncul autocomplete dari `mention.names` untuk workspace aktif (nama, project, provider, status sesi), dipilih dengan panah/Enter/klik; `@nama` yang valid diurai dari teks (pola `@[a-z0-9-]+` di awal), sisa teks dikirim lewat `mention.send` dengan `from` kosong. Tersedia pilihan "sertakan output terakhir sesi aktif" (ambil `last_n` baris lewat `session.attach`, dibuang ANSI, ditempel ke pesan) untuk skenario meneruskan output seperti pada spec. Hasil `Message` ditampilkan singkat (terkirim/antre); error IPC ditampilkan sebagai pesan singkat, tidak membuat UI crash. (2) `QueueBadge.svelte` dan daftar antrean (statusbar): berlangganan event `mention.queue` lewat `subscribe`, menampilkan jumlah pesan tertunda, dan saat dibuka menampilkan tiap pesan (dari, ke, potongan teks, state) dengan tombol Batalkan (`mention.queue.cancel`) dan Konfirmasi (`mention.confirm`, hanya untuk `awaiting_confirm`). Muat awal lewat `mention.queue.list`, selanjutnya hanya lewat event (tanpa `setInterval`). Daftar nama dimuat ulang saat workspace aktif berubah atau event sesi terbit. `index.ts` mendaftarkan kedua panel seperti pada Kontrak. Styling memakai CSS variables dari `frontend/src/styles/` (gelap/terang), tanpa framework tambahan.

Acceptance criteria:
- [ ] Build frontend lolos (`vite build`) dengan `modules/mention/` ada, juga bila backend mention belum hidup (panggilan IPC gagal ditangani sebagai pesan error, bukan crash).
- [ ] Mengetik `@` memunculkan autocomplete dari sesi workspace aktif; dapat dipilih dengan keyboard dan mouse.
- [ ] Mengirim `@nama teks` memanggil `mention.send` dengan `to` dan `text` yang benar, dan UI menampilkan hasilnya.
- [ ] Opsi menyertakan output terakhir sesi aktif menambahkan teks output ke pesan.
- [ ] Statusbar menampilkan jumlah pesan tertunda dan ter-update lewat event `mention.queue` tanpa polling.
- [ ] Pesan `awaiting_confirm` bisa dikonfirmasi; pesan `queued` bisa dibatalkan dari daftar antrean.
- [ ] Panel terdaftar lewat `registerPanel` tanpa mengedit `lib/registry.ts`, `App.svelte`, atau modul lain.
- [ ] Tidak ada `setInterval`; subscription dibersihkan saat komponen di-destroy.

ownedPaths: `frontend/src/modules/mention/`. Paralel dengan fitur lain; bergantung hanya pada nama method/event IPC pada Kontrak dan pada `lib/ipc.ts`/`lib/registry.ts`.

## Path yang dimiliki

Hanya path berikut yang boleh disentuh epic ini:
- `internal/mention/` (`mcp.go`, `tools.go`, `install.go`, `queue.go`, `ipc.go`, `types.go`)
- `cmd/aitermd/wire_mention.go`
- `cmd/aitermctl/cmd_mention.go`
- `frontend/src/modules/mention/`

## Wiring

- Daemon: `cmd/aitermd/wire_mention.go` mendaftar modul `mention` lewat `RegisterModule` (init registry, tanpa edit `modules.go`). Di dalamnya: bangun `Queue`, daftarkan method IPC, jalankan server MCP (`mention.Start`), dan daftarkan `LaunchContribution` ke `internal/provider`. Sesi yang dibuat setelah daemon start otomatis mendapat konfigurasi MCP lewat kontribusi itu.
- Urutan inisialisasi: modul `mention` memakai `provider`, `status`, `session`, dan `workspace`; ambil dependensi dari `Deps` saat callback `RegisterModule` dipanggil dan jangan berasumsi urutan antar-init; bila suatu dependensi belum tersedia di `Deps`, kembalikan error jelas dan jangan panic.
- CLI: `cmd/aitermctl/cmd_mention.go` mendaftar perintah `mention` lewat `RegisterCommand` (tanpa edit `registry.go`).
- Frontend: `modules/mention/index.ts` ditemukan otomatis oleh `import.meta.glob` (tiket 01) dan mendaftar panel `sidebar` dan `statusbar` yang dirender oleh ui-shell (tiket 06); tidak ada perubahan di `App.svelte`.
- Adapter provider yang menerjemahkan `MCPServers` ke format provider (tiket 05) sudah menjadi tanggung jawab tiket 05. Bila adapter belum menerjemahkan `MCPServers["aiterm-mention"]` untuk suatu provider, laporkan sebagai kekurangan di tiket 05; jangan edit `internal/provider`. Mention dari user lewat UI tetap berfungsi karena hanya memakai `provider.SendInput`.
- Verifikasi manual yang disarankan setelah semua fitur digabung (bukan guard otomatis): dua sesi shell dalam satu workspace, `aitermctl mention send <nama> "halo"` terkirim saat idle dan muncul di terminal tujuan; `@nama` dari UI melakukan hal yang sama; panggilan `tools/call` ke server MCP dengan token sesi mengembalikan daftar sesi.

## Guard

Cukup build berhasil, tanpa unit test:
- `go build ./...` (mencakup `internal/mention/`, `cmd/aitermd/wire_mention.go`, `cmd/aitermctl/cmd_mention.go`).
- Build frontend (`vite build`, termasuk type check bila dikonfigurasi di tiket 01).
- Seluruhnya lewat `make build`; harus lolos setelah tiap fitur digabung, termasuk bila hanya sebagian fitur epic ini yang sudah ada (tiap fitur hanya mengimpor yang sudah ada di Kontrak atau di tiket 01 sampai 08).
