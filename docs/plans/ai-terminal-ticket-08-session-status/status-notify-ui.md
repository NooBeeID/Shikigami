# Plan: Notifikasi macOS dan lencana UI

## Metadata

- Tiket: `ai-terminal-ticket-08-session-status` (`docs/tickets/ai-terminal-ticket-08-session-status.md`)
- Fitur: `status-notify-ui`
- ownedPaths: `internal/status/notify.go`, `frontend/src/modules/status/`
- Depends on foundation: ya. Sisi Go bergantung pada `internal/status/types.go` dan stub `Engine` dari Foundation (`00-foundation.md`). Sisi frontend tidak bergantung pada Foundation, hanya pada kontrak event dan `status.list`.
- Rujukan desain bersama: `overview.md` bagian 2.3 (tipe), 2.4 (API), 2.5 (IPC), 2.8 (frontend), 2.9 (notifikasi), 6 (guard), 7 (aturan paralel).

## Tujuan

1. Mengirim notifikasi macOS (via `osascript`) hanya saat sesi bertransisi ke `waiting` atau `done` bukan karena exit, event-driven dan tanpa polling.
2. Menampilkan status sesi secara real-time di UI: lencana per sesi di sidebar dan ringkasan di statusbar, dari satu langganan `session.status` ditambah muat awal `status.list`.

## Path yang dimiliki

HANYA path ini yang boleh dibuat atau diubah:

- `internal/status/notify.go` (ganti stub Foundation)
- `frontend/src/modules/status/` (folder baru: `index.ts`, `store.ts`, `StatusBadge.svelte`, `SidebarBadges.svelte`, `StatusBar.svelte`)

Dilarang menyentuh: `internal/status/types.go`, `engine.go`, `idle.go`, `ipc.go`, `hooks.go`, `install.go`, `cmd/aitermd/wire_status.go`, `cmd/aitermctl/cmd_status.go`, `frontend/src/App.svelte`, `frontend/src/lib/`, `frontend/src/modules/shell/`, `frontend/src/modules/workspace/`, `go.mod`/`go.sum`, serta path terlarang lain di overview bagian 7. Tipe atau konstanta tambahan harus berupa identifier privat di file milik sendiri. Jangan menyambungkan `StartNotifier` ke wiring (sudah dipanggil `wire_status.go` milik fitur lain).

## Kontrak

### Dipakai (dari Foundation, signature persis; jangan diubah)

- `types.go`: `Status`, `StatusRunning/Waiting/Done/Idle`, `SourceHook/Output/Exit`, `StatusEvent{SessionID, Status, Prev, Source, Reason, At}`.
- `(*Engine).Subscribe(fn func(StatusEvent)) (cancel func())` dan `(*Engine).Lookup(sessionID string) (session.Session, bool)` (metadata sesi: `ProviderID`, `Cwd`, `ID`). Subscriber dipanggil di luar mutex engine, jadi boleh memanggil `eng.Lookup` di dalam callback.
- `provider.Get(id string) (provider.Provider, bool)` dan `Provider.Name() string` (paket `aiterm/internal/provider`, hanya dibaca).
- IPC dari frontend: `status.list` `{}` -> `[{sessionId, status, since}]` (array JSON langsung). Event `session.status` payload `StatusEvent` JSON (`sessionId, status, prev, source, reason?, at`).
- Frontend lib (hanya diimpor, tidak diedit): `registerPanel` dari `../../lib/registry` (panel dirender tanpa props: `<panel.component />`); `call`, `onEvent` dari `../../lib/ipc`; store `sessions` dan `providers` dari `../../lib/stores` (`Session{id, providerId, cwd, status:'running'|'exited', ...}`, `Provider{id, name}`).
- CSS variable shell yang ada: `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--ok`, `--danger`.

### Disediakan

- Go: `func StartNotifier(srv *ipc.Server, eng *Engine)` (signature kontrak overview 2.4; tanpa return).
- Frontend (`store.ts`): `statusStore: Readable<Record<string, SessionStatus>>`, `startStatusSync(): () => void`, `statusOf(id: string): Readable<Status>`; tipe `Status`, `SessionStatus`.
- Frontend: komponen `StatusBadge.svelte` (props `sessionId: string`, `compact?: boolean`) yang dapat diimpor shell kelak.

