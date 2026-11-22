# Overview: agent-diff-review-ticket-01-review-capture

Tiket: `docs/tickets/agent-diff-review-ticket-01-review-capture.md`. Spec: `docs/superpowers/specs/2026-11-22-agent-diff-review-design.md`.
Dokumen ini adalah sumber kebenaran BERSAMA untuk semua plan fitur epic ini. Bila ada perbedaan kecil dengan tiket, tiket menang kecuali dinyatakan "koreksi" di bagian 2.2 (koreksi muncul karena kode nyata berbeda dari asumsi tiket). Tanpa dependency baru, tanpa unit test; guard hanya build.

Keadaan repo saat overview ditulis: belum ada `internal/review/`, `cmd/shikigamid/wire_review.go`, maupun `frontend/src/modules/review/`. Tiket review-diff, review-actions, dan review-comment (ticket-02 s.d. 04) baru berupa tiket; epic ini hanya menyediakan titik ekstensi. Working tree punya perubahan user di `frontend/src/modules/sidebar-right/FilesTab.svelte`: JANGAN disentuh.

## 1. Ringkasan epic dan urutan eksekusi

Fondasi fitur review perubahan agent: hook `PreToolUse` pada tool edit Claude (Edit, Write, MultiEdit, NotebookEdit) menyalin isi file ke penyimpanan sesi sekali pada sentuhan pertama (baseline per file per sesi); `PostToolUse` mencatat hash pasca-edit dan memicu event `review.changed` (debounce 300 ms). UI menampilkan tab kanan "Review" (daftar file berubah + filter "belum ditinjau") dan badge jumlah di `SessionRow`. Belum ada diff, revert, terima, komentar, atau deteksi `external`. Jalan di folder non-git.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): `internal/review/types.go` final (lengkap) serta `store.go` dan `capture.go` berupa stub berkompilasi dengan signature final (bagian 2.3). Setelah selesai `go build ./...` lulus. Tidak ada kerja frontend di Foundation.
2. **Fitur paralel** (3 fitur, bagian 3): `review-foundation` (mengganti stub `store.go` dan `capture.go` dengan implementasi penuh, boleh menyempurnakan `types.go` tanpa mengubah bentuk), `review-hooks-ipc`, `review-ui-list`. Signature bagian 2 TIDAK berubah. Fitur `review-hooks-ipc` memakai stub saat dikerjakan paralel; ia hanya perlu build lulus, perilaku akhir baru benar setelah `review-foundation` selesai.
3. **Wiring** (bagian 5): fitur `review-capture-wiring`, dikerjakan setelah ketiga fitur selesai.

## 2. Kontrak bersama

### 2.1 Paket, folder, penamaan

- Modul Go `github.com/NooBeeID/shikigami`; paket `internal/review` (package `review`). Berkas: `types.go`, `store.go`, `capture.go`, `ipc.go`; wiring daemon `cmd/shikigamid/wire_review.go` (package `main`). Tiket 02-04 menambah berkas baru di paket yang sama (`diff.go`, `diff_ipc.go`, `actions.go`, `actions_ipc.go`, `comment.go`): jangan dibuat di epic ini.
- Frontend: `frontend/src/modules/review/` (Svelte 5 runes, TypeScript): `index.ts`, `types.ts`, `store.ts`, `ReviewTab.svelte`, `ReviewBadge.svelte`.
- Komentar, log, pesan galat, teks UI bahasa Indonesia; identifier bahasa Inggris. Logger `*slog.Logger` nullable di `Deps` (cek nil, fallback `slog.Default()`).
- Helper tak-diekspor diberi awalan berkas (`store*`, `capture*`, `ipc*`; di package `main`: `reviewWire*`) agar tidak bentrok antar berkas paralel. Konstanta dan tipe didefinisikan hanya di berkas pemiliknya (2.3).
- JSON camelCase. Waktu disimpan sebagai unix milidetik (`int64`).
- Path file selalu relatif terhadap cwd sesi dengan pemisah `/` (`filepath.ToSlash`).

### 2.2 Koreksi dan keputusan atas tiket (WAJIB diikuti)

Hasil pembacaan kode nyata:

