# Overview: thread-ticket-03-thread-activity

Tiket: `docs/tickets/thread-ticket-03-thread-activity.md`. Spec: `docs/superpowers/specs/2026-11-22-thread-design.md`. Dependensi: `thread-ticket-01-thread-core` (sudah ada di kode: `internal/thread/`, `frontend/src/modules/thread/`) dan `thread-ticket-02-thread-agents` (plan: `docs/plans/thread-ticket-02-thread-agents/overview.md`, disebut "overview-02"; kodenya BELUM tentu ada saat epic ini dikerjakan, jadi epic ini tidak boleh bergantung pada simbolnya).

Dokumen ini adalah sumber kebenaran BERSAMA untuk semua plan fitur epic ini. Bila ada beda dengan tiket, overview menang hanya pada butir berlabel "koreksi/keputusan" (bagian 2.2); selain itu tiket menang. Tanpa dependency baru, tanpa unit test; guard hanya build.

Nama persis yang dipakai di sini sudah DIBACA dari kode tiket 01 yang ada (`manager.go`, `store.go`, `types.go`, frontend `thread-store.ts`, `MessageList.svelte`, `MessageItem.svelte`, `format.ts`). Bila saat implementasi kode berbeda, ikuti kode dan catat di "Catatan untuk wiring" plan fitur.

## 1. Ringkasan epic dan urutan eksekusi

Epic menurunkan pesan `kind=activity` dari hook tool call sesi peserta thread (`status.Engine.AddHookListener`, event `PostToolUse`), menggabungkannya per agent menjadi SATU pesan (id sama, `count`/`items`), menyimpannya ke log thread, lalu menampilkannya sebagai kartu aktivitas buka-tutup per agent di aliran chat. Pesan diskusi tidak pernah dilipat.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): field opsional `count`/`items` pada `ThreadMessage` (Go + TS); primitif log `Store.UpsertMessage` dan `Manager.UpsertMessage` (ganti-di-tempat, TANPA memicu hook `OnMessage`); `Manager.ActiveThreads()`; stub `activity.go` yang berkompilasi. `go build ./...` dan build frontend lulus.
2. **Fitur paralel** (2 fitur, bagian 3): `thread-activity-daemon`, `thread-activity-ui`. Kontrak bagian 2 tidak berubah.
3. **Wiring** (bagian 5): satu baris `InstallActivity` di `cmd/shikigamid/wire_thread.go` dan satu cabang render di `frontend/src/modules/thread/MessageList.svelte`.

## 2. Kontrak bersama

### 2.1 Paket, folder, penamaan

- Go: paket `internal/thread` (package `thread`). Berkas epic ini: `activity_support.go` (Foundation, tetap), `activity.go` (stub Foundation, diganti penuh oleh `thread-activity-daemon`).
- Frontend: `frontend/src/modules/thread/activity/` (Svelte 5 runes, TypeScript): `ActivityCard.svelte`, `ActivityFlowItem.svelte`, `groupActivity.ts`, `index.ts` (semua milik `thread-activity-ui`).
- Komentar, log, pesan galat, teks UI: bahasa Indonesia; identifier bahasa Inggris. JSON camelCase. Logger `*slog.Logger` nullable (fallback `slog.Default()`; di Manager sudah ada `m.log`, tidak diekspor, jadi `activity.go` memakai `slog.Default()` kecuali menerima logger lewat fungsi lain).
- Helper tak-diekspor di `activity.go` berawalan `activity` (mis. `activitySummarize`, `activityRelPath`) agar tak bentrok dengan berkas lain di paket.
- Helper di `groupActivity.ts` yang diekspor dipakai komponen kartu; tidak ada ekspor baru yang dipakai di luar folder `activity/` selain yang tercantum di 2.4.

### 2.2 Keputusan desain (WAJIB diikuti)

