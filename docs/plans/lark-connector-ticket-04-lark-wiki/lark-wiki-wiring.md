# Plan: Wiring wiki dan ctl

## Metadata
- Tiket: `lark-connector-ticket-04-lark-wiki` (`docs/tickets/lark-connector-ticket-04-lark-wiki.md`)
- Fitur: `lark-wiki-wiring` (WIRING, dikerjakan TERAKHIR, setelah `lark-wiki-data`, `lark-wiki-tree-ui`, `lark-doc-pane` selesai dan merge)
- ownedPaths: `cmd/shikigamictl/cmd_lark_wiki.go` (baru)
- Depends on foundation: ya (`00-foundation.md`: skeleton `internal/larkdata/wiki.go` dan `doc.go`). Juga bergantung pada hasil tiga fitur paralel, tiket 01 (`registerLarkSub` di `cmd_lark.go`), dan tiket 02 (`WikiSection`, placeholder yang kini ditimpa).

Baca `overview.md` (terutama bagian 1.3, 1.6, 1.7, 2.2, 2.5, 5) sebelum mulai. Plan ini tidak mengulang isinya.

## Tujuan
Menyediakan alat debug `shikigamictl lark wiki list|get` yang memanggil IPC `lark.wiki.list` dan `lark.wiki.get`, lalu memverifikasi seluruh epic tiket 04 tersambung: `WikiSection` merender `WikiTree` baru, `onOpen` membuka pane `lark-doc`, `wire_lark_wiki.go` terdaftar di daemon, `go build ./...` dan build frontend lulus, dan alur akhir berjalan di aplikasi sesuai checklist overview bagian 5. Wiring hanya menyambung dan memverifikasi; kekurangan fitur lain dilaporkan, bukan diperbaiki (kecuali perbaikan kecil lintas fitur yang memblokir, dicatat).

## Path yang dimiliki
HANYA berkas ini yang boleh dibuat atau diubah:
- `cmd/shikigamictl/cmd_lark_wiki.go` (baru)

Jangan sentuh: `cmd/shikigamictl/cmd_lark.go` (tiket 01), `cmd_lark_sources.go` dan `cmd_lark_tickets.go` (tiket 02/03), `internal/larkdata/*`, `cmd/shikigamid/*` (termasuk `wire_lark_wiki.go` dan `modules.go`), `frontend/*` (termasuk `WikiSection.svelte`, `WikiTree.svelte`, `wikiTreeClient.ts`, `doc/*`, `package.json`), `overview.md`, dan berkas plan lain. Working tree dapat memuat perubahan tak terkait; jangan di-stage. Jangan `pkill` luas: matikan hanya proses yang Anda luncurkan sendiri (PID sendiri). Jangan menjalankan perintah tulis `lark-cli`.

## Kontrak
Disediakan:
- Subcommand ctl `shikigamictl lark wiki`, didaftarkan lewat `registerLarkSub("wiki", larkWiki)` dari `init()` (kontrak overview 2.5). `shikigamictl lark` otomatis menampilkan `wiki` di bantuan karena `larkUsage()` membaca peta `larkSubcommands`.
  - `lark wiki list --workspace <id> --source <id|personal> [--parent <token>]`
  - `lark wiki get --workspace <id> --source <id|personal> --node <token>`

Dipakai (jangan diubah):
- `cmd_lark.go`: `type larkSubFunc func(ctx context.Context, c *ipc.Client, args []string) error`, `registerLarkSub(name string, fn larkSubFunc)`, `larkUsage()`.
- `registry.go`: `callAndPrint(c *ipc.Client, method string, params any) error`, `printJSON(v any) error`. Pola flag: `flag.NewFlagSet(name, flag.ContinueOnError)` + `fs.SetOutput(io.Discard)` (lihat `cmd_grimoire.go`); `c.Call(method, params, &out)` dengan `out` bertipe `map[string]any` atau struct.
- IPC (overview 2.2): `lark.wiki.list` params `{workspaceId, sourceId, parentToken?}` -> `{nodes: [{token, title, objType, objToken, hasChild}], hasMore?, hidden?, hiddenReason?}`; `lark.wiki.get` params `{workspaceId, sourceId, nodeToken}` -> `{title, objType, url, markdown?, cells?, truncated?}`. Galat IPC berformat `"[<code>] <pesan>"`; ctl cukup meneruskan galat apa adanya (tanpa mengurai).
- Pemeriksaan wiring memakai kontrak frontend overview 2.4 dan checklist bagian 5.

