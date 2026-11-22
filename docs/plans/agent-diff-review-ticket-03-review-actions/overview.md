# Overview: agent-diff-review-ticket-03-review-actions

Tiket: `docs/tickets/agent-diff-review-ticket-03-review-actions.md`. Spec: `docs/superpowers/specs/2026-11-22-agent-diff-review-design.md`. Bergantung pada tiket 01 (`review-capture`) dan 02 (`review-diff`), keduanya sudah ter-merge: `internal/review/{types,store,capture,ipc,diff,diff_ipc}.go`, `cmd/shikigamid/wire_review.go`, `frontend/src/modules/review/{DiffView.svelte,ReviewTab.svelte,ReviewBadge.svelte,diff.ts,store.ts,types.ts,index.ts}`.

Setiap agent fitur hanya membaca dokumen ini dan tiketnya. Bagian 2 adalah kontrak. Bila kode nyata berbeda dari uraian, ikuti kode yang ada dan catat penyimpangan di laporan/pesan commit.

## 1. Ringkasan dan urutan eksekusi

Epic menambah aksi pada hasil review: Revert (kembalikan ke baseline sesi) dan Terima (baseline maju ke isi sekarang) per hunk dan per file, pengaman `[CONFLICT]` (hash isi berubah sejak diff) dan `[EXTERNAL]` (file diubah di luar agent), flag `external` pada `review.files`, event `review.changed` setelah aksi, serta UI `HunkActions.svelte` dan navigasi keyboard `j/k/a/r`.

Urutan: **tanpa Foundation** (`hasFoundation=false`; tidak ada `00-foundation.md`). Alasan: sisi Go dan frontend terpisah total, dan semua tipe/antarmuka sudah dipatok di bagian 2 (tidak ada paket baru dan tidak ada tipe bersama yang harus ada lebih dulu). Lalu:

1. Fitur paralel: `review-actions-daemon` (Go) dan `review-actions-ui` (frontend). UI memakai data tiruan lokal sampai IPC siap.
2. Wiring (satu agen, setelah kedua fitur selesai): tiga edit aditif Go + verifikasi (bagian 5).

## 2. Kontrak bersama

### 2.1 Fakta kode yang ada (dibaca dari kode, bukan asumsi)

Go, paket `review` (modul `github.com/NooBeeID/shikigami`):
- `types.go`: `TouchedFile{Path, ExistedBefore, BaselineSha256, FirstTouchAt, Reviewed, ExpectedHash, Binary, TooLarge}`; `HashAbsent = "absent"` (nilai `ExpectedHash` untuk file yang tidak ada setelah edit agent); `EventChanged = "review.changed"`; `FileEntry` sudah punya `External bool` (`json:"external"`, saat ini selalu false); `Status` (`modified|added|deleted`).
- `store.go`: `Store` dengan `storeLock(id) func()` (pakai `defer s.storeLock(id)()`), `storeLoad(id) storeIndex` dan `storeSave(id, storeIndex) error` (KEDUANYA TIDAK mengunci; pemanggil memegang lock), `storeIndex{Cwd, Files []TouchedFile}`, `storeHashBytes`, `storeHashFile`, `storeValidID`, `Cwd(id)`, `Touched(id)` (MENGUNCI; jangan dipanggil saat sudah memegang lock sesi, deadlock), `Files(id, cwd)`, `BaselinePath(id, rel)`, `SessionDir(id)`. `Files` menyembunyikan entri yang isinya identik dengan baseline (`default: continue`) tetapi TIDAK menghapusnya dari index; penghapusan entri adalah tugas aksi di tiket ini.
- `capture.go`: `captureWriteAtomic(dst, data)` (tmp + rename, mode 0600; BUKAN untuk file pengguna karena tidak mempertahankan mode), `captureSample`, `captureRel`. Baseline TIDAK disalin bila file baru (`ExistedBefore=false`), biner, atau > 1 MiB (`maxBaselineBytes`); untuk kasus itu hanya hash + flag `Binary`/`TooLarge` yang tersimpan.
- `diff.go`: `ComputeDiff(old, cur []byte) DiffResult` (murni; `nil` = tidak ada; hunk `Hunk{ID,OldStart,OldLines,NewStart,NewLines,Lines []DiffLine}`, `DiffLine{Kind ctx|add|del, Text, OldNo, NewNo}`; `ID` = 12 hex pertama sha256 isi hunk, deterministik untuk pasangan (old, cur) yang sama), `DiffForFile(st, id, path) (DiffResult, error)`, `MaxDiffBytes`. Semantik sisi kosong: oldStart/oldLines = 0 untuk file baru (satu hunk semua `add`), newStart/newLines = 0 untuk file terhapus. Teks baris tanpa newline akhir. Pakai fungsi ini; jangan menulis ulang perhitungan hunk. Helper privat berawalan `diff`.
- `ipc.go`: `RegisterIPC(srv, store, supportedFn)` mendaftarkan `review.files`; handler memanggil `FillDiffStats(store, id, files)` tepat sebelum return. `diff_ipc.go`: `RegisterDiffIPC(srv, store, supportedFn)`; `diffResponse` tanpa additions/deletions; error memakai `ipc.NewError(ipc.ErrInvalid | ipc.ErrInvalidParams, msg)`; param di-trim.
- `fsx.ResolveUnder(cwd, rel)` memvalidasi path (termasuk symlink) dan mengembalikan path absolut; `fsx.IsBinary(sample)`.
- `cmd/shikigamid/wire_review.go`: di `d.OnReady` memanggil `review.RegisterIPC(...)` lalu `review.RegisterDiffIPC(srv, store, reviewWireSupported(eng))`. Debouncer 300 ms dan publikasi `review.changed` untuk hook ada DI DALAM closure itu dan tidak dapat dipakai ulang; karena itu aksi tiket ini menerbitkan event sendiri (lihat 2.2).