1. **Log append-only, pembaruan = ganti baris.** `store.go` tiket 01 hanya `AppendMessage` (satu baris JSON per pesan) dan `Messages` TIDAK menghapus duplikat id. Keputusan: Foundation menambah `Store.UpsertMessage(m)` di berkas baru `activity_support.go` (BUKAN mengubah `store.go`): di bawah `storeLock`, bila baris terakhir log ber-id sama diganti, bila id ada di baris lain baris itu diganti (tulis ulang atomik lewat `storeWriteFileAtomic`), bila id belum ada baris baru ditambahkan (append). Hasilnya satu id = satu baris log; `thread.messages` otomatis benar tanpa dedup di pembaca. Perilaku `AppendMessage`/`Messages` tiket 01 tidak berubah.
2. **Tanpa hook `OnMessage` untuk activity.** `Manager.UpsertMessage` mencatat + mempublikasikan `thread.message` tetapi TIDAK memanggil `managerFireMessage`. Dengan begitu pesan activity tidak pernah masuk router/antrean mention tiket 02, tidak dihitung anti-loop (yang membaca `OnMessage`, overview-02 2.2 no. 6), dan tidak diketahui hook lain tiket 04/05 sebagai pesan baru. Catat pilihan ini di commit (butir wiring tiket no. 2).
3. **Penanda pemutus penggabungan = hook `OnMessage`.** Karena activity tidak memicu `OnMessage`, SETIAP panggilan `OnMessage` berarti ada pesan non-activity (message/system) baru di log; daemon menjawabnya dengan membuang "ekor" penggabungan thread itu. Aksi dari agent lain tidak memicu `OnMessage` sehingga daemon sendiri memutus: ekor per thread hanya satu (`tail`); aksi agent lain mengganti ekor (sehingga kartu agent pertama tidak digabung lagi setelahnya).
4. **Kartu dibangun dari log.** UI tidak menyimpan state kartu selain buka/tutup; seluruh isi berasal dari daftar `ThreadMessage` (dimuat lewat `thread.messages`, diperbarui lewat `thread.message`). Store tiket 01 (`mergeMessages`) sudah meng-upsert berdasarkan `id` dan mengurutkan menurut `ts`; TIDAK perlu diubah (wiring hanya memverifikasi).
5. **`ts` pesan activity = waktu aksi TERAKHIR** yang digabung (diperbarui tiap penggabungan). Dipakai UI untuk status "mengerjakan" dan dipakai daemon untuk jendela penggabungan. Posisi di aliran tidak bergeser karena penggabungan hanya terjadi bila tidak ada pesan lain setelahnya.
6. **Pengenalan peserta tanpa membaca disk per event.** Daemon memelihara peta memori `sessionId -> {threadId, name}` dari: seed `Manager.ActiveThreads()` saat `InstallActivity`, hook `OnStart` (seed ulang), `OnParticipantAdded` (tambah), `OnClose` (hapus semua entri thread). Tidak memakai kode tiket 02 (`ThreadIDFromCwd`, token MCP). Nama = `Participant.Name`; bila kosong dan role master -> "master"; bila kosong lainnya -> 8 karakter pertama `sessionId`.
7. **Bentuk `items`: tanpa awalan nama.** `items[i]` = `<Tool> <target ringkas>` (mis. `Edit auth/claims.go`), maks 160 rune, satu baris. `text` = `<nama>: ` + item aksi terakhir, maks 160 rune total (potong target, bukan awalan, bila perlu; selalu satu baris). Kartu menampilkan `items` apa adanya. Untuk pesan activity tanpa `items` (mis. ditulis pihak lain), UI menurunkan satu item dari `text` dengan membuang awalan `<nama>: ` bila ada.
8. **Batas.** `count` = total aksi yang pernah digabung (tidak dibatasi); `items` menyimpan maks 50 aksi TERBARU (urut lama -> baru). Jendela penggabungan 30 detik (selisih waktu antara aksi baru dan `ts` pesan activity ekor). Jendela "mengerjakan" di UI juga 30 detik dari `ts` pesan terakhir grup.

### 2.3 Tipe dan signature bersama

**Go, `types.go` (Foundation menambah field opsional, bentuk lain tidak berubah):**

```go
type ThreadMessage struct {
    ID       string    `json:"id"`
    ThreadID string    `json:"threadId"`
    From     string    `json:"from"`
    To       string    `json:"to"`
    Text     string    `json:"text"`
    TS       time.Time `json:"ts"`
    Kind     string    `json:"kind"`
    Count    int       `json:"count,omitempty"` // hanya kind=activity: jumlah aksi yang digabung
    Items    []string  `json:"items,omitempty"` // hanya kind=activity: ringkasan per aksi (maks 50 terbaru)
}
```

**Go, `activity_support.go` (Foundation, tetap; daemon memakainya):**