## Langkah

### W0. Prasyarat
1. Pastikan tiket 01, 02, dan tiga fitur tiket 04 sudah merge: ada `cmd/shikigamictl/cmd_lark.go` dengan `registerLarkSub`, `cmd/shikigamid/wire_lark_wiki.go`, `internal/larkdata/wiki.go` dan `doc.go` berisi implementasi (bukan stub "belum diimplementasi"), `frontend/src/modules/lark/sections/WikiTree.svelte`, `wikiTreeClient.ts`, `doc/DocPane.svelte`, `docClient.ts`. Bila ada yang belum ada, hentikan dan laporkan; jangan membuatnya.
2. Bila `registerLarkSub` tidak ada, laporkan; jangan membuat dispatcher sendiri.
3. Baca `docs/lark/cli-findings.md` untuk aturan potong spike (langkah W6).

### W1. Buat `cmd/shikigamictl/cmd_lark_wiki.go`
`package main`. Impor: `context`, `errors`, `flag`, `fmt`, `io`, `os`, `strings` (hanya yang dipakai), dan `github.com/NooBeeID/shikigami/internal/ipc`. Semua identifier lokal berawalan `larkWiki` agar tidak bentrok di paket `main`.

Isi:
- `const larkWikiUsage = "penggunaan: shikigamictl lark wiki list --workspace <id> --source <id|personal> [--parent <token>] | get --workspace <id> --source <id|personal> --node <token>"`.
- `func larkWiki(ctx context.Context, c *ipc.Client, args []string) error`: bila `len(args) == 0` atau `args[0]` bukan `list`/`get`, kembalikan `errors.New(larkWikiUsage)`. Buat `fs := flag.NewFlagSet("lark wiki "+sub, flag.ContinueOnError)`, `fs.SetOutput(io.Discard)`, definisikan `--workspace`, `--source`, dan (list) `--parent`, (get) `--node`. `fs.Parse(args[1:])` gagal atau `fs.NArg() > 0` -> `errors.New(larkWikiUsage)`.
- Validasi: `--workspace` dan `--source` wajib tidak kosong (galat Indonesia: `"--workspace wajib diisi"`, `"--source wajib diisi (id sumber wiki atau personal)"`); untuk `get`, `--node` wajib (`"--node wajib diisi"`); `--parent` hanya valid untuk `list`, `--node` hanya untuk `get` (flag yang tidak didefinisikan di sub lain otomatis memicu usage). Nilai `--source` diteruskan apa adanya (`personal` adalah literal).
- `list`: params `map[string]any{"workspaceId": ws, "sourceId": src}`; tambahkan `"parentToken": parent` hanya bila tidak kosong. Panggil `c.Call("lark.wiki.list", params, &out)` dengan `out` struct lokal `larkWikiListResult{Nodes []struct{Token, Title, ObjType, ObjToken string; HasChild bool} ; HasMore, Hidden bool; HiddenReason string}` dengan tag json camelCase sesuai overview 2.2 (`token`, `title`, `objType`, `objToken`, `hasChild`, `hasMore`, `hidden`, `hiddenReason`). Cetak ke stdout:
  - bila `Hidden`: satu baris `tersembunyi: <hiddenReason>` (tanpa baris node);
  - selain itu satu baris per node `token<TAB>objType<TAB>hasChild<TAB>title` (hasChild sebagai `true`/`false`); bila tak ada node cetak `(kosong)`;
  - bila `HasMore`: baris akhir `hasMore: true (menampilkan 50 pertama)`.
- `get`: params `{"workspaceId", "sourceId", "nodeToken"}`; `c.Call("lark.wiki.get", params, &out)` dengan `larkWikiGetResult{Title, ObjType, URL string; Markdown *string; Cells [][]string; Truncated bool}` (tag `title`, `objType`, `url`, `markdown`, `cells`, `truncated`). Cetak `title: ...`, `objType: ...`, `url: ...` (bila `url` kosong cetak `url: (kosong, host belum diketahui)`), lalu:
  - `Markdown != nil`: baris kosong lalu isi markdown apa adanya;
  - `Cells != nil` (sheet): baris kosong lalu TSV (sel dipisah TAB, satu baris per baris sel; ganti TAB dan newline dalam sel dengan spasi), lalu bila `Truncated` baris `truncated: true (rentang awal 50 baris x 20 kolom)`.