1. **Matcher hook: tidak ada logika merge.** `writeHooksFile` di `internal/status/install.go` menulis berkas hooks BARU per sesi (`DataDir/run/status-hooks-<token>.json`) dari konstanta `preToolUseMatcher = "Workflow|Agent"`. Cukup ubah konstanta menjadi `"Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit"` dan perbarui komentar (termasuk komentar `hookEvents` dan "membatasi PreToolUse ke tool Workflow dan Agent"). Tidak ada entri lama yang dipertahankan; sesi lama yang sudah berjalan tidak ikut. Tidak perlu mengubah `hooks.go` kecuali butuh helper.
2. **Aman terhadap runs dan status, tanpa mengubah keduanya.** Sudah diverifikasi: `status.ClassifyEvent` tidak mengenal `PreToolUse` (event "tidak dikenal" tidak mengubah status); `internal/runs/runs.go` `preToolUse` memakai `switch tool_name` hanya untuk `Workflow` dan `Agent` sehingga tool edit jatuh ke default dan mengembalikan false; `ActivityLog.Record` hanya memproses `PostToolUse` (perilaku `PostToolUse` tidak berubah karena sudah terpasang tanpa matcher). Maka tidak ada guard tambahan di `internal/runs` atau `ClassifyEvent`. Guard hanya ada di listener review: abaikan semua `tool_name` selain empat tool edit.
3. **Cwd sesi dari `eng.Lookup`, bukan dari payload.** `status.Engine.Lookup(sessionID) (session.Session, bool)` mengembalikan `Cwd` dan `ProviderID` sesi (lewat `SessionSource`). Cwd payload hook bisa berubah setelah agent `cd`, sehingga path relatif tidak konsisten. Aturan: `cwd = Lookup(sessionID).Cwd`; bila Lookup gagal atau Cwd kosong, pakai `payload["cwd"]`; bila keduanya kosong, abaikan event (log debug). `Store` menyimpan `cwd` pada `index.json` saat Capture pertama; `review.files` memakai `store.Cwd(sessionID)` dan hanya jatuh ke `Lookup` bila index belum ada.
4. **`supportedFn`**: `func(sessionID string) bool` dibuat di `wire_review.go`: `s, ok := eng.Lookup(id)`; `provider.Get(s.ProviderID)` lalu `Capabilities().Hooks`. Tidak ok atau tidak mendukung -> false. (Paket `internal/provider`: `provider.Get(id) (Provider, bool)`, `Capabilities().Hooks`; claude mendukung, shell tidak.)
5. **Signature `Prune` diperluas.** Tiket menulis `Prune(keepEnded int)` tetapi juga meminta daftar id sesi aktif dari pemanggil. Signature final: `Prune(keepEnded int, activeSessionIDs []string)`. Direktori sesi yang id-nya ada di `activeSessionIDs` tidak pernah dihapus; sisanya (sesi berakhir) diurutkan menurun berdasarkan `firstTouchAt` maksimum dari entri sesi itu (fallback mtime direktori bila index kosong) dan hanya `keepEnded` teratas yang disimpan; sisanya dihapus dengan `os.RemoveAll`. `wire_review.go` memanggil `Prune(20, ids)` sekali saat `OnReady` (ids dari `eng.List()` -> `SessionState.SessionID`; bila field berbeda, baca `internal/status` dan sesuaikan) tanpa memblokir (goroutine).
6. **Tambahan API `Store` (hanya menambah, aditif untuk tiket 02-04):** `Cwd(sessionID string) string`, `SessionDir(sessionID string) string`, `BaselinePath(sessionID, relPath string) string`, `HashPath(relPath string) string` (sha256 hex dari path relatif; dipakai untuk nama berkas baseline). Konstanta `HashAbsent` untuk hash "file tidak ada".
7. **Sesi aktif di frontend = pane terminal yang fokus.** Tidak ada store "sesi aktif" tunggal. `store.ts` menurunkannya dari `layout` (`frontend/src/lib/stores/layout.ts`): `focusId` pada `LayoutDoc` + `findLeaf(doc, id)` dari `frontend/src/modules/layout/model`; bila leaf fokus `kind === 'terminal'` dan punya `sessionId`, itu sesi aktif; bila fokus pada pane non-terminal (mis. file), pertahankan sesi aktif sebelumnya; bila belum ada, `null`. Pola sinkronisasi mengikuti `modules/runs/store.ts` (subscribe di `start...Sync`, hindari balasan basi dengan nomor muat).
8. **Hitungan awal badge.** `review.changed` hanya datang saat ada edit baru, jadi setelah reload UI badge kosong. `store.ts` juga berlangganan store `sessions`: untuk setiap sesi baru yang terlihat (belum exited), panggil `review.files` sekali di latar (abaikan galat, jangan melempar) agar `reviewCounts` terisi. Sesi dengan `supported:false` tidak diberi count.
9. **Penghitungan `count` untuk event**: jumlah entri hasil `Files` dengan `Reviewed == false`.
10. **Kode galat IPC**: `ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh {\"sessionId\": string}")` untuk params salah (`internal/ipc/errors.go` memuat `ErrInvalidParams = -32602` dan `ErrInvalid = -32003`; modul lain memakai pola `ipc.NewError(code, msg)`). Gunakan `ErrInvalidParams` untuk bentuk params, `ErrInvalid` untuk nilai tidak valid (sessionId kosong).