```go
// Store
func (s *Store) UpsertMessage(m ThreadMessage) error // ganti baris ber-id sama atau tambah; ID/ThreadID/TS wajib terisi seperti AppendMessage

// Manager
func (m *Manager) UpsertMessage(threadID string, msg ThreadMessage) (ThreadMessage, error)
// Isi default seperti Post (ID bila kosong, TS, Kind, To=all, From=system); tolak thread closed (ErrClosed);
// Store.UpsertMessage; m.managerPublish(EventMessage, msg); TIDAK memanggil managerFireMessage.
func (m *Manager) ActiveThreads() []Thread // thread berstate active, Participants non-nil (managerNormalize)
```

**Go, `activity.go` (stub Foundation; implementasi penuh `thread-activity-daemon`; signature final):**

```go
type ActivitySource interface {
    AddHookListener(fn func(sessionID, event string, payload map[string]any)) (cancel func())
}
const (
    ActivityMergeGap     = 30 * time.Second
    ActivityMaxItems     = 50
    ActivitySummaryMax   = 160 // rune
)
func InstallActivity(m *Manager, src ActivitySource) (stop func())
```

`*status.Engine` memenuhi `ActivitySource`. `InstallActivity` aman dipanggil sebelum atau sesudah `m.FireStart()`; `nil` untuk `m`/`src` -> kembalikan `func(){}` tanpa panic. `stop` melepas listener hook status dan membersihkan state.

**Format `ThreadMessage` activity yang dihasilkan daemon:** `kind="activity"`, `from=<sessionId peserta>`, `to="all"`, `text="<nama>: <Tool> <target>"` (aksi terakhir), `count`, `items` (2.2 no. 7-8), `ts` = waktu aksi terakhir, `id` stabil selama penggabungan.

**Sumber data hook:** event `PostToolUse` saja; `payload["tool_name"]` (string, wajib tak kosong) dan `payload["tool_input"]` (map). Kunci target: Bash `command`; Read/Edit/Write/NotebookEdit `file_path` (NotebookEdit juga `notebook_path`); Grep/Glob `pattern`; WebFetch `url`; Task `description`; lainnya string pertama berurut abjad (pola `status.summarize`). Path absolut di bawah `Store.ThreadDir(threadId)` diubah relatif terhadap worktree (buang `<ThreadDir>/<slug>/`); path absolut lain dibiarkan; semua newline -> spasi.

**TypeScript, `types.ts` (Foundation menambah):**

```ts
export interface ThreadMessage {
  id: string; threadId: string; from: string; to: string; text: string; ts: string; kind: ThreadKind;
  count?: number;   // hanya kind 'activity'
  items?: string[]; // hanya kind 'activity'
}
```

### 2.4 Kontrak frontend (`modules/thread/activity/`, milik `thread-activity-ui`)

Memakai dari tiket 01 (nama sudah ada di kode): `ThreadMessage`, `Thread` dari `../types`; `participantName(thread, id)` dari `../thread-store`; `avatarColor`, `formatTime` dari `../format`.

```ts
// groupActivity.ts
export const ACTIVITY_RUNNING_MS = 30_000;
export interface ActivityGroup {
  type: 'activity';
  key: string;            // id pesan activity pertama di grup (stabil)
  from: string;           // sessionId agent
  count: number;          // jumlah aksi (jumlah count tiap pesan, min 1 tiap pesan)
  items: string[];        // item gabungan, urut lama -> baru (tanpa awalan nama)
  lastTs: string;         // ts pesan terakhir grup
  last: boolean;          // true bila tidak ada item aliran lain sesudah grup ini
  messages: ThreadMessage[];
}
export interface PlainFlowItem { type: 'message'; key: string; message: ThreadMessage }
export type FlowItem = ActivityGroup | PlainFlowItem;
export function groupActivity(messages: ThreadMessage[], names?: (id: string) => string): FlowItem[]
// Fungsi murni. Pesan non-activity -> PlainFlowItem apa adanya (urutan dipertahankan). Pesan activity BERURUTAN
// dengan from sama (tanpa item lain di antaranya) -> satu ActivityGroup. Pesan ber-id sama: versi terakhir yang
// dipakai (dedup di awal). `names` dipakai hanya untuk membuang awalan "<nama>: " pada text bila items kosong.
export function isRunning(group: ActivityGroup, now: number, followedByOwnMessage: boolean, threadClosed: boolean): boolean
```