- `func init() { registerLarkWiki() }` TIDAK diperlukan; cukup `func init() { registerLarkSub("wiki", larkWiki) }`. Jangan memanggil `RegisterCommand` (perintah `lark` sudah didaftarkan `cmd_lark.go`).
- Galat dari `c.Call` dikembalikan apa adanya (`return err`) sehingga format `[code] pesan` terlihat di terminal. Parameter `ctx` boleh tak dipakai (beri nama `_` bila perlu agar lolos vet).

### W2. Build Go
`go build ./...` dari root repo harus lulus. Jalankan juga `go vet ./cmd/shikigamictl/` bila cepat. Pastikan `go build` tidak panik karena `registerLarkSub("wiki", ...)` terdaftar dua kali (cari `"wiki"` di `cmd/shikigamictl/`; bila sudah ada pendaftar lain, laporkan, jangan menimpa).

### W3. Verifikasi daemon
1. Konfirmasi `cmd/shikigamid/wire_lark_wiki.go` memanggil `RegisterModule("lark-wiki", ...)` dari `init()`, nama modul unik (cari `RegisterModule(` di `cmd/shikigamid/`), dan `modules.go` tidak berubah di `git diff`.
2. Jalankan daemon/app (skill `run`; lihat W5) lalu uji lewat ctl, tanpa menulis ke Lark:
   - `shikigamictl lark wiki list --workspace <wsId> --source personal` mencetak node akar `my_library` atau baris `tersembunyi: ...`.
   - `shikigamictl lark wiki list --workspace <wsId> --source personal --parent <token>` mencetak anak satu level.
   - `shikigamictl lark wiki get --workspace <wsId> --source personal --node <docxToken>` mencetak judul, tipe, url, dan markdown; ulangi untuk satu sheet (TSV) dan satu node tipe lain (hanya judul/tipe/url).
   - Sumber tak dikenal: `--source tidak-ada` harus menghasilkan `[source_not_found] Sumber tidak ada di workspace ini`, bukan panic. `--node` kosong ditolak ctl.
   - `shikigamictl lark` (tanpa argumen) menampilkan `wiki` di daftar penggunaan.
   - Ambil `wsId` lewat `shikigamictl workspace ...` atau `shikigamictl lark sources get --workspace <id>` (tiket 02).

### W4. Verifikasi frontend (baca saja, JANGAN diubah)
1. Buka `frontend/src/modules/lark/sections/WikiSection.svelte`: pastikan merender `WikiTree` dengan props `{source, onOpen}` dan `onOpen(nodeToken)` memanggil `layout.openPane({ kind: 'lark-doc', sourceId, nodeToken, title })` (Pribadi: `sourceId: 'personal'`). Bila bentuk berbeda atau tidak membuka pane, laporkan sebagai temuan (jangan diubah).
2. Pastikan tidak ada siklus impor (overview 5 butir 4): `DocPane` -> `doc/MarkdownView` -> `files/viewer/MarkdownView`; `DocPane` -> `docClient`; `WikiTree` -> `wikiTreeClient`; klien tidak mengimpor komponen. `wikiTreeClient.ts` dan `docClient.ts` tidak mengimpor `board/boardClient.ts` maupun `client.ts`.
3. `cd frontend && npm run build` lulus dan `npm run check` tanpa galat baru dibanding sebelum tiket 04 (catat jumlah galat bila sudah ada galat lama).
4. Pastikan `frontend/package.json` hanya berubah sesuai keputusan fitur `lark-doc-pane` (overview 1.1: tanpa dependensi baru kecuali sudah diputuskan di sana); laporkan bila ada dependensi tak terduga.

