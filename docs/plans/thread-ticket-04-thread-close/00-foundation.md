# Foundation: thread-ticket-04-thread-close

Rujukan: `docs/plans/thread-ticket-04-thread-close/overview.md` (bagian 2 = kontrak dan keputusan). Foundation dikerjakan SENDIRIAN sebelum fitur paralel. Tanpa unit test; guard hanya build.

Tujuan: setelah selesai, field `closeSummary` dan konstanta method close ada di tipe (Go dan TS), serta stub `close.go` dan `close/index.ts` berkompilasi sehingga kedua fitur paralel dan Wiring punya titik awal. Aplikasi berperilaku sama seperti sebelumnya.

Berkas yang boleh disentuh: `internal/thread/types.go` (hanya menambah), `internal/thread/close.go` (stub baru), `frontend/src/modules/thread/types.ts` (hanya menambah), `frontend/src/modules/thread/close/index.ts` (stub baru). Tidak ada yang lain.

## Langkah 0: verifikasi prasyarat

1. Pastikan ada `internal/thread/{types,manager,ipc,mcp_tools,worktree,worktree_records,agents_install,prompt}.go` dan `frontend/src/modules/thread/{types.ts,slots.ts,ThreadTab.svelte,thread-store.ts,thread-client.ts}`; `go build ./...` lulus.
2. Pastikan simbol ini ada: `Manager.UpdateThread`, `Manager.PostSystem`, `Manager.FireClose`, `Manager.RegisterMethod`, `RegisterTool`, `ToolDef`, `Caller`, `InspectWorktree`, `RemoveWorktree`, `LoadWorktreeRecords`, `DeleteWorktreeRecord`, `Store.ThreadDir`, `Store.MasterDir`, `ToolsSection`, `ErrClosed`, `ErrNotFound`, `ErrInvalid`, `registerThreadSlot` (TS). Bila nama berbeda, ikuti kode dan catat di bagian "Catatan" bawah; jangan mengubah berkas tiket lain.
3. Pastikan `internal/thread/close.go` dan `frontend/src/modules/thread/close/` belum ada; bila sudah ada, jangan menimpa, catat di "Catatan".

## Langkah 1: `types.go`

1. Pada struct `Thread`, tambahkan setelah `ReadyToClose`:
   ```go
   CloseSummary string `json:"closeSummary,omitempty"` // ringkasan usul tutup dari master (tiket thread-close)
   ```
2. Pada blok konstanta method (ganti komentar "Ditambah tiket thread-close" dengan konstanta nyata):
   ```go
   MethodClosePreview = "thread.closePreview"
   MethodClose        = "thread.close"
   ```
   Tidak mengubah field/tag/konstanta lain.

## Langkah 2: stub `internal/thread/close.go`

```go
package thread

// Stub Foundation thread-ticket-04. Diganti penuh oleh thread-close-daemon.
// Pemanggilan dari wire_thread.go: thread.RegisterClose(m), setelah InstallAgents.

// RegisterClose memasang usul tutup, preview, dan Close. Stub: belum melakukan apa pun.
func RegisterClose(m *Manager) {}
```

Komentar dalam bahasa Indonesia; paket `thread`.

## Langkah 3: `types.ts`

1. Pada interface `Thread` tambahkan field opsional: `closeSummary?: string;`.
2. Pada `THREAD_METHODS` tambahkan `closePreview: 'thread.closePreview', close: 'thread.close',` (pertahankan `as const`). Tidak mengubah yang lain.

## Langkah 4: stub `frontend/src/modules/thread/close/index.ts`

```ts
// Stub Foundation thread-ticket-04. Diganti penuh oleh thread-close-ui.
export function installClose(): () => void {
  return () => {};
}
```

## Langkah 5: guard

1. `go build ./...` lulus dan `gofmt -l internal/thread` kosong.
2. `cd frontend && npm run check && npm run build` lulus.
3. `git status` hanya menunjukkan empat berkas di atas; tidak ada perubahan `go.mod`/`package.json`.

## Catatan

(Isi bila nama simbol di kode berbeda dari yang diasumsikan.)