`ActivityGroup.last` + pesan diskusi dari agent yang sama setelahnya menentukan "selesai" (`followedByOwnMessage` dihitung `groupActivity` -> field tambahan `finished: boolean` di grup; implementasi boleh menaruhnya sebagai field itu asal `ActivityFlowItem` memakainya). Aturan status: `running = !threadClosed && !finished && (now - Date.parse(lastTs)) < ACTIVITY_RUNNING_MS`. `finished` true bila setelah grup ada pesan `kind=message` dengan `from` sama (agent itu bicara berarti berhenti).

```ts
// ActivityCard.svelte  props: { agentName: string; color: string; items: string[]; count: number; running: boolean }
// ActivityFlowItem.svelte props: { thread: Thread; group: ActivityGroup }
//   memakai participantName(thread, group.from), avatarColor(group.from), thread.state === 'closed',
//   menjadwalkan setTimeout satu kali agar status "mengerjakan" -> "selesai" setelah 30 detik tanpa pembaruan.
// index.ts  export { default as ActivityFlowItem } from './ActivityFlowItem.svelte'; export { groupActivity, ACTIVITY_RUNNING_MS } dan tipenya.
```

Tampilan: header tombol `<nama>: N aksi` + status ("mengerjakan"/"selesai") + penanda warna; `aria-expanded`; default tertutup; daftar item satu baris dengan ellipsis dan `title` berisi teks penuh; token tema (`var(--fg, ...)`, `var(--bg-elev, ...)`, `var(--border, ...)`, `var(--fg-muted, ...)` seperti `MessageItem.svelte`); tanpa scroll horizontal (`min-width:0`, `overflow-wrap:anywhere` atau ellipsis). State buka-tutup per kartu di memori komponen; `localStorage` boleh (kunci mis. `thread.activity.open.<key>`, try/catch).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `thread-activity-daemon` | `ActivitySource`, `InstallActivity`: listener hook `PostToolUse`, penyaringan sesi peserta thread aktif (peta memori), ringkasan `<Tool> <target>`, penggabungan beruntun (ekor per thread, jendela 30 dtk, `count`/`items`), simpan lewat `Manager.UpsertMessage` (urutan terjaga lewat antrean/goroutine per thread, tidak memblok hook status, recover panic), `OnMessage` memutus ekor, `OnClose` melepas state, thread closed tidak menerima tulisan. | `internal/thread/activity.go` |
| `thread-activity-ui` | `groupActivity.ts`, `ActivityCard.svelte`, `ActivityFlowItem.svelte`, `index.ts`: kartu buka-tutup per agent, status mengerjakan/selesai, warna agent, aksesibilitas, tema terang/gelap, lebar sempit. Boleh memakai data uji lokal berformat `ThreadMessage`. | `frontend/src/modules/thread/activity/` |

Dependensi antar fitur setelah Foundation: tidak ada; lintas fitur hanya lewat format `ThreadMessage` activity (2.3) dan konstanta 30 detik/50 item/160 rune.

## 4. Foundation

Ada (`00-foundation.md`, hasFoundation = true). Isi ringkas: field `Count`/`Items` di `types.go` dan `types.ts`; `activity_support.go` (`Store.UpsertMessage`, `Manager.UpsertMessage`, `Manager.ActiveThreads`); stub `activity.go` berkompilasi; guard build Go dan frontend. Foundation menyentuh dua berkas tiket 01 hanya secara aditif (`types.go`, `types.ts`), sesuai klausul tiket "boleh ditambah field opsional bila mutlak perlu" (field itu disyaratkan Kontrak tiket).

## 5. Wiring checklist (setelah semua fitur selesai)

Hanya berkas berikut yang boleh disentuh agen wiring:

1. **`cmd/shikigamid/wire_thread.go`** (tiket 01/02; tidak ada titik ekstensi lain, jadi satu baris aditif): di `wireThread`, SETELAH `thread.NewManager(...)`, `d.Provide("thread.manager", m)`, `thread.RegisterIPC(srv, m)` (dan setelah `thread.InstallAgents(...)` bila tiket 02 sudah memasangnya), SEBELUM `m.FireStart()`:
   ```go
   if v, ok := d.Lookup("status.engine"); ok {
       if eng, ok2 := v.(*status.Engine); ok2 && eng != nil {
           thread.InstallActivity(m, eng)
       } else {
           d.Logger.Warn("thread: status.engine bertipe tak terduga, kartu aktivitas dinonaktifkan")
       }
   } else {
       d.Logger.Warn("thread: status.engine tidak tersedia, kartu aktivitas dinonaktifkan")
   }
   ```
   (paket `status` sudah diimpor; hasil `stop` boleh diabaikan karena seumur daemon). Wiring berjalan di dalam `d.OnReady`, jadi `status.engine` sudah tersedia.
