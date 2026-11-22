# Plan: Wiring rail Lark dan ctl sources

## Metadata
- Tiket: `lark-connector-ticket-02-lark-sources`
- Fitur: `lark-sources-wiring` (WIRING, dikerjakan TERAKHIR, setelah `lark-config-ipc`, `lark-link-parser`, `lark-pane-foundation`, `lark-panel-ui` selesai)
- ownedPaths:
  - `cmd/shikigamictl/cmd_lark_sources.go` (baru)
  - `frontend/src/modules/lark/index.ts` (hanya penambahan rail item dan impornya)
  - `cmd/shikigamid/modules.go` (hanya bila ternyata perlu; default TIDAK diubah)
- Depends on foundation: ya (`00-foundation.md`: tipe Lark, stub Store, `workspace.store`, tipe Leaf). Juga bergantung pada hasil keempat fitur paralel dan pada tiket 01 (`registerLarkSub`) dan `main-rail-ticket-01-main-rail` (`registerRailItem`).

## Tujuan
Menyambungkan hasil fitur lain: mendaftarkan item rail **Lark** (memakai `LarkPanel` dan `LarkIcon` milik `lark-panel-ui`), menyediakan `shikigamictl lark sources get|add|remove|rename` sebagai alat debug, lalu memverifikasi seluruh epic (build Go, build frontend, aplikasi dijalankan, checklist overview bagian 5). Wiring hanya menyambung dan memverifikasi; kekurangan fitur lain dilaporkan.

## Path yang dimiliki
HANYA ini yang boleh disentuh:
- `cmd/shikigamictl/cmd_lark_sources.go` (baru)
- `frontend/src/modules/lark/index.ts`: tambah rail item saja; jangan hapus atau ubah registrasi pane dan ekspor `resolveSource` milik `lark-pane-foundation`
- `cmd/shikigamid/modules.go`: hanya bila build membuktikan perlu (lihat langkah 2)

Jangan sentuh: `cmd/shikigamictl/cmd_lark.go` (tiket 01), `internal/*`, `wire_lark*.go`, file frontend lark lain, `frontend/src/main.ts`, `shell/*`, `layout/*`. Working tree punya perubahan tak terkait (`app/*`, `internal/provider/*`, dll); jangan disentuh atau di-stage. Jangan `pkill` luas; matikan hanya proses yang diluncurkan sendiri. Jangan menjalankan perintah tulis `lark-cli`.

## Kontrak
Dipakai (rincian di `overview.md` bagian 2.5, 2.6, 2.7):
- Frontend: `registerRailItem`, `RailItemSpec` dari `../shell/registry` (`visible?: Readable<boolean>`); `LarkPanel` dari `./LarkPanel.svelte`; `LarkIcon` dari `./LarkIcon.svelte`; store `connectors` (`Writable<ConnectorInfo[]>`) dan `loadConnectors()` dari `../settings/connectors/store`. `ConnectorInfo = { id, name, status: { state, ... } }` (`../settings/connectors/client`).
- Go ctl: `registerLarkSub(name string, fn larkSubFunc)` dari `cmd_lark.go` (tiket 01), `type larkSubFunc func(ctx context.Context, c *ipc.Client, args []string) error`; `callAndPrint(c, method, params)` dan `printJSON(v)` dari `registry.go`; `*ipc.Client.Call(method, params, &out)`.
- IPC daemon: `lark.sources.get|add|remove|rename`, `lark.link.parse` (bentuk persis di overview 2.5).

Disediakan:
- Item rail `{ id: 'lark', title: 'Lark', icon: LarkIcon, component: LarkPanel, order: 10, visible }`.
- Subcommand `shikigamictl lark sources get|add|remove|rename` (flag di langkah 3).

## Langkah

### 0. Pra-pemeriksaan (jangan membuat pengganti sendiri)
Jalankan `ls cmd/shikigamictl/cmd_lark.go frontend/src/modules/lark/ frontend/src/modules/lark/index.ts` dan `grep -n "registerRailItem" frontend/src/modules/shell/registry.ts`; `grep -n "func registerLarkSub" cmd/shikigamictl/`.
- Bila `registerLarkSub` tidak ada (tiket 01 belum merge): JANGAN membuat dispatcher `lark` sendiri dan jangan menulis `cmd_lark_sources.go` (tidak akan terkompilasi). Kerjakan bagian frontend, lalu laporkan sebagai blokir (`ok:false`, alasan "tiket 01 belum merge: registerLarkSub tidak ada").
- Bila `registerRailItem` tidak ada di `shell/registry.ts` (main-rail belum merge): laporkan sebagai blokir yang sama; jangan menambah ke registry shell.
- Pastikan semua berkas fitur lain ada: `LarkPanel.svelte`, `LarkIcon.svelte` (default export), dan `index.ts` dari `lark-pane-foundation`. Bila `index.ts` belum ada, baca ulang kondisi dan laporkan; jangan membuat registrasi pane sendiri.