### 2.3 Signature Go bersama (final)

**`types.go`** (ditulis penuh oleh Foundation; `review-foundation` boleh menambah komentar atau field opsional, tidak mengubah yang ada):

```go
package review

type Status string

const (
	StatusModified Status = "modified"
	StatusAdded    Status = "added"
	StatusDeleted  Status = "deleted"
)

const (
	maxBaselineBytes = 1 << 20 // di atas ini: tooLarge, tanpa salinan isi
	HashAbsent       = "absent" // expectedHash untuk file yang tidak ada (deleted)
	IPCMethodFiles   = "review.files"
	EventChanged     = "review.changed"
)

type FileEntry struct {
	Path      string `json:"path"`
	Status    Status `json:"status"`
	Additions int    `json:"additions"` // 0 sampai ticket-02
	Deletions int    `json:"deletions"` // 0 sampai ticket-02
	Reviewed  bool   `json:"reviewed"`
	External  bool   `json:"external"` // false sampai ticket-03
	Binary    bool   `json:"binary"`
	TooLarge  bool   `json:"tooLarge"`
}

// TouchedFile = entri index.json mentah. Nama field JSON adalah kontrak tetap
// (hanya boleh ditambah, tidak diganti nama).
type TouchedFile struct {
	Path           string `json:"path"`           // relatif cwd, '/'
	ExistedBefore  bool   `json:"existedBefore"`
	BaselineSha256 string `json:"baselineSha256"` // "" bila existedBefore=false
	FirstTouchAt   int64  `json:"firstTouchAt"`   // unix ms
	Reviewed       bool   `json:"reviewed"`
	ExpectedHash   string `json:"expectedHash"`   // sha256 hex isi pasca-edit agent terakhir, atau HashAbsent
	Binary         bool   `json:"binary"`
	TooLarge       bool   `json:"tooLarge"`
}

// FilesResult adalah hasil IPC review.files.
type FilesResult struct {
	Supported bool        `json:"supported"`
	Files     []FileEntry `json:"files"` // selalu [] (bukan null)
}
```

**`index.json`** (dimiliki `store.go`): objek `{"cwd": "<abs cwd>", "files": [TouchedFile, ...]}`. Kunci `cwd` di tingkat atas (kontrak untuk tiket 02-04, diakses lewat `Store.Cwd`). Ditulis atomik (berkas sementara `index.json.tmp` + `os.Rename`), izin `0o600`, direktori `0o700`. JSON rusak atau tidak ada diperlakukan kosong.

**`store.go` dan `capture.go`** (stub di Foundation, implementasi penuh oleh `review-foundation`; signature final):

