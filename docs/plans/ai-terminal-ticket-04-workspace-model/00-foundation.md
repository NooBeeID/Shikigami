# Foundation: Epic 04 Workspace Model

Dikerjakan SATU agent sebelum fitur paralel. Baca `overview.md` (bagian 2 dan 4) dan tiket dahulu. Tujuan: `go build ./...` hijau dengan semua simbol kontrak ada (Store berupa stub), sehingga `workspace-store` dan `workspace-ipc-ctl` bisa dikerjakan bersamaan.

Batasan: hanya buat `internal/workspace/types.go`, `internal/workspace/store.go`, `internal/workspace/config.go`. Jangan ubah `go.mod`/`go.sum` atau file lain. Hanya stdlib.

## Langkah

1. **`types.go` (final, bukan stub).** `package workspace`. Tulis tipe `Project`, `WorkspaceConfig`, `Workspace`, `GlobalConfig` persis seperti tiket (tag json sama). Tambahkan sentinel `ErrNotFound` ("not found"), `ErrInvalid` ("invalid argument"), `ErrConflict` ("conflict") dengan `errors.New`. Implementasikan `newID() string`: 6 byte timestamp ms (big-endian, 48-bit) + 10 byte `crypto/rand` = 128 bit, di-encode Crockford base32 (alfabet `0123456789ABCDEFGHJKMNPQRSTVWXYZ`) menjadi 26 karakter (karakter pertama hanya 0-7). `crypto/rand.Read` praktis tidak pernah gagal; bila gagal, `panic` dengan pesan jelas (satu-satunya panic yang diizinkan, di dalam `newID`).

2. **`store.go` (stub).** Deklarasikan `type Store struct{ mu sync.RWMutex }` dan semua method pada overview bagian 2.3 dengan signature persis. Stub: `NewStore` mengembalikan `&Store{}, nil`; `List` mengembalikan `[]Workspace{}`; method lain mengembalikan nilai nol dan `fmt.Errorf("%w: belum diimplementasi", ErrInvalid)`. Tandai tiap method dengan komentar `// STUB(foundation): diganti oleh workspace-store`. Semua method memakai receiver pointer `*Store`.

3. **`config.go` (stub).** `package workspace` dengan komentar `// STUB(foundation): diganti oleh workspace-store` dan `func validateURL(u string) error { return nil }` (agar file tidak kosong dan helper ada; `workspace-store` menggantinya dengan versi final). Boleh tidak dipakai (fungsi tak terpakai valid di Go).

4. **Verifikasi.** `go build ./...` dan `gofmt -l internal/workspace` (kosong). Pastikan `go.mod` tidak berubah.

## Keluaran yang diharapkan
- `internal/workspace/` berisi 3 file; `types.go` final, `store.go` dan `config.go` stub bertanda STUB.
- Build hijau.
