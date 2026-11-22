# Plan: Field activeRail di UIState (daemon)

## Metadata

- Tiket: `docs/tickets/main-rail-ticket-01-main-rail.md` (`main-rail-ticket-01-main-rail`)
- Fitur: `rail-state-persist-go`
- ownedPaths: `internal/workspace/types.go`, `internal/workspace/store.go` (store.go hanya bila terbukti menjatuhkan field)
- Depends on foundation: tidak (tahap A, paralel dengan `rail-registry-store`)
- Kontrak lengkap: lihat `docs/plans/main-rail-ticket-01-main-rail/overview.md` bagian 2.3 dan 6.

## Tujuan

Daemon Go dapat menyimpan dan membaca kembali item rail aktif untuk sidebar kiri lewat field `ActiveRail` pada `SidebarState`, dengan default `"agents"`. File state lama tanpa `activeRail` tetap dimuat tanpa error.

## Path yang dimiliki

HANYA boleh disentuh:
- `internal/workspace/types.go`
- `internal/workspace/store.go` (hanya bila merge terbukti menjatuhkan field; hampir pasti tidak perlu)

Jangan sentuh `internal/workspace/layout.go`, `internal/workspace/config.go`, file wiring, file frontend, maupun path fitur lain. Jangan sentuh perubahan uncommitted di `app/*`, `internal/provider/*`.

## Kontrak

Disediakan (sesuai overview 2.3):

```go
type SidebarState struct {
    Visible    bool   `json:"visible"`
    Width      int    `json:"width"`
    ActiveTab  string `json:"activeTab,omitempty"`  // hanya sidebar kanan
    ActiveRail string `json:"activeRail,omitempty"` // hanya sidebar kiri
}
func defaultLeftSidebar() SidebarState { return SidebarState{Visible: true, Width: 260, ActiveRail: "agents"} }
```

Dipakai (sudah ada, tidak diubah): `UIPatch.LeftSidebar *SidebarState`, `Store.SetUI`, `Store.GetUI`. Tidak ada pesan IPC baru; patch UI yang ada membawa `SidebarState` utuh, kini dengan `activeRail`.

Hasil pemeriksaan yang sudah diverifikasi saat plan ditulis:
- `layout.go` `SetUI` (baris ~108-110): `nu.LeftSidebar = *p.LeftSidebar` lalu hanya `ActiveTab = ""`. `ActiveRail` ikut tersimpan. Tidak perlu edit.
- `config.go` (baris ~118-127): sidebar kiri diganti default hanya bila `Width == 0`; selain itu hanya `ActiveTab` dikosongkan. `ActiveRail` tidak dijatuhkan. Tidak perlu edit.
- `store.go` `persistLocked` menulis `UIState` utuh lewat `writeJSONAtomic`; tidak ada penyaringan field.

## Langkah

1. Di `internal/workspace/types.go`, pada `type SidebarState struct`, tambahkan field setelah `ActiveTab`:
   ```go
   ActiveRail string `json:"activeRail,omitempty"` // hanya sidebar kiri
   ```
   Rapikan alignment field (gofmt).
2. Di file yang sama ubah `defaultLeftSidebar()` menjadi:
   ```go
   func defaultLeftSidebar() SidebarState {
       return SidebarState{Visible: true, Width: 260, ActiveRail: "agents"}
   }
   ```
   `defaultRightSidebar()` tidak diubah.
3. Verifikasi cepat (baca saja) bahwa `SetUI`, `config.go` dan `persistLocked` di `store.go` tidak menjatuhkan `ActiveRail` (grep `ActiveRail`/`LeftSidebar` di `internal/workspace`). Bila ternyata ada kasus penjatuhan di `store.go`, perbaiki minimal di sana. Bila penjatuhan ada di `layout.go`/`config.go`, JANGAN edit; catat di laporan.
4. Jalankan `gofmt -l internal/workspace` (harus kosong untuk dua berkas tadi) dan `go build ./...` dari root repo.

Catatan perilaku (tidak perlu kode): JSON lama tanpa `activeRail` terbaca sebagai string kosong; frontend memetakannya ke `'agents'`. Untuk state versi >= 2 dengan `Width != 0`, `ActiveRail` kosong dibiarkan kosong (sengaja, frontend yang menjadi fallback).

## Perubahan

- Diubah: `internal/workspace/types.go` (field `ActiveRail`, default `defaultLeftSidebar()`).
- Diubah hanya bila terbukti perlu: `internal/workspace/store.go` (diperkirakan tidak diubah).
- Tidak ada berkas baru, tidak ada unit test, tidak ada dependensi baru.

## Wiring

Tidak ada yang perlu disambungkan di sisi Go. Fitur ini hanya mengekspos field JSON `leftSidebar.activeRail` pada state UI dan patch UI (`UIPatch.leftSidebar`) yang nanti dipakai `rail-state-persist-ui` (restore/autosave frontend). Jangan menyentuh binding Wails atau `app/*`.

## Selesai bila

- `SidebarState` punya field `ActiveRail` dengan tag `json:"activeRail,omitempty"`.
- `defaultLeftSidebar()` mengembalikan `ActiveRail: "agents"`.
- Patch UI yang membawa `activeRail` tidak dijatuhkan oleh `SetUI`/`config.go`/`store.go` (terverifikasi lewat pembacaan kode); state lama tanpa field tetap dimuat tanpa error.
- `gofmt` bersih untuk berkas yang diubah.
- Guard: `go build ./...` dari root repo sukses. Tanpa unit test.
- Hanya `types.go` (dan `store.go` bila terbukti perlu) yang berubah di diff fitur ini.