Frontend (Svelte 5 runes, TS strict):
- `diff.ts`: `DiffLine`, `Hunk`, `DiffResult{hunks,currentHash,binary,tooLarge}`, `fetchDiff(sessionId, path)`, `hunkSummary`. `store.ts` mengekspor `reviewFiles`, `reviewCounts`, `refreshReview(sessionId)` (memuat ulang `review.files`, memperbarui badge), dan event `review.changed` sudah memicu `refreshReview` untuk sesi aktif. `ReviewFile` sudah punya `external`.
- `DiffView.svelte` props: `{diff, path, onLineClick?, hunkActions?: Snippet<[Hunk]>, footer?: Snippet}`. `hunkActions` dirender di kanan header hunk (`.hunk-head .actions`).
- `ReviewTab.svelte`: state `sessionId` (sesi aktif turunan layout), `selectedPath`, `diffData`, `loadDiff({silent?})`, `backToList()`, `selectedFile`, header `.dhead` (tombol kembali, path, `+/-`, tombol buka file), `DiffView` dirender bila `diffData`. Efek yang ada: daftar file berubah -> `loadDiff({silent:true})`, file hilang -> `backToList()`.
- Pola konfirmasi inline: tombol berubah menjadi "Yakin?" (kelas `danger`) dan "Batal" tanpa modal (lihat `modules/workspace/ProjectList.svelte`, `confirmId`). Pola shortcut global: `modules/layout/shortcuts.ts` memasang `keydown` capture di `window` (kombinasi bermodifier); `j/k/a/r` polos hanya aktif saat fokus di panel diff sehingga tidak bentrok.

### 2.2 IPC (milik `review-actions-daemon`)

```
review.revert {sessionId, path, hunkIds?: string[], expectHash: string, force?: boolean} -> {ok: true, removed: boolean}
review.accept {sessionId, path, hunkIds?: string[]}                                      -> {ok: true, removed: boolean}
```
- `removed=true` bila file tidak lagi muncul di `review.files` (entri dibuang dari index).
- `hunkIds` kosong/absen = seluruh file.
- `expectHash`: sha256 hex isi file saat diff dihitung; `""` bila file tidak ada (sama dengan `DiffResult.currentHash`).
- Error (semua lewat `ipc.NewError(ipc.ErrInvalid, ...)`, pesan Indonesia, awalan tag persis):
  - `"[CONFLICT] file berubah sejak diff dihitung, muat ulang"` bila hash isi sekarang != `expectHash`.
  - `"[EXTERNAL] file diubah di luar agent, butuh konfirmasi (force)"` bila file `external` dan `force != true`. Urutan cek: validasi param -> path -> `[CONFLICT]` -> `[EXTERNAL]` -> terapkan.
  - Sesi tidak didukung / sesi atau entri tak dikenal / path di luar cwd / `hunkIds` berisi id tak dikenal / `hunkIds` terisi untuk file `Binary` atau `TooLarge` -> `ErrInvalid` dengan pesan jelas. Params rusak -> `ErrInvalidParams`.
