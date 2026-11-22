# Overview: agent-diff-review-ticket-02-review-diff

Tiket: `docs/tickets/agent-diff-review-ticket-02-review-diff.md`. Spec: `docs/superpowers/specs/2026-11-22-agent-diff-review-design.md`. Depends on tiket 01 (`agent-diff-review-ticket-01-review-capture`, sudah ter-merge: `internal/review/{types,store,capture,ipc}.go`, `cmd/shikigamid/wire_review.go`, `frontend/src/modules/review/*`).

Setiap agent fitur hanya membaca dokumen ini dan tiketnya. Bagian 2 adalah kontrak; bila kode nyata berbeda dari uraian, ikuti kode tiket 01 dan catat di pesan commit.

## 1. Ringkasan dan urutan eksekusi

Epic menghitung diff (Myers, Go murni, tanpa dependency) antara baseline sesi dan isi file sekarang, mengisi `additions/deletions/binary/tooLarge` di `review.files`, menambah IPC `review.diff`, dan menampilkan unified diff per file di tab Review lewat `DiffView.svelte`. `DiffView` menyediakan slot `hunk-actions`, slot `footer`, dan prop `onLineClick` sebagai titik ekstensi tiket 03 dan 04 (dibiarkan kosong di epic ini).

Urutan: **tanpa Foundation** (`hasFoundation=false`; tidak ada `00-foundation.md`). Alasan: dua fitur menyentuh sisi berbeda (Go vs frontend) dengan path yang terpisah, dan seluruh tipe/antarmuka sudah dipatok di bagian 2. Lalu:

1. Fitur paralel: `review-diff-engine` (Go) dan `review-diff-ui` (frontend). UI memakai data tiruan sampai IPC siap.
2. Wiring (satu agen, setelah kedua fitur selesai): dua edit aditif Go + verifikasi (bagian 5).

## 2. Kontrak bersama

### 2.1 Fakta tiket 01 yang dipakai (dibaca dari kode)

- `Store` (internal/review/store.go): `Touched(sessionID) []TouchedFile` (salinan), `Cwd(sessionID) string`, `Files(sessionID, cwd) ([]FileEntry, error)`, `BaselinePath(sessionID, relPath) string`, `HashPath`, `SessionDir`. Helper privat berawalan `store`/`capture` (mis. `storeHashBytes`, `storeHashFile`, `storeValidID`, `captureSample`) ada di paket yang sama dan boleh dipakai ulang dari `diff.go`/`diff_ipc.go` (paket sama), tetapi jangan diubah.
- `TouchedFile` (JSON di index.json): `Path` (relatif cwd, `/`), `ExistedBefore`, `BaselineSha256`, `Binary`, `TooLarge`, dst. `Path` adalah kunci entri.
- Perilaku capture yang memengaruhi diff: bila baseline **biner** atau **> 1 MiB**, salinan baseline TIDAK ditulis (hanya hash + flag `Binary`/`TooLarge`). File baru (`ExistedBefore=false`) juga tanpa salinan. Maka `DiffForFile` wajib memeriksa flag entri lebih dulu dan tidak boleh gagal karena salinan baseline tidak ada pada kasus itu; salinan baseline yang hilang untuk file teks kecil yang `ExistedBefore` adalah error.
- `FileEntry` sudah punya field `Additions`, `Deletions`, `Binary`, `TooLarge` (JSON `additions`, `deletions`, `binary`, `tooLarge`); `Store.Files` mengisi `Binary`/`TooLarge` dari entri index, `Additions/Deletions` 0.
- `fsx.ResolveUnder(root, rel)` memvalidasi path relatif di dalam root (error `ErrInvalid` bila keluar). `fsx.IsBinary(sample)` (NUL atau bukan UTF-8 valid dalam 8 KiB pertama) adalah helper deteksi biner; pakai ulang, bukan implementasi baru.
- `RegisterIPC(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool)` di `ipc.go` mendaftarkan `review.files`; error memakai `ipc.NewError(ipc.ErrInvalid | ipc.ErrInvalidParams, msg)`; `srv.Register(method, func(ctx, params json.RawMessage) (any, error))`.
- `cmd/shikigamid/wire_review.go`: di `d.OnReady`, memanggil `review.RegisterIPC(srv, store, reviewWireSupported(eng))`.
- Frontend: `types.ts` (`ReviewFile`, `ReviewFilesResult`), `store.ts` (`reviewFiles`, `refreshReview`, `startReviewSync`; sesi aktif disimpan privat `activeSessionId`; event `review.changed` memicu `refreshReview`), `ReviewTab.svelte` (props `onselect?`, state `selected`, `onlyUnreviewed`, pola token CSS `var(--fg)`, `--fg-muted`, `--border`, `--bg-hover`, `--accent`, `--ok`, `--warn`, `--danger`), `index.ts` (registerRightTab).
- `openFilePane({projectId, filePath, workspaceId?, title?})` di `frontend/src/modules/shell/helpers.ts`; `filePath` adalah path **absolut**. Ia tidak menerima nomor baris. Jangan ubah modul shell/files; bila tidak ada mekanisme baris yang sudah ada, buka file saja.
- Tipe `Session` (`lib/stores/types.ts`) punya `id`, `projectId`, `cwd`, `workspaceId`. Path absolut file = `session.cwd` + `/` + path relatif (path relatif memakai `/`). `projectId` dan `workspaceId` diambil dari sesi aktif (store `sessions`).