## Langkah

### A. Sisi Go: `internal/status/notify.go`

1. Hapus komentar `// STUB(foundation)` dan ganti isi file. Paket `status`. Impor stdlib (`os`, `os/exec`, `path/filepath`, `strings`, `sync`, `time`, `log`) ditambah `aiterm/internal/ipc` dan `aiterm/internal/provider`. Pakai hanya impor yang terpakai.
2. Konstanta privat: `notifyDebounce = 2 * time.Second`, `notifyMaxEntries = 256`.
3. `StartNotifier(srv *ipc.Server, eng *Engine)`:
   - Return langsung bila `eng == nil` atau `os.Getenv("AITERM_NOTIFY") == "0"` (dibaca sekali di sini). `srv` tidak dipakai (boleh `_ = srv`; catat di komentar bahwa parameter dipertahankan sesuai kontrak).
   - Buat `n := &notifier{eng: eng, last: map[string]time.Time{}}` dan panggil `eng.Subscribe(n.handle)`. `cancel` tidak dipakai (hidup selama daemon); catat di komentar.
4. `type notifier struct { eng *Engine; mu sync.Mutex; last map[string]time.Time }`.
5. `func (n *notifier) handle(ev StatusEvent)`:
   - Lewati bila `ev.Status` bukan `StatusWaiting`/`StatusDone`, atau `ev.Prev == ev.Status`, atau `ev.Source == SourceExit`.
   - Debounce: di bawah `n.mu`, bila `now - n.last[ev.SessionID] < notifyDebounce` -> return; selain itu set `n.last[id] = now`. Bila `len(n.last) > notifyMaxEntries`, buang entri yang lebih tua dari `notifyDebounce` (satu kali sapuan saat menyisipkan, bukan goroutine/ticker). Peta tidak dibersihkan lewat Forget (sengaja, dibatasi ukuran; beri komentar).
   - Pesan: `waiting` -> `"Menunggu input Anda"`, `done` -> `"Selesai"`.
   - Judul: `n.title(ev.SessionID)`.
   - Spawn `go send(title, msg)` (goroutine sekali pakai, bukan loop).
6. `func (n *notifier) title(id string) string`: `s, ok := n.eng.Lookup(id)`; bila ok, nama provider = `provider.Get(s.ProviderID)` -> `p.Name()`, fallback `s.ProviderID`; hasil `"<Nama> · <filepath.Base(s.Cwd)>"` (bagian cwd dihilangkan bila `Cwd` kosong). Bila tidak ok: `"Sesi " + id[:min(8,len(id))]`.
7. `func escapeAppleScript(s string) string`: ganti `\` menjadi `\\` DULU, lalu `"` menjadi `\"`. Ganti juga `\n`/`\r` dengan spasi agar literal AppleScript tidak putus.
8. `func send(title, msg string)`: bangun skrip `display notification "<esc(msg)>" with title "<esc(title)>"` lalu `exec.Command("osascript", "-e", script).Run()` (argumen terpisah, TANPA shell, tanpa `sh -c`). Error dicatat dengan `log.Printf` (Indonesia) dan tidak panic. Beri komentar: kriteria "jendela app fokus" diabaikan karena daemon tidak tahu fokus jendela (keputusan tiket); nonaktif via `AITERM_NOTIFY=0`; proses hanya di-spawn saat ada event notifikasi. Bila `runtime.GOOS != "darwin"`, `send` return diam-diam (impor `runtime`).
9. Jangan memakai `time.Ticker`, `time.Tick`, atau loop tidur-bangun.

### B. Sisi frontend: `frontend/src/modules/status/`