- Event: setelah tiap aksi sukses, hitung `store.Files(id, cwd)`, hitung file `Reviewed == false` (rumus sama dengan `wire_review.go`), lalu `srv.Publish(EventChanged, map[string]any{"sessionId": id, "count": n})` (langsung, tanpa debounce).

### 2.3 API Go (milik `review-actions-daemon`)

```go
// actions.go
func (s *Store) Revert(sessionID, path string, hunkIDs []string, expectHash string, force bool) (removed bool, err error)
func (s *Store) Accept(sessionID, path string, hunkIDs []string) (removed bool, err error)
func (s *Store) IsExternal(sessionID, path string) bool
func FillExternal(st *Store, sessionID string, files []FileEntry) // isi files[i].External in place; aman untuk nil/kosong
// actions_ipc.go
const IPCMethodRevert = "review.revert"
const IPCMethodAccept = "review.accept"
func RegisterActions(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool)
```
Catatan: ticket menulis `Revert(...)`/`Accept(...)`/`IsExternal(...)` tanpa receiver; kontrak final adalah METODE pada `*Store` (agar memakai `storeLock`/`storeLoad`/`storeSave` privat). `FillExternal` ditambahkan agar wiring `ipc.go` cukup satu baris.

Aturan implementasi:
- Semua aksi memegang `defer s.storeLock(sessionID)()` selama baca-ubah-tulis (file + index). Di dalamnya JANGAN memanggil `Touched`, `Cwd`, `Files` (mengunci ulang); pakai `storeLoad` langsung. `IsExternal` dan `FillExternal` boleh memakai `Touched`/`Cwd` (di luar lock aksi).
- `IsExternal`: entri harus ada; `ExpectedHash == ""` (belum ada PostToolUse) -> false; hash sekarang = sha256 isi, atau `HashAbsent` bila file tidak ada; `external = hashSekarang != ExpectedHash`. Catatan: tiket menyebut hash kosong untuk file hilang; kode nyata memakai `HashAbsent`, ikuti kode.
- Penulisan file pengguna atomik: tulis ke berkas sementara di direktori yang sama (mis. `.<nama>.review-tmp-<pid>`), pertahankan mode file asli (`os.Stat` lalu `Chmod`; file baru 0644), lalu `os.Rename`. Buat direktori induk (`MkdirAll`, 0o755) untuk file `deleted` yang dibuat ulang. Jangan memakai `captureWriteAtomic` untuk file pengguna.
- Revert penuh: `modified` -> tulis isi baseline (`BaselinePath`); `added` (`!ExistedBefore`) -> `os.Remove` (abaikan not-exist); `deleted` -> buat ulang dari baseline. Lalu buang entri dari index, hapus berkas baseline, simpan index, return `removed=true`. File `Binary`/`TooLarge` yang `ExistedBefore` tidak punya salinan baseline: Revert penuh -> error jelas "baseline tidak tersimpan untuk file biner/terlalu besar" (celah dari tiket 01, jangan diperbaiki di sini); Revert penuh file `added` tetap boleh (hapus file). Accept penuh pada file `Binary`/`TooLarge` diperbolehkan (entri dibuang tanpa menyalin).
- Revert sebagian: `old` = isi baseline (nil bila `!ExistedBefore`), `cur` = isi sekarang (nil bila tidak ada); `ComputeDiff(old, cur)`; cocokkan `hunkIDs` ke `Hunk.ID`; balik-terapkan dari hunk paling bawah ke atas (ganti baris `newStart..newStart+newLines-1` pada isi sekarang dengan baris `del`+`ctx` lama hunk itu; untuk `newLines==0` sisipkan di posisi `newStart` per semantik sisi kosong). Pertahankan penanda akhir baris (newline akhir) apa adanya; pecah baris sama dengan `diffSplitLines`. Baseline tidak berubah. Setelah menulis, `ExpectedHash` <- hash isi baru (aksi pengguna sendiri tidak boleh ditandai `external`). Bila hasil identik baseline -> perlakukan seperti revert penuh (entri hilang, `removed=true`): untuk `ExistedBefore` bandingkan hash hasil dengan `BaselineSha256`; untuk `added` (tanpa baseline) bila seluruh hunk direvert, hapus file seperti revert penuh.
- Accept penuh: baseline <- isi sekarang. Karena identik, entri dibuang dari index dan berkas baseline dihapus (tidak perlu menulis baseline baru). `removed=true`. `Reviewed=true` tidak perlu disimpan karena entri dibuang.
- Accept sebagian: terapkan MAJU hunk terpilih pada isi baseline (hasil = baseline baru; tulis ke `BaselinePath` dengan `captureWriteAtomic`, perbarui `BaselineSha256`, set `ExistedBefore=true` bila sebelumnya false). Baris yang bukan hunk terpilih tetap seperti baseline. `ExpectedHash` tidak diubah. `Reviewed` tidak diubah (masih ada hunk belum ditinjau). Entri dibuang bila baseline baru identik isi sekarang (`removed=true`). Accept sebagian pada file `Binary`/`TooLarge` -> error.
- File yang hilang dan `ExistedBefore=false` pada Accept: entri dibuang (tidak ada yang diterima). Entri dibuang pula pada Accept penuh `deleted`.
- `review.files` dengan entri yang identik baseline sudah tersaring oleh `Files`; flag `external` diisi `FillExternal` (wiring di `ipc.go`, bagian 5 butir 1).

