# Plan: Adapter connector lark

## Metadata
- Tiket: `lark-connector-ticket-01-lark-connector` (`docs/tickets/lark-connector-ticket-01-lark-connector.md`)
- Fitur: `lark-connector-adapter`
- ownedPaths: `internal/connector/lark.go`
- Depends on foundation: tidak (epic ini tidak punya foundation; fitur ini hanya memakai `RunCLI` yang sudah ada)

## Tujuan
Mendaftarkan connector `lark` (ID `lark`, Name `Lark`) di kerangka Connector yang sudah ada. `Status()` membaca `lark-cli auth status` dan memetakan hasilnya ke `cli_missing`, `not_logged_in`, atau `connected` (dengan nama user). `ConnectGuide()` memberi langkah Indonesia dan perintah `lark-cli auth login`. Tanpa token, tanpa output mentah di status, tanpa cache, tanpa unit test.

## Path yang dimiliki
HANYA file ini yang boleh dibuat/disentuh:
- `internal/connector/lark.go` (baru)

Jangan menyentuh: `internal/connector/cli.go` (milik fitur `larkdata-runner`; pakai `RunCLI` apa adanya, jangan menunggu `RunCLITimeout`), `types.go`, `registry.go`, `fake.go`, `ipc.go`, `internal/larkdata/*`, `cmd/*`, `frontend/*`, `docs/lark/*`, file wiring apa pun, dan perubahan tak terkait di working tree (`app/*`, `frontend/*`, `internal/provider/*`). Jangan stage/commit file lain.

## Kontrak
Rujukan: overview bagian 2.1, 2.2, 2.4.

Disediakan (package `connector`):
```go
type larkConnector struct{}
func init() { Register(larkConnector{}) }
func (larkConnector) ID() string   // "lark"
func (larkConnector) Name() string // "Lark"
func (larkConnector) Status(ctx context.Context) (Status, error)
func (larkConnector) ConnectGuide() Guide
```

Dipakai (sudah ada, jangan diubah): `Status{State, User, Message}`, `StateCLIMissing`, `StateNotLoggedIn`, `StateConnected`, `Guide{Steps, Commands}`, `Connector`, `Register`, `RunCLI(ctx, name, args...) (string, error)`, `IsCLIMissing(err)`.

Aturan `Status()`:
- `RunCLI(ctx, "lark-cli", "auth", "status")` (tanpa flag format; tambahkan hanya bila `docs/lark/cli-findings.md` sudah ada saat Anda bekerja dan menyatakan perlu).
- Galat dari `RunCLI`: `IsCLIMissing(err)` -> `Status{State: StateCLIMissing, Message: "lark-cli belum terpasang"}, nil`. Galat lain -> `Status{}, err` (dikembalikan sebagai error; bungkus dengan `fmt.Errorf("lark-cli auth status: %w", err)` boleh). Pengecualian: bila `cli-findings.md` ada dan menyatakan exit non-nol berarti belum login, petakan ke `StateNotLoggedIn`.
- Decode stdout dengan struct privat (tag json `identities`, `user`, `status`, `available`, `userName`; `User` bertipe pointer agar field hilang terdeteksi). Tidak boleh ada field token.
- JSON tak terparse, `identities.user` nil, atau `available == false` -> `Status{State: StateNotLoggedIn, Message: "Belum login ke Lark. Ikuti langkah di dialog Connect."}, nil`.
- Selain itu -> `Status{State: StateConnected, User: userName}, nil`. `status == "needs_refresh"` tetap connected (tidak ada percabangan khusus).
- Jangan menaruh output mentah CLI atau pesan stderr ke `Status.Message`.

## Langkah
1. Baca `internal/connector/fake.go` dan `types.go` untuk pola (sudah diringkas di atas). Periksa apakah `docs/lark/cli-findings.md` sudah ada; bila ada, baca bagian skema `auth status` dan pola galat, dan ikuti temuan bila membantah skema awal. Bila belum ada, lanjut dengan skema awal; jangan menunggu.
2. Buat `internal/connector/lark.go`, `package connector`, impor `context`, `encoding/json`, `fmt`.
3. Tulis struct privat:
   ```go
   type larkAuthStatus struct {
       Identities struct {
           User *struct {
               Status    string `json:"status"`
               Available bool   `json:"available"`
               UserName  string `json:"userName"`
           } `json:"user"`
       } `json:"identities"`
   }
   ```
   `json.Unmarshal` mengabaikan field tak dikenal dan menerima field hilang, jadi tahan terhadap tambahan/kekurangan field tanpa panic.
4. Tulis `type larkConnector struct{}`, `func init() { Register(larkConnector{}) }`, `ID()` mengembalikan `"lark"`, `Name()` mengembalikan `"Lark"`.
5. Implementasi `Status(ctx)` sesuai aturan di Kontrak. Urutan: jalankan `RunCLI`; tangani galat (cli missing / lainnya); `json.Unmarshal([]byte(out), &st)`; bila err atau `st.Identities.User == nil` atau `!st.Identities.User.Available` -> not_logged_in; selain itu connected dengan `User: st.Identities.User.UserName`. Tanpa cache dan tanpa variabel global berstatus.
6. Implementasi `ConnectGuide() Guide`:
   - `Steps` (Indonesia), mis.:
     1. "Pasang lark-cli bila belum terpasang di mesin ini."
     2. "Salin perintah di bawah, lalu jalankan di terminal untuk login ke Lark."
     3. "Kembali ke sini dan tekan Cek ulang."
   - `Commands: []string{"lark-cli auth login"}`.
7. Beri komentar dokumentasi singkat (bahasa Indonesia) pada tipe dan fungsi, gaya `fake.go`.
8. Jalankan `gofmt -l internal/connector/lark.go` (harus kosong) dan `go build ./...` dari root repo. Bila build gagal karena file milik fitur lain yang belum selesai (mis. `internal/larkdata`), pastikan minimal `go build ./internal/connector/` dan `go vet ./internal/connector/` lulus, dan catat kondisinya; jangan memperbaiki file fitur lain.

## Perubahan
- Dibuat: `internal/connector/lark.go`
- Diubah: tidak ada.

## Wiring
Fitur ini mengekspos connector `lark` lewat `Register` di `init()`, sehingga IPC `connector.list/status/guide` dan Pengaturan > Connector memuatnya otomatis. JANGAN mengubah file wiring: paket `internal/connector` sudah diimpor daemon (`cmd/shikigamid/wire_connector.go`), tidak ada perubahan yang diperlukan. Langkah wiring nanti (overview bagian 5) hanya memverifikasi hal itu dan menyelaraskan skema/aturan exit di `lark.go` dengan `docs/lark/cli-findings.md` bila berbeda dari nilai awal.

## Selesai bila
- `internal/connector/lark.go` ada, terdaftar via `init()`; `Get("lark")` mengembalikan connector dengan `Name() == "Lark"`.
- Perilaku: CLI tak ada -> `cli_missing`; JSON rusak/identitas `user` hilang/`available false` -> `not_logged_in`; tersedia -> `connected` dengan `User = userName`; `needs_refresh` -> `connected`; galat lain -> `error`.
- `ConnectGuide()` memuat langkah Indonesia dan `lark-cli auth login`.
- Status tidak memuat token atau output mentah CLI; tidak ada cache.
- `gofmt` bersih dan build path fitur ini sukses (`go build ./...`, atau minimal `go build ./internal/connector/`). Tanpa unit test.
- Tidak ada file lain yang diubah (`git status` hanya menambah `internal/connector/lark.go` dari fitur ini).