Sebelum menulis, baca ulang `frontend/src/lib/registry.ts`, `frontend/src/lib/ipc.ts`, `frontend/src/modules/shell/*.svelte`, `frontend/src/modules/workspace/SessionList.svelte` untuk memastikan kontrak masih sama (panel dirender tanpa props). Semua file memakai Svelte 5 (`$props`, `$derived`, `$state` atau sintaks store `$store`, selaras gaya modul workspace) dan TypeScript. Komentar Indonesia.

10. **`store.ts`**
    - `export type Status = 'running' | 'waiting' | 'done' | 'idle'`; `export interface SessionStatus { status: Status; since: number }` (nama lokal modul; tidak bentrok karena tidak diekspor ulang ke `lib`).
    - `const store = writable<Record<string, SessionStatus>>({})`; `export const statusStore: Readable<...> = { subscribe: store.subscribe }`.
    - `export function statusOf(id: string): Readable<Status>` = `derived(store, m => m[id]?.status ?? 'idle')`.
    - `startStatusSync(): () => void`: idempoten dengan penghitung referensi/modul-level `unsub`. Panggilan pertama: (a) `unsub = onEvent('session.status', handler)` DAHULU (agar tak ada event hilang), lalu (b) `call<Array<{sessionId, status, since}>>('status.list')` dan gabungkan ke store tanpa menimpa entri yang sudah diperbarui event (entri dari event dengan `at` lebih baru dipertahankan; simpan `since` event = `ev.at`). Error `status.list` ditelan (log `console.warn`), tidak melempar. Panggilan berikutnya mengembalikan fungsi pembersih yang sama. Fungsi pembersih memanggil `unsub()` dan mereset flag.
    - `handler(topic, payload)`: validasi payload bentuk `StatusEvent`; peta `status`: bila `payload.source === 'exit'` simpan sebagai `done`; set `store.update(m => ({...m, [sessionId]: {status, since: payload.at}}))`.
    - Karena engine membuang state saat exit dan shell menghapus sesi sendiri, tambahkan juga pembersihan entri: di `SidebarBadges`/`StatusBar` hanya sesi yang ada di store `sessions` (lib) yang dihitung/ditampilkan, jadi entri yatim tidak tampil (tidak perlu timer).
    - Dilarang `setInterval`/`setTimeout`/polling.
11. **`StatusBadge.svelte`**: props `sessionId: string`, `compact?: boolean`. Ambil `status` dari `statusOf(sessionId)`. Render `<span class="badge" role="status" aria-label={label} title={label}>` berisi `<span class="dot {status}">` dan, bila bukan `compact`, teks label. Label Indonesia: running "Berjalan", waiting "Menunggu", done "Selesai", idle "Diam". Warna: `running` biru (`var(--accent, #4f8cff)`), `waiting` kuning (`#e5b13a`), `done` hijau (`var(--ok, #3fb950)`), `idle` abu (`var(--fg-muted, #8b949e)`). Animasi denyut `running` murni `@keyframes` CSS (opacity/scale), dengan `@media (prefers-reduced-motion: reduce) { animation: none }`. Tanpa timer JS.
12. **`SidebarBadges.svelte`**: panel tanpa props. Registry tidak menyediakan slot lencana di baris sesi milik shell dan shell tidak boleh diedit, maka (catat pilihan ini di komentar) panel ini menampilkan daftar ringkas sesi + lencana sendiri. Sumber daftar: store `sessions` dan `providers` dari `../../lib/stores` (hanya baca), tampilkan sesi `status === 'running'` (belum exit). Tiap baris: `<StatusBadge sessionId={s.id} />`, nama provider (`$providers.find(...)?.name ?? s.providerId`), dan basename `s.cwd` (potong dengan ellipsis). Judul bagian "Status sesi". Bila tidak ada sesi aktif: render kosong (tidak ada teks panel) atau satu baris redup "Tidak ada sesi aktif". Gaya memakai `--fg`, `--fg-muted`, `--border`, `--bg-hover` dengan fallback.
13. **`StatusBar.svelte`**: panel tanpa props. Hitung dari `$statusStore` terbatas pada ID di `$sessions` yang `status === 'running'` (sesi tak ada di store dianggap `idle`). Teks ringkas, mis. `"2 berjalan, 1 menunggu"`; sertakan `selesai` dan `diam` hanya bila > 0 (urutan: berjalan, menunggu, selesai, diam). Bila tidak ada sesi aktif, tidak merender apa pun (elemen kosong/`{#if}`). Beri `aria-live="polite"`. Warna `--fg-muted`.
14. **`index.ts`**:
    ```ts
    import { registerPanel } from '../../lib/registry';
    import SidebarBadges from './SidebarBadges.svelte';
    import StatusBar from './StatusBar.svelte';
    import { startStatusSync } from './store';

    registerPanel({ id: 'status-badges', area: 'sidebar', order: 90, component: SidebarBadges });
    registerPanel({ id: 'status-summary', area: 'statusbar', order: 50, component: StatusBar });
    startStatusSync();
    export { default as StatusBadge } from './StatusBadge.svelte';
    ```
    (Ekspor `StatusBadge` agar shell dapat mengimpornya kelak; catat di komentar.) `startStatusSync()` aman saat di luar Wails: `onEvent` mengembalikan no-op dan `call` menolak, yang sudah ditangani di `store.ts`.