```go
type Store struct { /* dataDir string; mutex per sesi (map[string]*sync.Mutex dilindungi mutex global) */ }

func NewStore(dataDir string) *Store

// store.go
func (s *Store) Touched(sessionID string) []TouchedFile                      // salinan; nil -> []TouchedFile{}
func (s *Store) Files(sessionID, cwd string) ([]FileEntry, error)            // diurut Path naik; selalu non-nil
func (s *Store) Prune(keepEnded int, activeSessionIDs []string)
func (s *Store) Cwd(sessionID string) string
func (s *Store) SessionDir(sessionID string) string                          // <dataDir>/review/<sessionID>
func (s *Store) BaselinePath(sessionID, relPath string) string               // <SessionDir>/baseline/<HashPath(relPath)>
func (s *Store) HashPath(relPath string) string                              // sha256 hex

// capture.go
func (s *Store) Capture(sessionID, cwd, path string) error                   // path boleh absolut atau relatif; di luar cwd -> nil tanpa efek
func (s *Store) MarkChanged(sessionID, cwd, path string) error               // entri tak ada -> nil tanpa efek
```

Aturan status `Files` (rinci di tiket): `added` bila `existedBefore=false` dan file ada sekarang; `deleted` bila `existedBefore=true` dan file tidak ada; `modified` bila ada keduanya dan sha256 sekarang != `baselineSha256`; selain itu entri dibuang. File `binary`/`tooLarge` hanya dibandingkan hash. Validasi path: `fsx.ResolveUnder(cwd, rel)` (di `internal/fsx/readwrite.go`; menolak root relatif, `..`, path absolut pada argumen `rel`, dan symlink keluar root). Path absolut dari hook diubah dulu ke relatif lewat `filepath.Rel(cwd, abs)`; hasil diawali `..` atau galat -> abaikan tanpa galat. Deteksi biner: `fsx.IsBinary(sample)` (sampel 8 KiB pertama).

### 2.4 Kontrak IPC dan event (final)

- `review.files {sessionId}` -> `FilesResult`. `supported:false` dan `files:[]` untuk sesi provider tanpa hooks (`supportedFn`).
- Event `review.changed {sessionId, count}` lewat `srv.Publish("review.changed", map[string]any{"sessionId": id, "count": n})`.
- Hook: matcher `PreToolUse` jadi `Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit`.
- Pendaftaran: `review.RegisterIPC(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool)` di `ipc.go`; mendaftarkan `review.files` lewat `srv.Register(method, func(ctx context.Context, params json.RawMessage) (any, error))`. Tiket 02-04 menambah fungsi pendaftaran lain di berkas lain; jangan menyentuh berkas itu.
- Layanan: `d.Provide("review.store", store)` (sekali) agar tiket berikut mengambilnya lewat `d.Lookup("review.store")` di `OnReady`.

### 2.5 Kontrak frontend (final)

```ts
// types.ts
export type ReviewStatus = 'modified' | 'added' | 'deleted';
export interface ReviewFile { path: string; status: ReviewStatus; additions: number; deletions: number; reviewed: boolean; external: boolean; binary: boolean; tooLarge: boolean }
export interface ReviewFilesResult { supported: boolean; files: ReviewFile[] }
// store.ts
export const reviewFiles: Readable<ReviewFilesResult>;        // sesi aktif; awal { supported: true, files: [] }
export const reviewCounts: Readable<Record<string, number>>;  // sessionId -> jumlah belum ditinjau
export function refreshReview(sessionId: string): Promise<void>;
export function startReviewSync(): () => void;                // idempoten; dipanggil index.ts
// ReviewBadge.svelte: props { sessionId: string }; tidak merender apa pun bila count 0
// index.ts
registerRightTab({ id: 'review', title: 'Review', order: 15, component: ReviewTab });
export { ReviewBadge };
export { reviewFiles, reviewCounts, refreshReview } from './store';
export type { ReviewFile, ReviewStatus, ReviewFilesResult } from './types';
```