### 2.2 Go: `internal/review/diff.go` (milik `review-diff-engine`)

```go
package review

const MaxDiffBytes = 1 << 20 // 1.048.576
const ContextLines = 3

type DiffLine struct {
    Kind  string `json:"kind"`  // "ctx" | "add" | "del"
    Text  string `json:"text"`  // tanpa newline akhir
    OldNo int    `json:"oldNo"` // 0 untuk add
    NewNo int    `json:"newNo"` // 0 untuk del
}
type Hunk struct {
    ID       string     `json:"id"` // 12 hex pertama sha256(oldStart,oldLines,newStart,newLines + isi baris)
    OldStart int        `json:"oldStart"`
    OldLines int        `json:"oldLines"`
    NewStart int        `json:"newStart"`
    NewLines int        `json:"newLines"`
    Lines    []DiffLine `json:"lines"`
}
type DiffResult struct {
    Hunks       []Hunk `json:"hunks"`       // selalu non-nil
    CurrentHash string `json:"currentHash"` // sha256 hex isi sekarang; "" bila file tidak ada
    Binary      bool   `json:"binary"`
    TooLarge    bool   `json:"tooLarge"`
    Additions   int    `json:"additions"`
    Deletions   int    `json:"deletions"`
}

func ComputeDiff(old, cur []byte) DiffResult                          // murni, nil = tidak ada
func DiffForFile(st *Store, sessionID, path string) (DiffResult, error)
```

Catatan kontrak:
- Tag JSON pada struct di atas adalah kontrak; `DiffResult` di-serialisasi langsung untuk `review.diff`, tetapi `additions`/`deletions` DIKECUALIKAN dari respons IPC (lihat 2.3; gunakan struct respons terpisah atau abaikan field itu di klien). Frontend tidak bergantung pada keduanya di `review.diff`.
- Semantik sisi kosong: file baru atau terhapus memberi `oldStart/oldLines` (atau `newStart/newLines`) = 0 dan satu hunk berisi semua baris add (atau del). `ComputeDiff(x, x)` tanpa hunk. Berbasis 1 untuk sisi tidak kosong.
- Hunk digabung bila jarak perubahan <= `2*ContextLines`; konteks maksimum 3 baris. Baris tanpa newline akhir diperlakukan biasa.
- Myers dengan batas biaya D; terlampaui = fallback semua del lalu semua add (satu hunk).
- `DiffForFile`: ambil entri dari `st.Touched` (kunci `Path`) dan cwd dari `st.Cwd`; sesi/entri tidak dikenal atau cwd kosong = error jelas; path divalidasi lewat `fsx.ResolveUnder(cwd, path)`; flag `Binary`/`TooLarge` entri atau hasil Stat/sampel isi sekarang menentukan hasil tanpa hunk, tetap mengisi `CurrentHash` (streaming; `storeHashFile`). `ExistedBefore=false` = `old=nil`; file sekarang tidak ada = `cur=nil`.

### 2.3 Go: `internal/review/diff_ipc.go` (milik `review-diff-engine`)

```go
const IPCMethodDiff = "review.diff" // didefinisikan di diff_ipc.go, bukan types.go

// RegisterDiffIPC mendaftarkan review.diff. Sama seperti RegisterIPC: sesi tidak didukung ->
// hasil kosong (hunks:[], currentHash:"", binary:false, tooLarge:false), bukan error.
func RegisterDiffIPC(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool)

// FillDiffStats mengisi Additions, Deletions, Binary, TooLarge pada files IN PLACE (index slice).
// Binary/TooLarge di-OR dengan nilai yang sudah ada pada entri. Galat satu file diabaikan
// (file itu tetap 0/0). Cache memori kunci (sessionID, path, currentHash). Aman dipanggil
// dengan files nil/kosong. Tidak mengembalikan error.
func FillDiffStats(st *Store, sessionID string, files []FileEntry)
```