### 2.4 Frontend: kontrak komponen

`HunkActions.svelte` (milik `review-actions-ui`):
```ts
// <script lang="ts" module> mengekspor:
export interface ReviewActionResult { removed: boolean; conflict?: boolean }
// props ($props):
{
  sessionId: string;              // DITAMBAHKAN dari kontrak tiket (komponen perlu id sesi untuk IPC)
  path: string;                   // path relatif file
  currentHash: string;            // dari DiffResult.currentHash yang sedang tampil
  external: boolean;
  scope: 'hunk' | 'file';
  hunk?: Hunk;                    // wajib bila scope === 'hunk'; diabaikan untuk 'file'
  onDone: (r: ReviewActionResult) => void;
}
// metode publik (export function), dipakai keys.ts lewat bind:this:
//   accept(): Promise<void>; startRevert(): void; confirmRevert(): Promise<void>; cancel(): void; get pending(): boolean
```
- Panggilan IPC langsung lewat `call` dari `../../lib/ipc`: `review.revert {sessionId, path, hunkIds: scope==='hunk' ? [hunk.id] : undefined, expectHash: currentHash, force}` dan `review.accept {sessionId, path, hunkIds?}`. `force=true` bila `external`, atau setelah error `[EXTERNAL]` dikonfirmasi keras.
- `[CONFLICT]`: tampilkan "File berubah sejak diff dihitung, dimuat ulang", panggil `onDone({removed:false, conflict:true})` (induk memuat ulang diff dan daftar; tidak ada aksi lanjutan otomatis). `[EXTERNAL]`: tampilkan konfirmasi keras (peringatan "File ini diubah di luar agent; revert akan menimpa perubahan itu") lalu kirim ulang dengan `force:true` setelah dikonfirmasi. Error lain: pesan singkat inline. Tombol nonaktif selama panggilan (`busy`). Tidak ada `unhandled rejection`.

