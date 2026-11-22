---
id: agent-diff-review-ticket-02-review-diff
title: Review: diff unified per file
spec: docs/superpowers/specs/2026-11-22-agent-diff-review-design.md
status: ready
wave: 2
depends_on: [agent-diff-review-ticket-01-review-capture]
---
# Review: diff unified per file

## Tujuan
Tiket 01 menangkap baseline per file dan menampilkan daftar file yang diubah agent, tetapi pengguna belum bisa melihat isi perubahannya. Epic ini menghitung diff antara baseline sesi dan kondisi file sekarang di Go (tanpa dependency baru), menyajikannya sebagai unified diff dengan hunk di tab Review, dan mengisi ringkasan `+/-` di daftar file. Klik file di daftar membuka diff, dan ada tombol untuk membuka file itu di pane yang ada. File biner atau di atas 1 MB masuk daftar tanpa diff. Epic ini juga menyediakan komponen `DiffView` dengan slot aksi per hunk dan handler klik baris yang akan diisi tiket 03 (aksi) dan 04 (komentar).

## Hasil yang bisa dijalankan
Di atas tiket 01 (baseline, daftar file, tab Review, badge):
- Di tab Review, tiap baris file kini menampilkan ringkasan `+N -M` (angka nyata, bukan 0).
- Klik baris file membuka tampilan diff unified di tab yang sama: header hunk `@@ -a,b +c,d @@`, nomor baris lama dan baru, warna tambah/hapus/konteks, dan tombol kembali ke daftar.
- Dua edit berturut-turut pada file yang sama dalam satu sesi menghasilkan satu diff terhadap baseline pertama.
- File `added` ditampilkan sebagai diff dengan seluruh baris tambah; file `deleted` seluruh baris hapus.
- File biner atau di atas 1 MB: baris daftar menandainya ("biner" / "terlalu besar"), dan membukanya menampilkan pesan tanpa diff.
- Tombol "Buka file" pada baris daftar dan pada header diff membuka file di pane yang ada lewat `openFilePane`; bila pane file mendukung posisi baris, hunk pertama dipakai sebagai target (lihat catatan di fitur `review-diff-ui`).
- Bagian aksi per hunk tampil kosong (belum ada tombol Revert/Terima); itu milik tiket 03.
- `go build ./...` dan build frontend lulus; aplikasi jalan.

## Batasan
In scope:
- Algoritma diff baris (Myers) di Go, konteks 3 baris, pengelompokan hunk, id hunk stabil, hash isi saat ini.
- IPC `review.diff` dan pengisian `additions`/`deletions` (serta `binary`/`tooLarge`) pada hasil `review.files`.
- `DiffView.svelte` (render unified diff, slot aksi hunk, handler klik baris), `diff.ts` (tipe dan pemanggil IPC), perluasan kecil `ReviewTab.svelte`.
- Penanganan file biner dan di atas 1 MB (1.048.576 byte), file baru (baseline "tidak ada"), file terhapus.

Out of scope:
- Revert, Terima, deteksi `external`, `expectHash` dipakai (tiket 03). Epic ini hanya mengembalikan `currentHash`.
- Komentar dan "Kirim ke agent" (tiket 04). Navigasi keyboard `j/k/a/r` (tiket 03).
- Tampilan side-by-side, diff semantik, syntax highlighting, diff intra-baris.
- Mengubah format penyimpanan baseline, hook, `ClassifyEvent`, registry runs, atau tipe yang dimiliki tiket 01 (kecuali edit aditif yang disebut eksplisit di Wiring).
- Dependency baru, unit test.