2. **Router tiket 02**: tidak perlu diubah. Konfirmasi: aksi activity ditulis lewat `Manager.UpsertMessage` yang tidak memicu `OnMessage` (2.2 no. 2), jadi tidak masuk antrean mention/anti-loop. Catat di pesan commit: "activity ditulis lewat jalur log+publish tanpa OnMessage".
3. **Frontend `frontend/src/modules/thread/MessageList.svelte`** (tiket 01, satu cabang render + impor):
   - impor: `import { ActivityFlowItem, groupActivity } from './activity';`
   - `const flow = $derived(groupActivity(messages, (id) => participantName(thread, id)));` (impor `participantName` dari `./thread-store`);
   - ganti `{#each messages as m (m.id)}<MessageItem .../>` menjadi `{#each flow as item (item.key)}{#if item.type === 'activity'}<ActivityFlowItem {thread} group={item} />{:else}<MessageItem {thread} message={item.message} />{/if}{:else}...`;
   - efek tempel-ke-bawah (`$effect` yang membaca `messages.length`): ubah penanda "bertambah" menjadi `flow.length` (item aliran) ditambah `count` grup terakhir bila grup activity, agar kartu yang bertambah tidak memicu lompatan, tetapi item baru tetap menempel ke bawah. Tidak ada perubahan perilaku lain.
   - `MessageItem.svelte` TIDAK diubah (cabang `plain` untuk `activity` tetap sebagai cadangan).
4. **`thread-store.ts`**: verifikasi `mergeMessages` sudah upsert berdasarkan `id` (ya, `Map` per id); tidak ada perubahan. Bila kode berubah menjadi hanya-tambah, ubah menjadi upsert (perubahan kecil, sebut di commit).
5. **Verifikasi manual** (jalankan aplikasi): buat thread, minta master men-spawn peer yang mengedit file; kartu "<nama>: N aksi" muncul di aliran, hitungan bertambah langsung, bisa dibuka-tutup dengan mouse dan keyboard, status berubah "selesai" setelah jeda 30 detik atau setelah agent itu mengirim pesan; pesan diskusi tetap penuh dan memutus kartu; `cat <DataDir>/threads/<id>/messages.jsonl` menunjukkan satu baris activity per rentetan; sesi interaktif user di project yang sama tidak memunculkan kartu; tutup lalu buka ulang tab/restart daemon: kartu sama dari log, status "selesai" pada thread closed.

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- `go build ./...` (akar repo) dan `gofmt -l internal/thread` kosong, dijalankan Foundation, `thread-activity-daemon`, dan Wiring.
- Frontend: `cd frontend && npm run check && npm run build` (opsional `make build`), dijalankan Foundation, `thread-activity-ui`, dan Wiring.
- Tidak ada dependency baru di `go.mod`/`package.json`; `go build ./cmd/shikigamid` tanpa galat.

## 7. Aturan paralel

- Tiap agent HANYA menyentuh `ownedPaths` fiturnya (bagian 3). Dilarang menyentuh path fitur lain, berkas Foundation (`types.go`, `types.ts` bagian yang ditambah, `activity_support.go`), berkas wiring (`wire_thread.go`, `MessageList.svelte`), `store.go`/`manager.go`/`ipc.go` tiket 01, `internal/status`, `internal/runs`, `internal/mention`, `internal/session`.
- `thread-activity-daemon` hanya mengedit `internal/thread/activity.go` (menimpa stub Foundation, signature 2.3 tetap). `thread-activity-ui` hanya membuat/mengedit berkas di `frontend/src/modules/thread/activity/`; sebelum wiring, folder itu belum dirender siapa pun, jadi uji dengan data lokal.
- Butuh perubahan di luar `ownedPaths`: JANGAN dikerjakan; catat di bagian "Catatan untuk wiring" plan fitur.
- Jangan mengubah signature/konstanta/format 2.2-2.4; jangan menambah ekspor baru yang dipakai fitur lain.
- Tiap agent menjalankan guard bagian 6 yang relevan sebelum melapor selesai; kegagalan build akibat berkas fitur lain hanya dilaporkan, bukan diperbaiki.