`keys.ts` (milik `review-actions-ui`):
```ts
export interface ReviewKeyHandlers {
  target: HTMLElement;     // panel diff tempat listener dipasang (hanya aktif bila fokus/target di dalamnya)
  next(): void;            // j
  prev(): void;            // k
  accept(): void;          // a
  revert(): void;          // r (induk: bila konfirmasi sedang terbuka -> konfirmasi, jika tidak -> mulai)
  confirm(): void;         // Enter bila konfirmasi terbuka
  cancel(): void;          // Esc bila konfirmasi terbuka
}
export function attachReviewKeys(handlers: ReviewKeyHandlers): () => void  // mengembalikan fungsi lepas
```
- Abaikan event bila `e.target` adalah `input`/`textarea`/`select`/`contenteditable`, atau ada `ctrl/meta/alt`, atau `e.defaultPrevented`. `preventDefault` hanya untuk tombol yang ditangani. Tombol `Enter`/`Esc` hanya ditangani bila `handlers` melaporkan konfirmasi terbuka (induk yang menentukan; sediakan `isConfirming?: () => boolean` opsional pada `ReviewKeyHandlers`).

Sisipan di `DiffView.svelte` (aditif, tanpa mengubah perilaku lain): prop opsional `selectedHunkId?: string`; hunk dengan id itu diberi kelas `selected` (sorot memakai `color-mix` token yang ada), atribut `data-hunk-id` pada `.hunk-head` agar `scrollIntoView({block:'nearest'})` dapat dipanggil dari `ReviewTab`. `hunkActions` diisi oleh `ReviewTab`, bukan oleh `DiffView`.

Sisipan di `ReviewTab.svelte`: state `selectedHunkIndex`, registri instance `HunkActions` per hunk id (`bind:this`), `HunkActions scope='file'` di `.dhead`, `HunkActions scope='hunk'` lewat snippet `hunkActions`, badge `external` ("perubahan lain") di baris daftar dan di `.dhead`, `attachReviewKeys` pada elemen panel `.diffpane` (dibuat `tabindex="0"`; dilepas saat unmount/`$effect` cleanup), `onDone` = `loadDiff()` + `refreshReview(sessionId)`; bila `removed` pindah ke file terlihat berikutnya, atau `backToList()` bila tidak ada.

### 2.5 Konvensi

- Go: paket `review`; fungsi publik baru hanya yang tercantum di 2.3; helper privat berawalan `actions` (mis. `actionsApplyReverse`, `actionsApplyForward`, `actionsWriteFile`) agar tidak bentrok dengan `store*`/`capture*`/`diff*`. Komentar kode bahasa Indonesia. Tanpa dependency baru, tanpa unit test.
- Frontend: TypeScript strict, Svelte 5 runes, teks UI bahasa Indonesia, identifier bahasa Inggris, token tema yang ada (`--fg`, `--fg-muted`, `--border`, `--bg-hover`, `--accent`, `--ok`, `--warn`, `--danger`), tanpa dependency npm baru, tanpa modal baru.
- JSON camelCase persis seperti di atas.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `review-actions-daemon` | `Revert`/`Accept`/`IsExternal`/`FillExternal` pada `*Store` (balik/maju-terapkan hunk, penulisan atomik mempertahankan mode, mutex per sesi), IPC `review.revert` dan `review.accept` (`RegisterActions`), pengecekan `[CONFLICT]`/`[EXTERNAL]`, publikasi `review.changed` | `internal/review/actions.go`, `internal/review/actions_ipc.go` |
| `review-actions-ui` | `HunkActions.svelte` (Revert/Terima, konfirmasi inline, penanganan error), `keys.ts` (`j/k/a/r`), sisipan di `DiffView.svelte` (sorot hunk terpilih) dan `ReviewTab.svelte` (aksi file/hunk, badge `external`, pasang keys, refresh setelah aksi) | `frontend/src/modules/review/HunkActions.svelte`, `frontend/src/modules/review/keys.ts`, `frontend/src/modules/review/DiffView.svelte` (hanya sisipan), `frontend/src/modules/review/ReviewTab.svelte` (hanya sisipan) |

Plan masing-masing: `docs/plans/agent-diff-review-ticket-03-review-actions/review-actions-daemon.md` dan `review-actions-ui.md`. Wiring: `review-actions-wiring.md` (opsional; checklist ada di bagian 5).

## 4. Foundation