### W5. Verifikasi manual di aplikasi
Jalankan aplikasi dengan skill `run` (jangan `pkill` luas; catat PID yang Anda luncurkan dan matikan hanya itu). Jalankan seluruh checklist overview bagian 5 butir 5, berurutan:
1. Panel Lark bagian Wiki menampilkan Pribadi di paling atas, lalu wiki tim.
2. Buka pohon beberapa level: memuat per level, keadaan kosong "Belum ada isi", tombol segarkan; catatan "Menampilkan 50 pertama" bila `hasMore`.
3. Buka satu docx: markdown ter-render, tautan membuka browser. Buka satu sheet: tabel sel dengan catatan batas bila terpotong. Buka node tipe lain: judul + "Buka di Lark".
4. Salin link: clipboard terisi. Kirim ke agen dengan sesi terminal aktif: link tertempel tanpa Enter; tanpa sesi aktif: toast "Tidak ada sesi aktif". Bila `url` kosong (host tidak diketahui), kedua tombol nonaktif dengan keterangan overview 1.3.
5. Restart aplikasi: pane `lark-doc` dipulihkan dan isi diambil ulang.
6. Hapus sumber wiki lewat panel (atau `shikigamictl lark sources remove ...`) lalu pane yang terbuka menampilkan "Sumber tidak ada di workspace ini".
7. Jaringan gagal (matikan jaringan sementara): galat dengan tombol coba lagi, tidak crash.
8. Ganti workspace: pohon lama dikosongkan.
9. `shikigamictl lark wiki list --source personal` dan `lark wiki get` mencetak hasil terbaca (sudah di W3).
Catat hasil tiap butir (lulus/gagal/tidak bisa diuji beserta alasan, mis. `lark-cli` belum login). Bila `lark-cli` tidak login atau tidak ada, uji sebatas keadaan galat (ajakan ke Pengaturan > Connector) dan laporkan butir data yang tidak bisa diverifikasi.

### W6. Aturan potong spike
Baca `docs/lark/cli-findings.md` dan terapkan (overview bagian 5 butir 6):
- `my_library` tidak bisa di-list: Pribadi menampilkan keterangan `hiddenReason` (jalur `hidden` sudah ada di desain, overview 1.7). Verifikasi saja; jangan menambah kode.
- `sheets +cells-get` memakai flag berbeda dari yang diasumsikan `FetchSheetCells` dan membuat sheet tidak bisa dibaca: sheet cukup judul + link; laporkan sebagai temuan untuk `lark-wiki-data` (potong `FetchSheetCells`). Anda tidak mengedit `internal/larkdata/doc.go`, kecuali perbaikan kecil lintas fitur yang benar-benar memblokir dan dicatat.
Wiring hanya memverifikasi dan melaporkan. Perbaikan kecil lintas fitur hanya diizinkan bila memblokir sambungan, minimal, dan dicatat eksplisit di laporan (berkas dan alasan).

### W7. Pemeriksaan akhir
- `git status` hanya menampilkan `cmd/shikigamictl/cmd_lark_wiki.go` sebagai berkas baru dari plan ini (plus perubahan lintas fitur yang dicatat, bila ada).
- Jangan meng-commit perubahan tak terkait.

## Perubahan
- Baru: `cmd/shikigamictl/cmd_lark_wiki.go` (subcommand `lark wiki list|get`).
- Tidak ada berkas lain yang diubah. Verifikasi (W3 sampai W6) bersifat baca-saja; temuan dilaporkan di keluaran akhir.

## Wiring
Fitur ini adalah langkah wiring itu sendiri, jadi tidak ada yang diekspos untuk disambungkan kemudian. Yang disambungkan di sini: pendaftaran `registerLarkSub("wiki", larkWiki)` di `init()` (tanpa mengubah `cmd_lark.go`) ke IPC `lark.wiki.list` dan `lark.wiki.get` milik `wire_lark_wiki.go`. Temuan yang harus dilaporkan: selisih nama kode tiket 02 vs overview, bentuk `onOpen` di `WikiSection`, hasil checklist W5, penerapan aturan potong spike W6, dan perubahan lintas fitur bila ada.

## Selesai bila
Guard: `go build ./...` dari root repo lulus (path fitur ini: `cmd/shikigamictl`), tanpa unit test.
- `cmd/shikigamictl/cmd_lark_wiki.go` ada, `registerLarkSub("wiki", ...)` terdaftar sekali, `cmd_lark.go` tidak berubah.
- `shikigamictl lark` menampilkan `wiki`; `lark wiki list` dan `lark wiki get` memanggil IPC yang benar, mencetak format di W1, dan meneruskan galat `[code] pesan` (termasuk `[source_not_found] ...`).
- `go build ./...`, `cd frontend && npm run build` lulus; `npm run check` tanpa galat baru.
- `WikiSection` terverifikasi merender `WikiTree` dan `onOpen` membuka pane `lark-doc` (atau temuan dilaporkan); `wire_lark_wiki.go` terdaftar dengan nama modul unik dan `modules.go` tidak berubah.
- Checklist aplikasi W5 dijalankan dan hasilnya (termasuk butir yang tidak bisa diuji) dilaporkan; aturan potong spike W6 diterapkan/dilaporkan.
- Tidak ada berkas di luar ownedPaths yang berubah, kecuali perbaikan lintas fitur kecil yang dicatat.