- `registerRightTab` dan `RightTabSpec` ada di `frontend/src/modules/shell/registry.ts`; Files memakai `order: 10` (didaftarkan di `modules/sidebar-right/index.ts`), Activity `order: 20` (`modules/activity/index.ts`). `call` dan `onEvent(topicPrefix, (topic, payload) => void)` dari `frontend/src/lib/ipc.ts`. Store sesi di `frontend/src/lib/stores/sessions.ts` (`sessions`), `activeWorkspaceId` di `frontend/src/lib/stores/workspaces.ts`; semua dapat diimpor dari `frontend/src/lib/stores`.
- `ReviewTab.svelte` menerima props kosong (dirender sebagai `<C />` oleh `RightPanel`). Titik kait: state `selected: ReviewFile | null` dan callback `onselect?: (file: ReviewFile) => void` (prop opsional) tanpa efek di tiket ini.
- Gaya: token CSS yang sama dengan `modules/sidebar-right/FilesTab.svelte` dan `modules/activity/ActivityTab.svelte` (baca keduanya; jangan mengubah keduanya).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|------|-------|------------|
| `review-foundation` | Tipe final, penyimpanan baseline, `index.json` atomik, `Capture`, `MarkChanged`, `Touched`, `Files`, `Prune`, helper path. Mengganti stub Foundation. | `internal/review/types.go`, `internal/review/store.go`, `internal/review/capture.go` |
| `review-hooks-ipc` | Ubah konstanta matcher; `RegisterIPC` untuk `review.files`; `wire_review.go` (listener hook, debounce 300 ms per sesi, event, prune awal); spike `tool_input` MultiEdit/NotebookEdit sebagai komentar. | `internal/review/ipc.go`, `cmd/shikigamid/wire_review.go`, `internal/status/install.go`, `internal/status/hooks.go` |
| `review-ui-list` | Modul frontend `review`: tipe, store (sesi aktif, `review.changed`, hitungan awal), `ReviewTab`, `ReviewBadge`, `index.ts`. | `frontend/src/modules/review/index.ts`, `frontend/src/modules/review/ReviewTab.svelte`, `frontend/src/modules/review/store.ts`, `frontend/src/modules/review/types.ts`, `frontend/src/modules/review/ReviewBadge.svelte` |
| `review-capture-wiring` | Pasang `ReviewBadge` di `SessionRow`; verifikasi registrasi modul daemon/frontend dan alur penuh. Terakhir. | `frontend/src/modules/sidebar-left/SessionRow.svelte`, `frontend/src/main.ts` |

Ketergantungan: `review-foundation`, `review-hooks-ipc`, `review-ui-list` paralel setelah Foundation; `review-capture-wiring` setelah ketiganya.

Catatan per fitur:

- **review-foundation**: lihat 2.3. `Capture`: normalisasi dan validasi path, bila sudah ada di index kembalikan nil tanpa menyalin ulang; file ada dan <= 1 MiB dan bukan biner -> salin isi ke `BaselinePath` (tulis atomik juga); biner atau > 1 MiB -> entri dengan flag, tanpa salinan; tidak ada -> `existedBefore=false`; simpan `baselineSha256` (hash isi penuh untuk file ada, termasuk biner/besar via streaming), `firstTouchAt` (`time.Now().UnixMilli()`), `cwd` di index. Direktori (bukan file biasa) -> abaikan nil. Target < 20 ms untuk file < 1 MiB, tanpa jaringan. `MarkChanged` menghitung sha256 isi sekarang -> `expectedHash` (tidak ada -> `HashAbsent`). Tidak ada panic pada input apa pun.
- **review-hooks-ipc**: listener di `wire_review.go`: `PreToolUse` + tool edit -> ekstrak path dari `tool_input` (`file_path`; NotebookEdit coba `notebook_path` lalu `file_path`; MultiEdit memakai `file_path` per dokumentasi Claude Code, verifikasi dan catat di komentar) -> `store.Capture` sinkron dan cepat; `PostToolUse` + tool edit -> `store.MarkChanged`, lalu debounce 300 ms per sesi (map sesi -> `*time.Timer` dengan mutex; reset timer per event) yang menghitung `Files` dan `Publish("review.changed")`. `AddExitListener` membersihkan timer sesi. Error hanya dilog (Debug/Warn), listener tidak pernah mengembalikan galat. Panggil `Prune` awal. `ipc.go` berisi `RegisterIPC` dan `Files` hasil dipetakan ke `FilesResult`.
- **review-ui-list**: lihat 2.5 dan butir koreksi 7 dan 8. Teks UI: kosong "Belum ada file yang diubah agent di sesi ini", tidak didukung "Provider sesi ini tidak mendukung review perubahan", filter "Belum ditinjau". Penanda status M/A/D. Galat IPC -> state kosong, jangan melempar.
- **review-capture-wiring**: lihat bagian 5.