### 1. Rail item di `frontend/src/modules/lark/index.ts`
Baca berkas dulu, lalu tambah (Edit, bukan menimpa) di samping registrasi pane:
```ts
import { derived, type Readable } from 'svelte/store';
import { registerRailItem } from '../shell/registry';
import { connectors, loadConnectors } from '../settings/connectors/store';
import LarkPanel from './LarkPanel.svelte';
import LarkIcon from './LarkIcon.svelte';

// true bila connector lark ada dan state bukan cli_missing; true bila daftar belum dimuat.
const larkRailVisible: Readable<boolean> = derived(connectors, ($list) => {
  if ($list.length === 0) return true;
  const lark = $list.find((c) => c.id === 'lark');
  if (!lark) return false;           // daftar dimuat tapi tanpa connector lark
  return lark.status?.state !== 'cli_missing';
});

registerRailItem({ id: 'lark', title: 'Lark', icon: LarkIcon, component: LarkPanel, order: 10, visible: larkRailVisible });
void loadConnectors();
```
Keputusan yang harus dipegang:
- Overview 2.6: true bila lark ada dan bukan `cli_missing`; daftar kosong (belum dimuat) true; daftar terisi tanpa `lark` false.
- `void loadConnectors()` dipanggil sekali di level modul agar store terisi tanpa membuka Pengaturan (fungsi sudah dijaga `seq`, aman dipanggil ulang; galat ditelan ke `connectorsError`, rail tetap `true`). Bila ternyata store sudah dimuat oleh modul lain saat startup (`grep -rn "loadConnectors" frontend/src`), hapus baris ini agar tidak ganda.
- Gabungkan impor dengan impor yang sudah ada (jangan duplikat `derived`/`Readable`); jangan membuat siklus: `index.ts` mengimpor `LarkPanel`, komponen lark TIDAK boleh mengimpor `./index` (cek `grep -rn "from '\.\./index'\|from '\./index'" frontend/src/modules/lark`; `source.ts` sudah dipisah untuk alasan itu).
- Jangan mengubah `frontend/src/main.ts` (`import.meta.glob('./modules/*/index.ts')` sudah memuat modul).

### 2. Konfirmasi daemon tanpa mengubah `modules.go`
- `grep -n "RegisterModule(\"lark\"" cmd/shikigamid/wire_lark.go` dan `grep -n "RegisterModule(\"lark-link\"" cmd/shikigamid/wire_lark_link.go`; keduanya harus di `init()`. Pastikan tidak ada `RegisterModule` dengan nama ganda: `grep -rn "RegisterModule(" cmd/shikigamid | grep -i lark`.
- Bila keduanya terdaftar via `init()`, `modules.go` TIDAK diubah. Hanya bila `go build` atau pembacaan `modules.go` menunjukkan daftar modul eksplisit yang butuh baris tambahan, tambahkan baris minimal itu dan catat alasannya di laporan.

### 3. `cmd/shikigamictl/cmd_lark_sources.go`
`package main`. Impor: `context`, `errors`, `flag`, `fmt`, `io`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`.
```go
const larkSourcesUsage = `penggunaan:
  shikigamictl lark sources get    --workspace <id>
  shikigamictl lark sources add    --workspace <id> --kind ticket|wiki --link <url> [--name <n>]
  shikigamictl lark sources remove --workspace <id> --source <sid>
  shikigamictl lark sources rename --workspace <id> --source <sid> --name <n>`

func init() { registerLarkSub("sources", runLarkSources) }