IPC `review.diff` params `{sessionId: string, path: string}`; hasil:
```json
{"hunks":[{"id":"","oldStart":0,"oldLines":0,"newStart":0,"newLines":0,"lines":[{"kind":"ctx|add|del","text":"","oldNo":0,"newNo":0}]}],
 "currentHash":"","binary":false,"tooLarge":false}
```
`hunks` selalu array. Error: params tidak valid -> `ipc.ErrInvalidParams`; `sessionId`/`path` kosong, path di luar cwd, sesi/entri tak dikenal -> `ipc.ErrInvalid` dengan pesan jelas Indonesia.

`review.files` (diperluas): handler di `ipc.go` memanggil `FillDiffStats(store, id, files)` sebelum mengembalikan hasil (edit aditif di Wiring).

### 2.4 Frontend: `frontend/src/modules/review/diff.ts` (milik `review-diff-ui`)

```ts
export type DiffLineKind = 'ctx' | 'add' | 'del';
export interface DiffLine { kind: DiffLineKind; text: string; oldNo: number; newNo: number }
export interface Hunk { id: string; oldStart: number; oldLines: number; newStart: number; newLines: number; lines: DiffLine[] }
export interface DiffResult { hunks: Hunk[]; currentHash: string; binary: boolean; tooLarge: boolean }
export function fetchDiff(sessionId: string, path: string): Promise<DiffResult>  // call('review.diff', {sessionId, path}), normalisasi defensif (hunks selalu array)
export function hunkSummary(hunk: Hunk): string  // mis. "+3 -1"; helper murni
```
Tipe ini juga dipakai tiket 03/04; jangan ganti nama.

### 2.5 Frontend: `DiffView.svelte` (milik `review-diff-ui`)

Svelte 5 (proyek memakai `$props`/`$state`/`$derived`; lihat ReviewTab). Props: `{ diff: DiffResult; path: string; onLineClick?: (line: DiffLine, hunk: Hunk) => void; hunkActions?; footer? }`. Kontrak ekstensi publik dari tiket ditulis sebagai "slot `hunk-actions` (let:hunk)" dan "slot `footer`". Karena kode memakai runes, implementasikan sebagai snippet props bernama `hunkActions: Snippet<[Hunk]>` dan `footer: Snippet`, dan dokumentasikan di komentar kepala komponen bahwa nama slot publik tetap `hunk-actions` dan `footer`. Keduanya opsional dan tidak dirender (tanpa elemen pembungkus yang mengubah tampilan) bila tidak diberikan. Tiket 03/04 mengisi lewat snippet itu. Jangan mengganti nama setelah dipublikasikan.

Bentuk visual dan status: header hunk `@@ -a,b +c,d @@` dengan slot aksi di sisi kanan; dua kolom nomor; penanda `+`/`-`/spasi; `white-space: pre`, scroll horizontal di blok diff; warna add/del memakai token yang sudah ada (`--ok`, `--danger`, `--fg-muted`, `--border` + `color-mix` transparan), tanpa palet baru; pesan status: biner "File biner, tidak ada diff", terlalu besar "File terlalu besar (> 1 MB), diff tidak ditampilkan", kosong "Tidak ada perubahan terhadap baseline".

### 2.6 Frontend: perubahan `ReviewTab.svelte` (milik `review-diff-ui`)

Ringkasan `+N -M` dan penanda "biner"/"terlalu besar" per baris (field sudah ada di `ReviewFile`); state `selectedPath`; `DiffView` menggantikan daftar saat file dipilih (header: tombol kembali, path, tombol "Buka file"); tombol ikon "Buka file" di baris daftar dengan `stopPropagation`; `fetchDiff` saat dipilih dan diulang saat daftar file berubah (daftar `reviewFiles` diperbarui oleh event `review.changed`); abaikan respons basi (token urutan); kembali ke daftar bila sesi berganti atau file hilang dari daftar; state loading dan error inline dengan tombol coba lagi. Mempertahankan filter `onlyUnreviewed` dan scroll daftar saat kembali (jangan unmount daftar secara destruktif, mis. sembunyikan dengan `hidden`/CSS atau simpan `scrollTop`). Jangan mengubah `store.ts`/`types.ts`/`index.ts` tiket 01. Bila perlu id sesi aktif di UI, turunkan dari store yang sudah ada (mis. `layout`/`sessions`) di dalam `ReviewTab.svelte` atau `diff.ts`; jangan mengekspor yang baru dari `store.ts`.

### 2.7 Konvensi