## Kontrak
Dipakai dari tiket 01 (baca dulu `internal/review/store.go`, `capture.go`, `ipc.go`, `types.go`, `cmd/shikigamid/wire_review.go`, `frontend/src/modules/review/ReviewTab.svelte`, `store.ts`, `types.ts` dan ikuti nama aktualnya; bila berbeda dari uraian di bawah, ikuti kode dan catat di Wiring):
- Penyimpanan `DataDir/review/<sessionId>/baseline/<sha256(path)>` dan `index.json` (path, ada-tidaknya sebelum disentuh, sha256 baseline, waktu sentuh pertama, status ditinjau). Epic ini hanya membaca lewat API store tiket 01 (mis. ambil entri dan baca isi baseline); tidak menulis.
- Validasi path di dalam cwd sesi dengan aturan `internal/fsx` (dipakai ulang oleh tiket 01; pakai fungsi yang sama untuk membaca file sekarang).
- IPC `review.files {sessionId}` -> `{supported, files:[{path,status,additions,deletions,reviewed,external,binary,tooLarge}]}`; `additions`/`deletions` bernilai 0 sampai epic ini.
- Pendaftaran method IPC lewat mekanisme yang dipakai `ipc.go` tiket 01 (`RegisterMethod` atau setara) dan wiring di `cmd/shikigamid/wire_review.go`.
- Frontend: store daftar file per sesi aktif di `store.ts`, tipe di `types.ts`, `openFilePane({projectId, filePath, workspaceId?, title?})` dari `frontend/src/modules/shell/helpers.ts`, pemanggil IPC dari `lib/ipc.ts`.

Disediakan epic ini:
```go
// internal/review/diff.go
type DiffLine struct {
    Kind  string // "ctx" | "add" | "del"
    Text  string // tanpa newline akhir
    OldNo int    // 0 untuk baris add
    NewNo int    // 0 untuk baris del
}
type Hunk struct {
    ID                                   string // 12 hex pertama sha256(oldStart,oldLines,newStart,newLines + isi baris)
    OldStart, OldLines, NewStart, NewLines int
    Lines                                []DiffLine
}
type DiffResult struct {
    Hunks       []Hunk
    CurrentHash string // sha256 hex isi file sekarang; "" bila file tidak ada
    Binary      bool
    TooLarge    bool
    Additions   int
    Deletions   int
}
const MaxDiffBytes = 1 << 20
const ContextLines = 3
// ComputeDiff membandingkan dua isi (nil = tidak ada). Murni, tanpa IO.
func ComputeDiff(old, cur []byte) DiffResult
// DiffForFile membaca baseline (lewat store tiket 01) dan isi sekarang, lalu ComputeDiff.
func DiffForFile(st *Store, sessionID, path string) (DiffResult, error)
```
```
// internal/review/diff_ipc.go
IPC review.diff {sessionId, path}
  -> {hunks:[{id,oldStart,oldLines,newStart,newLines,lines:[{kind,text,oldNo,newNo}]}],
      currentHash, binary, tooLarge}
review.files (diperluas): additions/deletions/binary/tooLarge terisi dari ComputeDiff.
```
Catatan tipe JSON: nama field camelCase persis seperti di kontrak bersama (`oldStart`, `oldLines`, `newStart`, `newLines`, `currentHash`, `tooLarge`). `hunks` selalu array (bukan null) walau kosong.

```ts
// frontend/src/modules/review/diff.ts
export type DiffLineKind = 'ctx' | 'add' | 'del';
export interface DiffLine { kind: DiffLineKind; text: string; oldNo: number; newNo: number }
export interface Hunk { id: string; oldStart: number; oldLines: number; newStart: number; newLines: number; lines: DiffLine[] }
export interface DiffResult { hunks: Hunk[]; currentHash: string; binary: boolean; tooLarge: boolean }
export function fetchDiff(sessionId: string, path: string): Promise<DiffResult>
```
```svelte
<!-- DiffView.svelte -->
props: { diff: DiffResult; path: string; onLineClick?: (line: DiffLine, hunk: Hunk) => void }
slot "hunk-actions" (let:hunk) : dirender di header tiap hunk, kosong secara default
slot "footer" : dirender di bawah diff, kosong secara default
```
Slot dan `onLineClick` adalah titik ekstensi untuk tiket 03 dan 04. Jangan menghapus atau mengganti nama setelah dipublikasikan.

## Fitur