func runLarkSources(ctx context.Context, c *ipc.Client, args []string) error
```
Perilaku:
- `len(args)==0` atau aksi tak dikenal: `errors.New(larkSourcesUsage)`.
- Aksi pertama = `args[0]`; sisanya diurai dengan `flag.NewFlagSet("lark sources <aksi>", flag.ContinueOnError)`, `fs.SetOutput(io.Discard)`; galat parse dibungkus dengan usage. `--workspace` wajib (kosong -> `fmt.Errorf("--workspace wajib\n%s", larkSourcesUsage)`); idem `--source`/`--name`/`--link`/`--kind` sesuai aksi. `--name` pada `rename` tidak boleh kosong setelah `strings.TrimSpace`.
- `get`: `callAndPrint(c, "lark.sources.get", map[string]any{"workspaceId": ws})`.
- `remove`: `callAndPrint(c, "lark.sources.remove", map[string]any{"workspaceId": ws, "sourceId": sid})`.
- `rename`: `callAndPrint(c, "lark.sources.rename", map[string]any{"workspaceId": ws, "sourceId": sid, "name": name})`.
- `add`:
  1. `--kind` harus `ticket` atau `wiki`, selain itu galat jelas.
  2. `var p larkParsedLink` (struct lokal berfile ini, JANGAN impor `larkdata`): `Kind, Host, BaseToken, TableID, ViewID, SpaceID, NodeToken, DefaultName` dengan tag json camelCase sesuai overview 2.5. Panggil `c.Call("lark.link.parse", map[string]any{"link": link}, &p)`.
  3. Cek kind cocok: `ticket` butuh `p.Kind=="base"`, `wiki` butuh `p.Kind=="wiki"`. Bila tidak, galat Indonesia jelas, mis. `link ini adalah <p.Kind>, bukan <base|wiki>; gunakan --kind <benar>`.
  4. `name` = `--name` (trim) bila tidak kosong, selain itu `p.DefaultName`; bila masih kosong galat `nama kosong; berikan --name`.
  5. Params ticket: `{"workspaceId","kind":"ticket","name","baseToken":p.BaseToken,"host":p.Host}` ditambah `tableId`/`viewId` hanya bila tidak kosong. Params wiki: `{"workspaceId","kind":"wiki","name","spaceId":p.SpaceID,"host":p.Host}` ditambah `"rootToken": p.NodeToken` hanya bila tidak kosong.
  6. `callAndPrint(c, "lark.sources.add", params)`.
- Galat IPC dari `c.Call` dikembalikan apa adanya (pesan sudah berawalan `[kode]` per overview 2.4); tidak perlu dipetakan. Keluaran selalu JSON lewat `callAndPrint`/`printJSON`.
- Jangan mengubah `cmd_lark.go`; tidak ada `RegisterCommand("lark")` di berkas ini.

### 4. Verifikasi build
- `go build ./...` dari root repo harus lulus. `go vet ./cmd/shikigamictl/` bila cepat.
- `cd frontend && npm run build` dan `npm run check`: tidak ada galat baru dibanding sebelum perubahan (catat galat lama bila ada).
- `git diff --stat` hanya menunjuk path wiring ini (ditambah perubahan fitur lain dan perubahan tak terkait yang sudah ada sebelumnya).

### 5. Jalankan aplikasi dan periksa (skill `run`)
Jalankan aplikasi (skill `run`; catat PID yang diluncurkan sendiri untuk dimatikan di akhir). Periksa seluruh Wiring checklist overview bagian 5 butir 5:
1. Rail memuat item Lark.
2. Panel menampilkan Tiket dan Wiki, Pribadi paling atas.
3. Tempel link Base valid, nama default terisi, simpan, Base muncul; klik membuka pane `lark-board` placeholder.
4. Tambah wiki dari link node, muncul; klik membuka `lark-doc` placeholder.
5. Ganti nama dan hapus bekerja; ganti workspace mengganti isi panel.
6. Restart: sumber dan pane terbuka tetap ada.
7. Hapus Base lalu pane terbuka menampilkan "Sumber tidak ada di workspace ini".
8. Link tak valid menampilkan galat inline.
9. Tanpa login `lark-cli`, panel menampilkan ajakan ke Pengaturan > Connector.
10. `shikigamictl lark sources get --workspace <id>` sama dengan isi panel; perubahan lewat ctl (`add`, `rename`, `remove`) terlihat di panel tanpa muat ulang.
Butir yang butuh login Lark nyata atau link nyata yang tidak tersedia: tandai "tidak dapat diverifikasi" dengan alasan, jangan dikarang. Bila `docs/lark/cli-findings.md` memicu aturan pemotongan (overview 5 butir 6), verifikasi bahwa perilaku pemotongan muncul, bukan memperbaikinya.
Perbaikan lintas fitur hanya boleh bila memblokir sambungan dan kecil; catat tiap perbaikan di laporan. Kalau tidak, laporkan temuan.
Matikan hanya proses yang diluncurkan sendiri.

## Perubahan
- Baru: `cmd/shikigamictl/cmd_lark_sources.go`
- Diubah: `frontend/src/modules/lark/index.ts` (impor + `larkRailVisible` + `registerRailItem` + `loadConnectors`)
- Tidak diubah (dikonfirmasi): `cmd/shikigamid/modules.go`, `frontend/src/main.ts`, `cmd/shikigamictl/cmd_lark.go`

## Wiring
Fitur ini adalah langkah wiring itu sendiri. Ia tidak mengekspos apa pun untuk disambungkan lagi. Yang diharapkan sudah tersedia dari fitur lain: `LarkPanel`/`LarkIcon` (lark-panel-ui), registrasi pane dan `index.ts` (lark-pane-foundation), IPC `lark.*` lewat `init()` (lark-config-ipc, lark-link-parser), `registerLarkSub` (tiket 01), `registerRailItem` (main-rail).

## Selesai bila
- Item rail `lark` terdaftar (order 10, judul "Lark") dan `visible` mengikuti store connector sesuai aturan di langkah 1; tidak ada siklus impor; registrasi pane di `index.ts` utuh.
- `shikigamictl lark sources get|add|remove|rename` terdaftar lewat `registerLarkSub("sources", ...)`, `cmd_lark.go` tidak berubah, keluaran JSON.
- Guard: `go build ./...` lulus; `cd frontend && npm run build` lulus dan `npm run check` tanpa galat baru. Tanpa unit test.
- Aplikasi dijalankan dan butir checklist overview 5 diperiksa; hasil per butir (lulus, gagal, tidak dapat diverifikasi dan alasannya) dicatat di laporan akhir. Blokir dari prasyarat (tiket 01 atau main-rail belum merge) dilaporkan, bukan ditambal.
