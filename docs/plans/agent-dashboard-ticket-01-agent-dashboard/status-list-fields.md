# Plan: status.list menyertakan reason dan providerId

## Metadata

- Tiket: `docs/tickets/agent-dashboard-ticket-01-agent-dashboard.md` (agent-dashboard-ticket-01-agent-dashboard)
- Fitur: `status-list-fields`
- Tahap: A (paralel dengan `dashboard-foundation`; tidak ada ketergantungan kode)
- ownedPaths: `internal/status/types.go`, `internal/status/engine.go`, `internal/status/ipc.go`
- Depends on foundation: tidak
- Referensi kontrak: `docs/plans/agent-dashboard-ticket-01-agent-dashboard/overview.md` bagian 2.1 dan bagian 6 (jangan diubah)

## Tujuan

Backend Go: `status.list` dan `status.get` mengembalikan `reason` terakhir dari transisi status dan `providerId` sesi, agar dashboard dapat menampilkan label "menunggu: <reason>" tanpa pencarian tambahan. Ini hanya penambahan field opsional (`omitempty`); bentuk lain dan event `session.status` tidak berubah, sehingga klien lama tetap jalan.

## Path yang dimiliki

HANYA tiga file ini yang boleh disentuh:

- `internal/status/types.go`
- `internal/status/engine.go`
- `internal/status/ipc.go` (hanya bila benar-benar perlu; kemungkinan besar tidak diubah)

Jangan menyentuh file frontend, file wiring, path fitur lain, maupun perubahan belum di-commit di `frontend/src/modules/sidebar-right/FilesTab.svelte` dan dokumen `docs/`.

## Kontrak

Disediakan (lihat overview 2.1):

```go
// internal/status/types.go
type SessionState struct {
    SessionID  string `json:"sessionId"`
    Status     Status `json:"status"`
    Since      int64  `json:"since"` // unix milidetik
    Reason     string `json:"reason,omitempty"`
    ProviderID string `json:"providerId,omitempty"`
}
```

- `status.get` dan `status.list` mengembalikan `SessionState` di atas (JSON camelCase).
- `StatusEvent` (event `session.status`) TIDAK berubah: `{sessionId, status, prev, source, reason, at}`.
- Tidak ada IPC method baru. Signature `Engine.Set(sessionID string, st Status, source, reason string) bool`, `Engine.Get`, `Engine.List` tidak berubah.

Dipakai: struct internal `sessionState` yang sudah menyimpan `providerID`.

## Langkah

1. `internal/status/types.go`: ubah `SessionState` menjadi persis seperti pada Kontrak. Tambahkan dua field di bawah `Since`, tag `json:"reason,omitempty"` dan `json:"providerId,omitempty"`. Jangan mengubah `StatusEvent` atau tipe lain.

2. `internal/status/engine.go`, struct `sessionState`: tambahkan field `reason string` (komentar singkat: alasan transisi terakhir; "" bila belum pernah bertransisi). Letakkan setelah `since`.

3. `internal/status/engine.go`, `bindLocked`: tidak perlu mengubah apa pun. Literal `&sessionState{id: ..., providerID: ..., status: StatusIdle, since: now}` sudah membuat `reason` bernilai `""` (zero value). Biarkan tanpa field `reason` eksplisit.

4. `internal/status/engine.go`, `Engine.Set`: di dalam blok setelah guard `state.status == st` (tempat `state.status = st` dan `state.since = now`), tambahkan `state.reason = reason`. Penugasan hanya terjadi pada transisi nyata (guard sudah mengembalikan `false` bila status sama, jadi reason tidak tertimpa oleh panggilan tanpa transisi). Jangan mengubah urutan lock/unlock, publish, atau pemanggilan subscriber.

5. `internal/status/engine.go`, `Engine.Get`: ubah literal return menjadi
   `SessionState{SessionID: st.id, Status: st.status, Since: st.since.UnixMilli(), Reason: st.reason, ProviderID: st.providerID}`.

6. `internal/status/engine.go`, `Engine.List`: pada pembuatan `row`, ubah `SessionState{...}` menjadi memuat juga `Reason: st.reason, ProviderID: st.providerID`. Pengurutan (since menaik, lalu SessionID) tidak berubah.

7. Jalur exit: verifikasi saja. `handleExit` sudah memanggil `e.Set(id, StatusDone, SourceExit, "exit")`, jadi event `session.status` dan `state.reason` memakai `"exit"`. Ubah hanya bila argumen reason ternyata kosong (saat ini tidak). Catatan perilaku yang diterima apa adanya: jika sesi sudah `done` (mis. lewat hook `Stop`) saat exit, `Set` mengembalikan `false` (tanpa transisi) sehingga reason tetap reason sebelumnya; dan `handleExit` memanggil `Forget`, sehingga sesi exit biasanya hilang dari `List`. Konsumen frontend memetakan exit lewat event `session.status` (source `exit`). Jangan menambah perilaku baru untuk kasus ini.

8. `internal/status/ipc.go`: periksa bahwa `status.get` mengembalikan `st` (bertipe `SessionState`) dan `status.list` mengembalikan `eng.List()`; keduanya diserialisasi otomatis oleh encoding JSON sehingga field baru ikut. TIDAK ada perubahan yang diperlukan; biarkan file tidak tersentuh.

9. Jalankan `gofmt -l internal/status` (harus kosong) dan `go build ./...` dari root repo. Perbaiki bila gagal, hanya di path yang dimiliki. Cari juga apakah ada tempat lain yang membuat literal `SessionState{...}` secara posisional (`grep -rn "SessionState{" --include=*.go .`); bila ada di luar ownedPaths dan menjadi error kompilasi, laporkan alih-alih mengedit (literal bernama tetap kompatibel).

## Perubahan

Diubah:
- `internal/status/types.go` (field `Reason`, `ProviderID` pada `SessionState`)
- `internal/status/engine.go` (field `reason` pada `sessionState`; `Set` mengisi; `Get` dan `List` mengisi `Reason`/`ProviderID`)

Tidak diubah (kecuali terpaksa): `internal/status/ipc.go`.

Tidak ada berkas baru.

## Wiring

Tidak ada yang disambungkan oleh fitur ini. Yang diekspos untuk konsumen nanti (`dashboard-store` di tahap B, lewat `modules/status/store.ts`): payload `status.list`/`status.get` yang kini dapat memuat `reason?: string` dan `providerId?: string`. Jangan menyentuh frontend; tidak ada langkah wiring yang perlu dilakukan di sini.

## Selesai bila

- [ ] `SessionState` punya `Reason` dan `ProviderID` dengan tag `json:"reason,omitempty"` dan `json:"providerId,omitempty"`.
- [ ] `sessionState` punya `reason`; `Set` mengisinya hanya pada transisi nyata.
- [ ] `Engine.Get` dan `Engine.List` mengisi `Reason` dan `ProviderID`.
- [ ] Jalur exit memberi reason `"exit"` (terverifikasi di `handleExit`, tidak perlu diubah).
- [ ] Sesi yang belum pernah bertransisi tidak menyertakan `reason` di JSON (omitempty); field lain tidak berubah.
- [ ] `StatusEvent` dan event `session.status` tidak berubah bentuk.
- [ ] Hanya file di ownedPaths yang berubah (`git status` memperlihatkan paling banyak `types.go`, `engine.go`, `ipc.go` dari fitur ini).
- [ ] Guard: `go build ./...` lulus (daemon dan app); tanpa unit test.