### review-diff-engine: Mesin diff dan IPC review.diff
Deskripsi:
- `diff.go`: implementasi Myers kecil pada slice baris (pemisah `\n`; normalisasi `\r\n` hanya untuk perbandingan bila perlu, tetapi `Text` menyimpan baris apa adanya tanpa `\n` akhir). Pilih Myers di Go, bukan `git diff --no-index`, agar tidak menyentuh allowlist `internal/gitdata` dan berjalan di folder non-git. Pakai varian dengan batas biaya (mis. batas D maksimum) dan fallback "semua baris lama dihapus, semua baris baru ditambah" bila terlampaui, agar tidak lambat pada file mendekati 1 MB.
- Pengelompokan hunk: 3 baris konteks di sekitar perubahan, hunk yang jaraknya <= 2*ContextLines digabung. `oldStart`/`newStart` berbasis 1; untuk sisi kosong (file baru atau terhapus) `oldStart`/`newStart` = 0 dan `oldLines`/`newLines` = 0 mengikuti konvensi unified diff.
- Baris tanpa newline di akhir file diperlakukan sebagai baris biasa (tanpa penanda khusus pada v1), konsisten antara `ComputeDiff` dan tiket 03 yang akan menerapkan balik hunk.
- Id hunk stabil: deterministik dari angka rentang dan isi baris, sehingga memanggil ulang `review.diff` pada isi yang sama menghasilkan id sama.
- Deteksi biner: ada byte NUL dalam 8 KB pertama salah satu sisi, atau pakai helper yang sudah ada di `internal/fsx/binary.go` (baca dan pakai ulang bila tersedia). `Binary=true` mengembalikan `Hunks` kosong. `TooLarge=true` bila salah satu sisi > `MaxDiffBytes`, hunk kosong, tanpa membaca isi penuh bila ukuran sudah diketahui dari `Stat`.
- `CurrentHash`: sha256 isi file sekarang (dihitung juga untuk file biner dan terlalu besar, dengan membaca streaming); file tidak ada -> string kosong.
- `DiffForFile`: baseline dibaca lewat store tiket 01; entri "tidak ada sebelumnya" -> `old=nil`; file sekarang tidak ada -> `cur=nil` (status deleted). Path divalidasi di dalam cwd sesi dengan aturan `internal/fsx`; path di luar cwd atau sesi/entri tidak dikenal mengembalikan error jelas.
- `diff_ipc.go`: daftarkan `review.diff` lewat mekanisme IPC tiket 01. Sediakan fungsi baru (mis. `FillDiffStats(st *Store, sessionID string, files []FileEntry)`) yang mengisi `additions`, `deletions`, `binary`, `tooLarge` pada entri daftar file, dan dipanggil dari handler `review.files` lewat titik ekstensi tiket 01 atau edit aditif minimal yang dicatat di Wiring. Jangan merombak `ipc.go`. Biaya: hitung per file saat `review.files`, pakai cache kecil di memori kunci `(sessionId,path,currentHash)` agar pemanggilan berulang murah; kesalahan baca satu file tidak menggagalkan seluruh daftar (file itu tetap muncul dengan 0/0).
- File yang kembali identik dengan baseline menghasilkan `Hunks` kosong dan 0/0; penghapusannya dari daftar adalah wewenang tiket 01/03, jangan diubah di sini.

Acceptance criteria:
- [ ] `ComputeDiff(nil, x)` menghasilkan satu hunk semua baris `add`; `ComputeDiff(x, nil)` semua `del`; `ComputeDiff(x, x)` tanpa hunk.
- [ ] Nomor `oldNo`/`newNo` benar: `ctx` punya keduanya, `add` hanya `newNo`, `del` hanya `oldNo`.
- [ ] Dua perubahan berjauhan menghasilkan dua hunk; dua perubahan berdekatan (jarak <= 6 baris) satu hunk; konteks maksimum 3 baris.
- [ ] Id hunk sama untuk input sama dan berbeda untuk isi berbeda.
- [ ] File dengan byte NUL -> `binary:true`, tanpa hunk; file > 1 MB -> `tooLarge:true`, tanpa hunk; keduanya tetap mengembalikan `currentHash`.
- [ ] File besar mendekati 1 MB dengan banyak perubahan selesai dalam waktu wajar (tidak menggantung); batas biaya Myers bekerja.
- [ ] `review.diff` untuk path di luar cwd ditolak dengan error; untuk sesi/path tak dikenal error jelas.
- [ ] `review.files` mengembalikan `additions`/`deletions` benar, `binary`/`tooLarge` terisi, dan tetap valid bila satu file gagal dibaca.
- [ ] Tanpa dependency baru di `go.mod`; berjalan di folder non-git.
- [ ] `go build ./...` lulus.

