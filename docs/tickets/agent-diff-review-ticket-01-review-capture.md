---
id: agent-diff-review-ticket-01-review-capture
title: Review: tangkap baseline dan daftar file berubah
spec: docs/superpowers/specs/2026-11-22-agent-diff-review-design.md
status: implemented
wave: 1
depends_on: []
---
# Review: tangkap baseline dan daftar file berubah

## Tujuan

Epic ini membangun fondasi fitur review perubahan agent: menyimpan baseline per file per sesi lewat hook edit Claude, lalu menampilkan daftar file yang diubah sesi itu di UI. Tanpa baseline sentuhan pertama, tidak ada konsep "perubahan milik sesi X" yang bisa dibedakan dari edit user, sesi lain, atau commit. Epic ini juga menetapkan titik ekstensi (tipe, store, tab, badge, IPC, event) yang dipakai tiket review-diff, review-actions, dan review-comment. Pendekatan yang dipilih spec adalah "C": baseline per file lewat hook, bukan `git diff` dan bukan snapshot seluruh tree, sehingga jalan di folder non-git.

## Hasil yang bisa dijalankan

Setelah tiket ini selesai (berdiri sendiri, tanpa tiket lain):
- Saat agent Claude di sebuah sesi memakai Edit, Write, MultiEdit, atau NotebookEdit, tab kanan baru "Review" (setelah tab Files) menampilkan daftar file yang diubah sesi itu, dengan status `modified`, `added`, atau `deleted`, dan filter "belum ditinjau".
- Baris sesi di sidebar kiri (`SessionRow`) menampilkan badge jumlah file berubah dan belum ditinjau; badge ikut berubah saat agent mengedit (event `review.changed`, debounce 300 ms).
- Dua edit berturut-turut pada file yang sama menghasilkan satu entri (baseline = isi sebelum sentuhan pertama). File yang isinya kembali sama dengan baseline hilang dari daftar.
- Berfungsi di folder non-git.
- Sesi provider tanpa hooks (mis. shell, codex) menampilkan pesan "provider tidak mendukung" di tab Review, tanpa error.
- Belum ada: diff, angka `+/-` (selalu 0), revert, terima, komentar, deteksi perubahan lain (`external` selalu `false`), klik file untuk membuka. Build `go build ./...` dan build frontend lulus dan aplikasi jalan seperti sebelumnya.

## Batasan

In scope:
- Paket baru `internal/review`: tipe, penyimpanan baseline + `index.json`, `Capture`, `MarkChanged`, `Touched`/`Files`, retensi.
- Perluasan matcher `PreToolUse` di `internal/status/install.go` menjadi `Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit`.
- Wiring daemon `cmd/shikigamid/wire_review.go` (listener hook, debounce, event) dan IPC `review.files`.
- Modul frontend `frontend/src/modules/review` (store, ReviewTab, ReviewBadge, tipe) dan pemasangan badge di `SessionRow.svelte` (perubahan minimal).
- Spike singkat format `tool_input` MultiEdit/NotebookEdit.

Out of scope (milik tiket lain, jangan disentuh):
- `internal/review/diff.go`, `diff_ipc.go`, `DiffView.svelte`, `diff.ts` (ticket-02-review-diff): perhitungan diff dan angka `additions/deletions`.
- `internal/review/actions.go`, `actions_ipc.go`, `HunkActions.svelte`, `keys.ts` (ticket-03-review-actions): revert, terima, deteksi `external`, navigasi keyboard.
- `internal/review/comment.go`, `CommentBox.svelte` (ticket-04-review-comment).
- Mengubah perilaku `ClassifyEvent` atau registry runs (`internal/runs`). Menambah dependency Go/npm. Unit test. Menangkap perubahan dari `Bash` (`sed -i`, generator): celah yang diterima spec v1.

## Kontrak

### Yang dipakai (sudah ada di repo)