- Go: paket `review`, fungsi publik baru hanya yang tercantum; helper privat berawalan `diff` (mis. `diffMyers`, `diffGroupHunks`, `diffHunkID`) supaya tidak bentrok dengan helper `store*`/`capture*`. Komentar kode bahasa Indonesia. Module Go: `github.com/NooBeeID/shikigami`. Tanpa dependency baru.
- Frontend: TypeScript strict, Svelte 5 runes, teks UI bahasa Indonesia, identifier bahasa Inggris. Tanpa dependency npm baru.
- Nama field JSON camelCase persis seperti di atas.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `review-diff-engine` | Mesin diff Myers + hunk + id stabil + deteksi biner/terlalu besar + `CurrentHash`, `DiffForFile`, IPC `review.diff` (`RegisterDiffIPC`), `FillDiffStats` dengan cache | `internal/review/diff.go`, `internal/review/diff_ipc.go` |
| `review-diff-ui` | `diff.ts`, `DiffView.svelte` (slot/snippet aksi hunk, footer, `onLineClick`), perluasan `ReviewTab.svelte` (ringkasan +/-, penanda, buka file, tampilan diff) | `frontend/src/modules/review/DiffView.svelte`, `frontend/src/modules/review/diff.ts`, `frontend/src/modules/review/ReviewTab.svelte` (hanya penyisipan DiffView, ringkasan, tombol buka file) |

Plan masing-masing: `docs/plans/agent-diff-review-ticket-02-review-diff/review-diff-engine.md` dan `review-diff-ui.md`. Wiring: `review-diff-wiring.md` (opsional; checklist ada di bagian 5).

## 4. Foundation

Tidak ada. Tidak ada `00-foundation.md`. Kedua fitur bergantung hanya pada kontrak tertulis di bagian 2, bukan pada file satu sama lain.

## 5. Wiring checklist

Dikerjakan SETELAH kedua fitur selesai dan ter-merge. Hanya edit aditif minimal; jangan ubah perilaku lain.

1. `internal/review/ipc.go` (milik tiket 01, edit aditif): di handler `review.files`, setelah `files` dihitung dan dinormalisasi non-nil, tepat sebelum `return FilesResult{Supported: true, Files: files}`, tambahkan satu panggilan `FillDiffStats(store, id, files)`. Tidak ada perubahan lain.
2. `cmd/shikigamid/wire_review.go`: tepat setelah `review.RegisterIPC(srv, store, reviewWireSupported(eng))`, tambahkan `review.RegisterDiffIPC(srv, store, reviewWireSupported(eng))`.
3. Verifikasi bahwa `ReviewTab.svelte` memakai `fetchDiff` dan `DiffView` dengan bentuk kontrak 2.4 dan 2.5; perbaiki ketidakcocokan bentuk JSON kecil di sisi UI/engine seperlunya bila kedua fitur menyimpang dari kontrak (catat di pesan commit).
4. Pastikan slot/snippet `hunk-actions` dan `footer` tidak diisi (kosong), dan `onLineClick` tidak diberikan.
5. Jalankan Build guard (bagian 6). Verifikasi manual (jika memungkinkan menjalankan aplikasi): sesi Claude mengedit file yang sama dua kali -> satu diff terhadap baseline pertama dan `+/-` terisi; file baru (semua tambah); file biner (pesan); tombol "Buka file" tidak membuka diff.
6. Pesan commit mencatat dua edit aditif Go di atas.

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` dari root repo.
- `cd frontend && npm run build` (Vite; skrip `build` = `vite build`). Opsional tambahan: `npm run check` (svelte-check) bila hijau di baseline; jangan memperbaiki galat di luar ownedPaths.

Fitur `review-diff-engine` menjalankan `go build ./...`; `review-diff-ui` menjalankan `npm run build` di `frontend/`. Wiring menjalankan keduanya.

## 7. Aturan paralel

- Tiap agen hanya menyentuh `ownedPaths` fiturnya. Agen engine tidak menyentuh frontend; agen UI tidak menyentuh Go.
- Dilarang menyentuh `internal/review/{types,store,capture,ipc}.go`, `cmd/shikigamid/wire_review.go`, `frontend/src/modules/review/{store,types,index}.ts` dan `ReviewBadge.svelte`, serta modul shell/files/layout, `go.mod`, `package.json`. Dua edit aditif Go (ipc.go, wire_review.go) hanya dilakukan agen Wiring.
- Tidak menambah dependency, tidak menulis unit test, tidak menjalankan `git commit` (orkestrator yang menangani).
- Engine: sampai Wiring, `FillDiffStats`/`RegisterDiffIPC` berkompilasi walau belum dipanggil (fungsi publik, bukan error unused). UI: gunakan data tiruan lokal (di dalam berkas milik sendiri atau sementara) bila IPC belum siap, dan hapus data tiruan sebelum selesai.
- Bila kontrak terasa kurang, ikuti tiket dan kode tiket 01, lalu catat penyimpangan di laporan; jangan mengubah file fitur lain.