Tidak ada, sehingga tidak ada `00-foundation.md` (`hasFoundation=false`). Tidak ada paket baru, tipe, atau berkas skeleton yang perlu ada lebih dulu: paket `review` dan modul frontend `review` sudah ada, dan kedua fitur hanya bergantung pada kontrak tertulis di bagian 2, bukan pada berkas satu sama lain.

## 5. Wiring checklist

Dikerjakan SETELAH kedua fitur selesai dan ter-merge. Hanya edit aditif minimal; jangan mengubah perilaku lain.

1. `internal/review/ipc.go` (milik tiket 01, edit aditif): di handler `review.files`, tepat setelah baris `FillDiffStats(store, id, files)`, tambahkan satu baris `FillExternal(store, id, files)`.
2. `cmd/shikigamid/wire_review.go`: tepat setelah `review.RegisterDiffIPC(srv, store, reviewWireSupported(eng))`, tambahkan `review.RegisterActions(srv, store, reviewWireSupported(eng))`.
3. Verifikasi bentuk kontrak: `HunkActions` memanggil `review.revert`/`review.accept` dengan param persis 2.2; nama field respons `ok`/`removed`; `ReviewTab` memakai `HunkActions` dengan props 2.4 (termasuk `sessionId`) dan `attachReviewKeys` dilepas saat unmount. Perbaiki ketidakcocokan kecil di sisi UI/daemon bila ada (catat di pesan commit).
4. Pastikan setelah aksi daftar file dan badge turun: event `review.changed` dari `RegisterActions` diterima `store.ts` (sudah ada pendengar) dan `onDone` memanggil `refreshReview`. Pastikan `ReviewBadge` di `SessionRow` ikut turun.
5. Jalankan Build guard (bagian 6). Verifikasi manual bila aplikasi dapat dijalankan: sesi Claude mengedit file, Revert satu hunk dari tiga hunk, Revert file `added` (file terhapus) dan `deleted` (dibuat ulang), Terima hunk dan Terima file, `[CONFLICT]` (edit file dari luar saat diff terbuka lalu Revert), file `external` (edit dari luar lalu muat ulang daftar: badge "perubahan lain" muncul; Revert minta konfirmasi keras dan memakai `force`), keyboard `j/k/a/r` dan tidak aktif saat mengetik, folder non-git.
6. Pesan commit mencatat dua edit aditif Go (`ipc.go`, `wire_review.go`) dan penyimpangan kontrak bila ada.

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` dari root repo.
- `cd frontend && npm run build` (Vite). Opsional: `npm run check` (svelte-check) bila hijau di baseline; jangan memperbaiki galat di luar ownedPaths.

`review-actions-daemon` menjalankan `go build ./...`; `review-actions-ui` menjalankan `npm run build` di `frontend/`; Wiring menjalankan keduanya.

## 7. Aturan paralel

- Tiap agen hanya menyentuh `ownedPaths` fiturnya. Agen daemon tidak menyentuh frontend; agen UI tidak menyentuh Go.
- Dilarang menyentuh: `internal/review/{types,store,capture,ipc,diff,diff_ipc}.go`, `cmd/shikigamid/wire_review.go`, `frontend/src/modules/review/{store,types,diff,index}.ts`, `ReviewBadge.svelte`, modul shell/files/layout/workspace, `go.mod`, `package.json`. Edit aditif di `ipc.go` dan `wire_review.go` hanya dilakukan agen Wiring. Pengecualian pada `DiffView.svelte` dan `ReviewTab.svelte` terbatas pada sisipan di 2.4; jangan mengubah perilaku, markup, atau gaya yang sudah ada.
- Tidak menambah dependency, tidak menulis unit test, tidak menjalankan `git commit` (orkestrator yang menangani).
- Daemon: sampai Wiring, `RegisterActions` dan `FillExternal` berkompilasi walau belum dipanggil (fungsi publik). UI: gunakan data tiruan lokal sementara bila IPC belum siap dan hapus sebelum selesai; panggilan IPC gagal sampai daemon ter-wiring, itu wajar.
- Bila kontrak terasa kurang, ikuti tiket dan kode tiket 01/02, catat penyimpangan di laporan, jangan mengubah berkas fitur lain.