- `status.Engine.AddHookListener(fn func(sessionID, event string, payload map[string]any)) (cancel func())` di `internal/status/engine.go`. Payload adalah map hasil `ParseHookEvent`: field `tool_name`, `tool_input` (objek, memuat `file_path`), `cwd`, `session_id` ada sebagai kunci map; field tak dikenal dipertahankan. Listener dilindungi dari panic, tetapi tetap tulis kode yang tidak panic.
- Pola modul daemon: `RegisterModule("review", func(srv *ipc.Server, d *Deps){...})`, `d.Provide`, `d.OnReady`, `d.Lookup("status.engine")` (tipe `*status.Engine`), `d.DataDir`, `srv.Register(method, func(ctx, params json.RawMessage) (any, error))`, `srv.Publish(topic, payload)`. Ikuti `cmd/shikigamid/wire_runs.go` sebagai contoh persis (termasuk log `Warn` bila engine tidak tersedia). Cwd sesi: ambil dari payload hook `cwd`, dan untuk `review.files` dari index sesi yang menyimpan `cwd` saat Capture pertama (hindari dependensi baru ke modul sesi bila lookup-nya tidak jelas; baca `wire_activity.go`/`wire_session.go` untuk melihat apakah ada layanan sesi yang sudah di-`Provide`).
- Validasi path: aturan `internal/fsx` (`fsx.ResolveUnder(root, rel)` menolak path di luar root dan symlink keluar). Path hook sering absolut: ubah ke relatif terhadap cwd sebelum `ResolveUnder`; bila di luar cwd, abaikan tanpa error.
- Frontend: `registerRightTab` dari `frontend/src/modules/shell` (lihat `modules/activity/index.ts`: `registerRightTab({ id, title, order, component })`; Files memakai order 10, Activity 20), `call` dan `onEvent` dari `frontend/src/lib/ipc`, store sesi di `frontend/src/lib/stores/sessions` dan `activeWorkspaceId`. Pola store: `frontend/src/modules/runs/store.ts`.
- `frontend/src/main.ts` memuat modul lewat `import.meta.glob('./modules/*/index.ts', { eager: true })`, jadi `modules/review/index.ts` otomatis ter-load.

### Yang disediakan

Go, paket `internal/review`:
```go
type Status string // "modified" | "added" | "deleted"

type FileEntry struct {
    Path      string `json:"path"`      // relatif cwd, pemisah '/'
    Status    Status `json:"status"`
    Additions int    `json:"additions"` // 0 sampai ticket-02
    Deletions int    `json:"deletions"` // 0 sampai ticket-02
    Reviewed  bool   `json:"reviewed"`
    External  bool   `json:"external"`  // false sampai ticket-03
    Binary    bool   `json:"binary"`
    TooLarge  bool   `json:"tooLarge"`
}

type Store struct{ /* dataDir, mutex per sesi */ }
func NewStore(dataDir string) *Store
func (s *Store) Capture(sessionID, cwd, path string) error   // salin baseline sekali pada sentuhan pertama
func (s *Store) MarkChanged(sessionID, cwd, path string) error // catat sha256 pasca-edit (expectedHash)
func (s *Store) Touched(sessionID string) []TouchedFile       // entri index mentah (path, existedBefore, baselineHash, firstTouchAt, reviewed, expectedHash, binary, tooLarge)
func (s *Store) Files(sessionID, cwd string) ([]FileEntry, error) // bandingkan isi sekarang vs baseline, buang yang sama
func (s *Store) Prune(keepEnded int)                          // retensi
```
`TouchedFile` dan field index adalah kontrak untuk tiket 02-04 (mereka menambah fungsi di file baru, bukan mengubah file ini); tambahkan hanya, jangan ganti nama field `index.json` setelah ditetapkan: `path`, `existedBefore`, `baselineSha256`, `firstTouchAt` (unix ms), `reviewed`, `expectedHash`, `binary`, `tooLarge`, `cwd`.

Penyimpanan: `<DataDir>/review/<sessionId>/baseline/<sha256(pathRelatif) hex>` (isi) dan `<DataDir>/review/<sessionId>/index.json`. Tulis index atomik (file sementara + rename).

IPC:
- `review.files {sessionId}` -> `{supported: bool, files: [{path, status, additions, deletions, reviewed, external, binary, tooLarge}]}`. `supported:false` (dan `files:[]`) untuk sesi provider tanpa hooks. Error params tidak valid memakai kode ERR_INVALID seperti modul lain.
- Event `review.changed {sessionId, count}`; `count` = jumlah file belum ditinjau. Debounce 300 ms per sesi, dikirim setelah `PostToolUse` edit.

Hook: `PreToolUse` matcher menjadi `Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit`. `PostToolUse` sudah terpasang tanpa matcher; tidak diubah.