## 4. Foundation

Ada (`hasFoundation = true`): `00-foundation.md`. Cakupan:

1. Buat `internal/review/types.go` lengkap sesuai 2.3.
2. Buat `internal/review/store.go` dan `internal/review/capture.go` berupa stub: struct `Store`, `NewStore`, dan semua method di 2.3 dengan signature final dan badan minimal (kembalikan nilai nol non-nil: `[]TouchedFile{}`, `[]FileEntry{}`, `""`, `nil`). Satu-satunya tujuan: paket berkompilasi sehingga `review-hooks-ipc` dapat dibangun paralel.
3. Verifikasi `go build ./...`.

Tidak ada foundation frontend; `review-ui-list` memakai kontrak JSON langsung.

## 5. Wiring checklist

Untuk fitur `review-capture-wiring` (setelah ketiga fitur lain selesai):

1. **Modul daemon**: pastikan `cmd/shikigamid/wire_review.go` punya `func init() { RegisterModule("review", ...) }`. Modul terbangun otomatis karena semua berkas package `main` dikompilasi; JANGAN mengubah `modules.go`. Pastikan nama modul `"review"` unik dan `d.Provide("review.store", ...)` dipanggil sekali. `go build ./...`.
2. **Modul frontend**: `frontend/src/main.ts` memuat modul lewat `import.meta.glob('./modules/*/index.ts', { eager: true })`; verifikasi glob itu ada dan karena itu `modules/review/index.ts` ter-load (jangan mengubah `main.ts` bila sudah tercakup; ubah hanya jika glob tidak ada). Pastikan `registerRightTab` jalan (tab Review tampil setelah Files, sebelum Activity) dan `startReviewSync()` dipanggil dari `index.ts`.
3. **Badge**: di `frontend/src/modules/sidebar-left/SessionRow.svelte` tambah tepat satu import `import { ReviewBadge } from '../review';` (di blok import, dekat `StatusBadge` pada baris 6) dan satu elemen `<ReviewBadge sessionId={session.id} />` di dalam tombol `.main`, tepat setelah `<span class="title">...</span>` dan sebelum `<span class="prov">` (sehingga di dekat `StatusBadge`). Badge tidak merender apa pun saat count 0, sehingga tata letak baris tidak berubah. Jangan menyentuh logika lain.
4. **Hook**: matcher baru ada di `internal/status/install.go` (`preToolUseMatcher`); hanya sesi yang dibuat setelah daemon baru memakainya. Jalankan aplikasi (skill `run`: `make run-daemon` / app), buka sesi Claude, minta agent membuat dan mengedit file; konfirmasi tab Review menampilkan file dan badge di `SessionRow` bertambah; edit file yang sama dua kali menghasilkan satu entri; sesi shell menampilkan "Provider sesi ini tidak mendukung review perubahan".
5. **Non-git**: ulangi langkah 4 pada folder tanpa `.git`.
6. Jalankan seluruh Build guard (bagian 6).

## 6. Build guard

Satu-satunya guard (tanpa unit test), jalankan dari root repo:

- `go build ./...`
- `cd frontend && npm run build` dan `cd frontend && npm run check` (`svelte-check`, script ada di `frontend/package.json`).

Fitur Go cukup `go build ./...`; fitur frontend cukup dua perintah frontend. Fitur wiring menjalankan semuanya.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, berkas wiring (`SessionRow.svelte`, `main.ts`, `modules.go`), `internal/runs`, `ClassifyEvent`, `FilesTab.svelte`, ataupun berkas tiket 02-04 (`diff*.go`, `actions*.go`, `comment.go`, `DiffView.svelte`, `diff.ts`, `HunkActions.svelte`, `keys.ts`, `CommentBox.svelte`).
- Signature, nama field JSON, nama IPC/event/topic, dan nama ekspor di bagian 2 tidak boleh berubah. Bila kontrak terasa keliru, tulis catatan di plan fiturmu, jangan ubah diam-diam.
- Tanpa dependency baru (Go maupun npm), tanpa unit test, tanpa commit pada berkas milik fitur lain.
- Setiap fitur harus lulus guard bagiannya sebelum dilaporkan selesai.