ownedPaths: `internal/review/diff.go`, `internal/review/diff_ipc.go`.
Paralel: bisa dikerjakan paralel dengan `review-diff-ui` bila kontrak di bagian Kontrak dipegang sebagai antarmuka; UI dapat memakai data tiruan sampai IPC siap. Integrasi akhir ada di Wiring.

### review-diff-ui: DiffView dan navigasi dari daftar
Deskripsi:
- `diff.ts`: tipe dan `fetchDiff(sessionId, path)` yang memanggil IPC `review.diff` lewat `lib/ipc.ts`. Tambah juga helper murni kecil yang dibutuhkan UI (mis. `hunkSummary(hunk)`), tanpa state global.
- `DiffView.svelte`: render unified diff dari prop `diff`. Per hunk: header `@@ -oldStart,oldLines +newStart,newLines @@` (tampilkan di sisi kanan header slot `hunk-actions`, kosong secara default), lalu baris dengan dua kolom nomor (lama, baru), penanda `+`/`-`/spasi, dan teks monospace (`white-space: pre`, scroll horizontal di dalam blok diff, bukan halaman). Warna baris add/del memakai token tema yang sudah ada di aplikasi (cek variabel CSS yang dipakai `FilesTab`/viewer; ikuti, jangan menambah palet baru), cocok di tema gelap dan terang. Klik nomor baris atau baris memanggil `onLineClick(line, hunk)` bila disediakan, kursor pointer hanya bila prop ada. Status khusus: `binary` -> pesan "File biner, tidak ada diff"; `tooLarge` -> "File terlalu besar (> 1 MB), diff tidak ditampilkan"; hunk kosong -> "Tidak ada perubahan terhadap baseline". Render hunk panjang secara efisien (jangan membuat ribuan komponen anak per baris; cukup `{#each}` pada baris datar).
- `ReviewTab.svelte` (milik tiket 01; perubahan di sini kecil dan eksplisit): tiap baris file menampilkan ringkasan `+additions -deletions` (hijau/merah) dan penanda "biner"/"terlalu besar" bila flag aktif; klik baris memilih file dan menampilkan `DiffView` menggantikan daftar (dengan header berisi tombol kembali, path, dan tombol "Buka file"); `fetchDiff` dipanggil saat file dipilih dan diulang ketika event `review.changed` untuk sesi aktif tiba atau daftar file berubah selagi diff terbuka. Tombol "Buka file" pada baris daftar juga ada (ikon kecil, `stopPropagation` agar tidak membuka diff) dan memanggil `openFilePane({projectId, filePath: <path absolut = cwd proyek + path relatif>, workspaceId})`. Ambil projectId dan cwd sesi aktif dari store/helper yang sudah dipakai `ReviewTab`/shell; ikuti cara `FilesTab` membentuk path absolut. Posisi baris: `openFilePane` saat ini tidak menerima nomor baris; bila pane file sudah punya mekanisme membuka pada baris tertentu (baca `frontend/src/modules/shell/helpers.ts` dan komponen viewer), pakai `newStart` hunk pertama dan dari header hunk (ikon "buka di baris" lewat handler yang tiket 03/04 tidak ganggu); bila tidak ada, buka file saja tanpa mengubah modul shell (di luar kepemilikan). Jangan memodifikasi file di luar ownedPaths.
- Pemuatan: tampilkan state loading singkat dan error inline dengan tombol coba lagi; jangan melempar. Race: abaikan respons `fetchDiff` basi bila pengguna sudah berpindah file atau sesi.
- Sesi berganti atau file hilang dari daftar saat diff terbuka: kembali ke daftar.
- Aksesibilitas: baris file dapat difokus (button/role), `aria-label` pada tombol buka file; teks UI dalam bahasa Indonesia.