Frontend (`frontend/src/modules/review`):
```ts
// types.ts
export type ReviewStatus = 'modified' | 'added' | 'deleted';
export interface ReviewFile { path: string; status: ReviewStatus; additions: number; deletions: number; reviewed: boolean; external: boolean; binary: boolean; tooLarge: boolean }
export interface ReviewFilesResult { supported: boolean; files: ReviewFile[] }
// store.ts
export const reviewFiles: Readable<ReviewFilesResult>;          // untuk sesi aktif
export const reviewCounts: Readable<Record<string, number>>;     // sessionId -> count belum ditinjau (dari review.changed dan review.files)
export function refreshReview(sessionId: string): Promise<void>;
// ReviewBadge.svelte: props { sessionId: string }; kosong bila count 0
```
Tiket 02-04 memakai `reviewFiles`, `reviewCounts`, `ReviewFile`; jaga nama ini.

## Fitur

Urutan: `review-foundation` dikerjakan lebih dulu (tipe dan Store dipakai fitur lain). `review-hooks-ipc` dan `review-ui-list` bisa dikerjakan paralel setelah tipe/kontrak di atas tetap (UI bisa memakai kontrak JSON tanpa menunggu Go selesai). `review-capture-wiring` terakhir.

### review-foundation: Tipe, penyimpanan baseline, dan Capture

Deskripsi:
- `types.go`: `Status`, `FileEntry`, `TouchedFile`, konstanta batas ukuran (`maxBaselineBytes = 1 << 20`).
- `store.go`: `Store` dengan mutex per sesi, baca/tulis `index.json` atomik, `Touched`, `Files`, `Prune`. `Files` menghitung status: `added` bila `existedBefore=false` dan file ada sekarang; `deleted` bila `existedBefore=true` dan file tidak ada sekarang; `modified` bila ada keduanya dan sha256 isi sekarang != baseline. Entri yang isinya sudah sama dengan baseline (atau `added` lalu dihapus kembali / dibuat lalu hilang) tidak dikembalikan. Untuk file `binary`/`tooLarge` bandingkan hash saja.
- `capture.go`: `Capture(sessionID, cwd, path)`: normalisasi path ke relatif cwd, validasi dengan `fsx.ResolveUnder`, bila sudah ada di index kembalikan nil tanpa menyalin ulang. Bila file ada dan <= 1 MB dan bukan biner (deteksi seperti `internal/fsx/binary.go`): salin isi ke `baseline/<hash>`; bila biner atau > 1 MB: catat entri dengan `binary`/`tooLarge` true dan tanpa salinan isi; bila file tidak ada: `existedBefore=false`. Simpan `baselineSha256` dan `firstTouchAt`. `MarkChanged` menghitung sha256 isi sekarang dan menyimpannya sebagai `expectedHash` (file tidak ada -> hash kosong khusus "deleted"). Target latensi `Capture` < 20 ms untuk file < 1 MB; tanpa operasi jaringan. `Prune(keepEnded)` menghapus direktori `review/<sessionId>` sesi berakhir di luar 20 terakhir (urut `firstTouchAt` terbaru); cara tahu sesi berakhir: parameter daftar id sesi aktif dari pemanggil, tidak menambah dependensi baru.

Acceptance criteria:
- [ ] `Capture` dua kali pada path yang sama tidak menimpa baseline pertama.
- [ ] File baru dicatat `existedBefore=false`, tanpa salinan isi; `Files` mengembalikannya sebagai `added`.
- [ ] Path di luar cwd atau symlink keluar cwd ditolak/diabaikan tanpa panic dan tanpa menulis di luar DataDir.
- [ ] File biner dan > 1 MB masuk index tanpa salinan isi dengan flag yang benar.
- [ ] `Files` membuang entri yang isinya sama dengan baseline.
- [ ] `index.json` ditulis atomik; JSON rusak/tidak ada diperlakukan sebagai kosong tanpa panic.
- [ ] `go build ./...` lulus.

ownedPaths: `internal/review/types.go`, `internal/review/store.go`, `internal/review/capture.go`.
Dikerjakan lebih dulu; fitur lain bergantung pada tipenya.

### review-hooks-ipc: Hook edit, review.files, dan event review.changed