15. Modul dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts')` di `main.ts`; jangan mengedit `main.ts`.

### C. Verifikasi (guard)

16. `gofmt -l internal/status/notify.go` kosong; `go build ./...` hijau; opsional `go vet ./internal/status/...`.
17. `cd frontend && npm run build` hijau (opsional `npm run check` untuk `svelte-check` pada file modul ini).
18. `grep -nE "time\.(NewTicker|Tick)\(|setInterval|setTimeout" internal/status/notify.go frontend/src/modules/status` kosong. `grep -rn "STUB(foundation)" internal/status/notify.go` kosong. Pastikan hanya path milik fitur ini yang berubah.

## Perubahan

- Diubah: `internal/status/notify.go` (stub diganti implementasi `StartNotifier`, `notifier`, `escapeAppleScript`, `send`).
- Dibuat: `frontend/src/modules/status/index.ts`, `store.ts`, `StatusBadge.svelte`, `SidebarBadges.svelte`, `StatusBar.svelte`.

## Wiring

Fitur ini hanya mengekspos; penyambungan dilakukan langkah wiring (overview bagian 5), bukan di sini:

- Go: `status.StartNotifier(srv, eng)` dipanggil dari `cmd/aitermd/wire_status.go` (milik `status-engine`, sudah memanggilnya sejak Foundation) setelah `NewEngine`/`RegisterIPC`/`InstallHooks`.
- Frontend: `index.ts` dimuat otomatis oleh glob `main.ts`; panel `status-badges` (sidebar, order 90) dan `status-summary` (statusbar, order 50) dirender shell lewat registry. Tidak ada edit shell/`App.svelte`/`lib`.
- Opsional kelak (di luar tiket): shell mengimpor `StatusBadge` dari `modules/status` untuk lencana di baris sesinya sendiri.
- Ketergantungan runtime: `session.status` dan `status.list` disediakan `status-engine`; frontend memperlakukan kegagalan `status.list` sebagai daftar kosong.

## Selesai bila

- `go build ./...` sukses (guard utama path Go); `cd frontend && npm run build` sukses (guard path frontend). Tanpa unit test.
- `StartNotifier` hanya memicu `osascript` untuk transisi ke `waiting`/`done` dengan `Prev != Status` dan `Source != exit`; debounce 2 detik per sesi; `AITERM_NOTIFY=0` menonaktifkan; argumen `osascript` terpisah tanpa shell dan teks di-escape (`\` lalu `"`).
- Tidak ada ticker/polling di `notify.go` maupun `frontend/src/modules/status/`; animasi lencana hanya CSS.
- `frontend/src/modules/status/` memuat kelima file, dua panel terdaftar (`status-badges` sidebar, `status-summary` statusbar), satu langganan `session.status` + muat awal `status.list`.
- Tidak ada stub Foundation tersisa di `notify.go` dan tidak ada file di luar ownedPaths yang berubah.