Acceptance criteria:
- [ ] Daftar file menampilkan `+N -M` benar dan penanda biner/terlalu besar.
- [ ] Klik file menampilkan unified diff dengan header hunk, nomor baris lama/baru, dan warna add/del/ctx yang terbaca di tema gelap dan terang.
- [ ] File biner dan terlalu besar menampilkan pesan, bukan diff atau error.
- [ ] Tombol kembali mengembalikan ke daftar tanpa kehilangan filter/scroll yang sudah ada dari tiket 01.
- [ ] Tombol "Buka file" membuka file di pane yang ada lewat `openFilePane` (fokus pane lama bila sudah terbuka) dan tidak membuka diff.
- [ ] `DiffView` mengekspos slot `hunk-actions` dan `footer` (kosong secara default) serta prop `onLineClick`; tanpa keduanya tampilan tidak berubah.
- [ ] Diff terbuka diperbarui saat `review.changed` tiba; respons basi diabaikan.
- [ ] Tidak ada dependency npm baru; build frontend lulus.

ownedPaths: `frontend/src/modules/review/DiffView.svelte`, `frontend/src/modules/review/diff.ts`, `frontend/src/modules/review/ReviewTab.svelte` (hanya perubahan penyisipan DiffView, ringkasan, dan tombol buka file).
Paralel: dapat dikerjakan paralel dengan `review-diff-engine`; satu-satunya titik temu adalah bentuk JSON `review.diff` dan field tambahan `review.files` pada Kontrak. Edit pada `ReviewTab.svelte` dikerjakan pada penutup tiket (Wiring) bila agent berbeda mengerjakan dua fitur agar tidak bentrok.

## Path yang dimiliki
Hanya ini yang boleh disentuh:
- `internal/review/diff.go`
- `internal/review/diff_ipc.go`
- `frontend/src/modules/review/DiffView.svelte`
- `frontend/src/modules/review/diff.ts`
- `frontend/src/modules/review/ReviewTab.svelte` (milik tiket 01; hanya perubahan kecil yang disebut di fitur `review-diff-ui` dan Wiring)
- Edit aditif minimal yang dicatat eksplisit di Wiring, bila titik ekstensi tiket 01 tidak cukup: `internal/review/ipc.go` (satu titik panggil `FillDiffStats` atau variabel hook) dan `cmd/shikigamid/wire_review.go` (satu baris pendaftaran `review.diff`). Tidak boleh mengubah perilaku lain di file-file itu.

## Wiring
Langkah penutup tiket ini, sehingga hasilnya terlihat dan jalan tanpa bergantung pada tiket 03/04:
1. Backend: periksa apakah `ipc.go`/`wire_review.go` tiket 01 menyediakan titik ekstensi (pendaftaran method tambahan dan pengayaan hasil `review.files`). Bila ada, pakai dari `diff_ipc.go`. Bila tidak, lakukan hanya dua edit aditif: (a) di `ipc.go`, panggil `FillDiffStats(...)` pada hasil `review.files` sebelum dikembalikan; (b) di `wire_review.go`, daftarkan `review.diff` (mis. `RegisterDiffIPC(reg, store)`). Catat edit ini dalam pesan commit.
2. Frontend: di `ReviewTab.svelte` sisipkan ringkasan `+/-`, penanda biner/terlalu besar, tombol "Buka file", dan tampilan `DiffView` saat file dipilih (state `selectedPath`); `DiffView` dibiarkan tanpa isi slot `hunk-actions` dan `footer`.
3. Verifikasi manual: jalankan aplikasi, di sesi Claude minta agent mengedit file yang sama dua kali; di tab Review pastikan `+/-` terisi, klik file menampilkan satu diff terhadap baseline pertama; coba file baru (diff semua tambah), file biner (pesan), dan tombol "Buka file".
4. Tiket ini tidak bergantung pada tiket 03/04: tanpa mereka, tab Review tetap menampilkan daftar dan diff read-only.

## Guard
- `go build ./...` berhasil.
- Build frontend berhasil (jalankan perintah build di `frontend/`, mis. `npm run build`; cek `package.json` untuk skrip yang berlaku).
- Tanpa unit test.