Deskripsi:
- `internal/status/install.go`: ubah `preToolUseMatcher` menjadi `"Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit"` dan perbarui komentarnya. Pastikan instalasi ulang menimpa matcher lama (hook yang sudah terpasang dengan `Workflow|Agent` harus diperbarui; periksa logika merge di `install.go` dan sesuaikan bila entri lama dipertahankan). `ClassifyEvent` dan `internal/runs` tidak boleh berubah perilaku: karena `PreToolUse` kini datang untuk tool edit, pastikan listener runs/status mengabaikan `tool_name` selain Workflow/Agent seperti sekarang (cek di `internal/runs/notifier.go` dan `ClassifyEvent` bahwa PreToolUse tool lain tidak mengubah status atau registry; bila perlu hanya tambah guard di sisi review, bukan di runs).
- `internal/status/hooks.go`: hanya bila perlu (mis. helper ekstraksi `tool_name`/`tool_input.file_path`); jangan ubah `HookPayload` yang ada kecuali menambah field opsional.
- `internal/review/ipc.go`: `RegisterIPC(srv, store, supportedFn)` mendaftarkan `review.files`. `supportedFn(sessionID) bool` menentukan dukungan provider (sesi dengan hooks, yaitu provider claude); false -> `{supported:false, files:[]}`.
- `cmd/shikigamid/wire_review.go`: `RegisterModule("review", ...)`: buat `review.NewStore(d.DataDir)`, `d.Provide("review.store", store)`, di `d.OnReady` ambil `status.engine`, daftarkan IPC dan `eng.AddHookListener`. Listener: `PreToolUse` + `tool_name` in {Edit, Write, MultiEdit, NotebookEdit} -> ekstrak path dari `tool_input` (`file_path`; untuk NotebookEdit coba `notebook_path` lalu `file_path`) -> `store.Capture` (jalankan cepat, log error pada level debug/warn, tidak pernah mengembalikan error ke hook); `PostToolUse` tool edit -> `store.MarkChanged`, lalu jadwalkan publikasi `review.changed {sessionId, count}` dengan debounce 300 ms per sesi (timer per sesi, dibersihkan saat sesi exit lewat `AddExitListener` bila ada). Count dihitung dari `store.Files` (belum ditinjau).
- Spike: tulis hasil verifikasi format `tool_input` MultiEdit dan NotebookEdit sebagai komentar di `wire_review.go` (kunci apa yang berisi path). Bila tidak ada path yang dikenali, abaikan event tanpa error.

Acceptance criteria:
- [ ] Hook baru terpasang untuk sesi baru dengan matcher `Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit`; matcher lama diganti, bukan diduplikasi.
- [ ] Status sesi (running/idle/waiting) dan registry runs tidak berubah akibat PreToolUse tool edit.
- [ ] Edit pertama pada file menyimpan baseline sebelum tool menulis (PreToolUse); edit kedua tidak menimpanya.
- [ ] `review.files` mengembalikan `[{path,status,reviewed,external:false,...}]`, tidak menyertakan file yang sudah sama dengan baseline, dan `supported:false` untuk provider tanpa hooks.
- [ ] `review.changed` terkirim satu kali per rentetan edit cepat (debounce 300 ms) dengan `count` benar.
- [ ] Hook tidak pernah memblokir atau gagal (selalu exit 0 di sisi `shikigamictl`; listener tidak panic).
- [ ] `go build ./...` lulus.

ownedPaths: `internal/review/ipc.go`, `cmd/shikigamid/wire_review.go`, `internal/status/install.go`, `internal/status/hooks.go`.
Paralel dengan `review-ui-list` setelah `review-foundation` selesai.

### review-ui-list: Tab Review dan badge sesi

Deskripsi:
- `types.ts`: tipe pada bagian Kontrak.
- `store.ts`: `reviewFiles` mengikuti sesi aktif (turunkan dari store sesi aktif seperti `runs/store.ts`), memanggil `call('review.files', { sessionId })`; subscribe `onEvent('review.changed', ...)` untuk memperbarui `reviewCounts` dan me-refresh daftar bila event untuk sesi aktif. Ganti sesi aktif memicu refresh. Tangani error IPC dengan state kosong (jangan melempar ke UI).
- `ReviewTab.svelte`: daftar file (nama file tebal, direktori redup, penanda status M/A/D, penanda "ditinjau"), toggle filter "belum ditinjau", keadaan kosong ("Belum ada file yang diubah agent di sesi ini") dan pesan "Provider sesi ini tidak mendukung review perubahan" bila `supported=false`. Pakai token CSS/tema yang sama dengan `FilesTab.svelte`/`ActivityTab.svelte`. Komponen harus menyisakan titik kait untuk tiket berikutnya: item daftar dapat dipilih (event `onselect(file)` disimpan di state; tiket 02 yang menampilkan DiffView).
- `ReviewBadge.svelte`: pil kecil dengan `reviewCounts[sessionId]`, disembunyikan bila 0, label aksesibilitas (`aria-label`).
- `index.ts`: `registerRightTab({ id: 'review', title: 'Review', order: 15, component: ReviewTab })` (setelah Files order 10, sebelum Activity 20), dan `export { ReviewBadge }`.

Acceptance criteria:
- [ ] Tab "Review" muncul di panel kanan setelah Files dan sebelum Activity.
- [ ] Daftar berubah live saat `review.changed` datang untuk sesi aktif; berganti isi saat ganti sesi aktif.
- [ ] Filter "belum ditinjau" menyembunyikan file `reviewed=true`.
- [ ] `supported=false` menampilkan pesan, bukan error.
- [ ] `ReviewBadge` tersembunyi saat count 0 dan menampilkan angka saat > 0.
- [ ] Build frontend lulus (termasuk `svelte-check` bila ada di script build).

ownedPaths: `frontend/src/modules/review/index.ts`, `ReviewTab.svelte`, `store.ts`, `types.ts`, `ReviewBadge.svelte` (semua di `frontend/src/modules/review/`).
Paralel dengan `review-hooks-ipc`.

### review-capture-wiring: Wiring tiket review-capture

Deskripsi: lihat bagian Wiring. Dikerjakan terakhir, setelah ketiga fitur lain selesai.

Acceptance criteria:
- [ ] `ReviewBadge` terpasang di `SessionRow.svelte` dengan perubahan minimal (satu import, satu elemen) dan tidak mengubah tata letak baris saat count 0.
- [ ] Tab Review dan badge terlihat saat agent mengedit file di aplikasi yang berjalan.
- [ ] `go build ./...` dan build frontend lulus.

ownedPaths: `frontend/src/modules/sidebar-left/SessionRow.svelte`, `frontend/src/main.ts`.
Bergantung pada ketiga fitur lain di epic ini.

## Path yang dimiliki

Hanya ini yang boleh disentuh:
- `internal/review/types.go`
- `internal/review/store.go`
- `internal/review/capture.go`
- `internal/review/ipc.go`
- `cmd/shikigamid/wire_review.go`
- `frontend/src/modules/review/index.ts`
- `frontend/src/modules/review/ReviewTab.svelte`
- `frontend/src/modules/review/store.ts`
- `frontend/src/modules/review/types.ts`
- `frontend/src/modules/review/ReviewBadge.svelte`
- Perluasan terbatas file tiket lama (diakui): `internal/status/install.go`, `internal/status/hooks.go`, `frontend/src/modules/sidebar-left/SessionRow.svelte`, `frontend/src/main.ts`.

## Wiring

Langkah penutup tiket ini (tidak bergantung tiket lain):
1. Modul daemon: pastikan `wire_review.go` terdaftar lewat `init()` + `RegisterModule("review", ...)` sehingga ikut terbangun di `cmd/shikigamid` tanpa mengubah `modules.go`.
2. Frontend: `modules/review/index.ts` sudah ter-load otomatis oleh glob di `main.ts`; verifikasi dan jangan menambah import bila glob sudah mencakup (ubah `main.ts` hanya jika tidak). Pastikan `registerRightTab` dijalankan.
3. Pasang `<ReviewBadge sessionId={session.id} />` di `SessionRow.svelte` (satu import dari `../review` dan satu elemen di dekat `StatusBadge`), tanpa menyentuh logika lain.
4. Pastikan hook ter-install ulang untuk sesi baru dengan matcher baru (sesi lama yang berjalan tidak ikut; cukup sesi yang dibuat setelah daemon baru). Jalankan aplikasi, buka sesi Claude, minta agent mengedit/membuat file, dan konfirmasi tab Review menampilkan file serta badge di SessionRow bertambah; konfirmasi sesi shell menampilkan pesan "provider tidak mendukung".
5. Pastikan di folder non-git alurnya tetap jalan.

## Guard

- `go build ./...`
- Build frontend (jalankan di `frontend/`: script build di `package.json`, mis. `npm run build`, termasuk check tipe bila skripnya ada).
- Tanpa unit test.
