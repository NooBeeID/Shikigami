# Plan: Sidebar kiri Agent

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-04-shell-sidebars.md` (fitur `sidebar-left-agent`)
- Fitur: `sidebar-left-agent` (Sidebar kiri Agent)
- ownedPaths: `frontend/src/modules/sidebar-left/`
- Depends on foundation: ya (`shell-frame`: `modules/shell/helpers.ts`, `modules/shell/index.ts`, `lib/stores/shell.ts` harus sudah ada; lihat `00-foundation.md`)
- Kontrak bersama: `overview.md` (bagian 2.1, 2.5, 2.7, 2.9, 7). Plan ini tidak mengulangnya.

## Tujuan

Membangun isi slot `left` shell v2: komponen `AgentSidebar` berisi header workspace (nama, "N project · M sesi", dropdown ganti workspace), daftar sesi workspace aktif dikelompokkan per project (status, penanda "di layout", menu Akhiri sesi / Buka di layout, sesi `exited` redup), dan tombol "New Session" (popover project + provider, atau ajakan Open Folder bila tanpa project). Semua perilaku layout/sesi memakai helper `modules/shell`; tidak ada pemanggilan IPC baru dan tidak ada penyusunan argumen provider di frontend.

## Path yang dimiliki

HANYA `frontend/src/modules/sidebar-left/`. Dilarang menyentuh: `App.svelte`, `modules/shell/` (termasuk `register.ts`; foundation dan wiring), `main.ts`, `lib/stores/*`, modul `workspace`/`status`/`files`/`openfolder`/`appmenu`/`zoom`/`layout`/`tabs`, dan path fitur lain (`topbar`, `sidebar-right`, `palette`, `notifications`). Tidak boleh mengimpor fitur paralel lain. Bila butuh perubahan di luar path ini, laporkan sebagai blocker, jangan dikerjakan.

## Kontrak

Disediakan:
- `modules/sidebar-left/AgentSidebar.svelte`: default export, tanpa props wajib (dipakai wiring di slot `left`, overview 2.7).
- `modules/sidebar-left/index.ts`: re-export `AgentSidebar` saja, tanpa efek samping (tidak mendaftar apa pun ke registri).

Dipakai (nama persis, semua sudah ada; baca berkasnya sebelum menulis kode):
- Dari `../../lib/stores` (`index.ts`): `workspaces`, `activeWorkspaceId`, `activeWorkspace`, `activeProjectId`, `selectWorkspace(id): Promise<void>`, `sessions`, `killSession(id)`, `providers` (`Provider {id,name,available?}`); tipe `Session`, `Project`, `Workspace`.
- Dari `../../lib/stores/layout`: `layout` (store; dibaca lewat `$layout` atau `layout.leaves`) hanya untuk mengetahui sesi mana yang sudah jadi pane. Dari `../layout/model`: tipe `Leaf`. Impor langsung, bukan lewat `lib/stores/index.ts` (overview 2.9).
- Dari `../shell` (helper foundation, overview 2.5): `focusOrOpenSession(sessionId): boolean`, `findSessionPane(sessionId): Leaf | null`, `sessionTitle(sessionId): string`, `createAndOpenSession(providerId, projectId?): Promise<Session>`, `openFolder(): void`, `basename(path)`.
- Dari `../status/store`: `statusStore` (opsional, hanya bila perlu); dari `../status/StatusBadge.svelte`: `StatusBadge` (props `{sessionId, compact?}`). Jangan memanggil `startStatusSync()` dan jangan membuat pendengar `session.status`.
- Dari `../workspace/errors`: `errMessage(e)` untuk pesan galat. `WorkspacePicker` TIDAK dipakai ulang (UI-nya select + tombol Baru/Ganti nama/Hapus, tidak cocok sebagai dropdown header); header memakai komponen sendiri yang memanggil `selectWorkspace` yang sama, tanpa menyalin logika IPC.
- Teks UI bahasa Indonesia kecuali label baku: `AGENT`, `New Session`, `Open Folder…`.

## Langkah

Baca dulu: `lib/stores/index.ts`, `lib/stores/layout.ts`, `modules/layout/model.ts`, `modules/shell/helpers.ts` + `index.ts` (hasil foundation), `modules/status/StatusBadge.svelte`, `modules/workspace/SessionList.svelte` dan `NewSession.svelte` (referensi gaya dan penanganan galat). Bila `modules/shell/helpers.ts` belum ada, hentikan dan laporkan blocker. Svelte 5 runes; tanpa dependency baru; tanpa `console.log`; tanpa polling; warna hanya lewat variabel tema (`--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok`).

1. **`modules/sidebar-left/groups.ts`** (util murni, tanpa IPC): 
   ```ts
   import type { Session, Project } from '../../lib/stores';
   export interface SessionGroup { projectId: string; name: string; sessions: Session[] }
   export function groupSessionsByProject(sessions: Session[], projects: Project[], workspaceId: string | null): SessionGroup[];
   ```
   Filter `s.workspaceId === workspaceId`; satu grup per project di urutan `projects` (grup tanpa sesi tetap dikembalikan agar header project muncul); sesi yang `projectId`-nya tidak ada di `projects` masuk grup tambahan `{projectId, name: 'Project lain', ...}` di akhir. Dalam grup urut `createdAt` naik.

2. **`WorkspaceHeader.svelte`** (props `{ onerror: (msg: string) => void }`). Tampilkan nama `$activeWorkspace?.name ?? 'Tanpa workspace'` dan baris kecil `"{N} project · {M} sesi"` dengan N = `$activeWorkspace?.projects.length ?? 0`, M = jumlah sesi di `$sessions` dengan `workspaceId === $activeWorkspaceId`. Seluruh header berupa tombol; klik membuka dropdown (`$state open`) berisi `$workspaces` (item aktif diberi tanda dan gaya aksen). Klik item memanggil `selectWorkspace(id)` dibungkus try/catch -> `onerror(errMessage(e))`, lalu menutup dropdown. Tutup dropdown dengan Esc, klik di luar (listener `pointerdown` di `window` dipasang di `$effect` hanya saat `open`, dilepas di cleanup), dan setelah memilih. Dropdown diposisikan `position:absolute` di bawah header, `z-index` di atas daftar, `max-height` dengan scroll. Aksesibilitas: `aria-haspopup="listbox"`, `aria-expanded`. Tidak ada buat/ganti nama/hapus workspace di sini (itu milik topbar).

3. **`SessionRow.svelte`** (props `{ session: Session; inLayout: boolean; onerror: (msg: string) => void }`). Satu baris sesi:
   - `StatusBadge` (`sessionId={session.id}`, `compact`), judul `sessionTitle(session.id)`, lencana provider (teks singkat: `claude`->CL, `codex`->CX, `shell`->SH, lainnya 2 huruf pertama kapital), penanda "di layout" (titik/ikon kecil dengan `title="Tampil di layout"`) bila `inLayout`.
   - Klik baris (tombol utama): `focusOrOpenSession(session.id)`; bila mengembalikan `false`, `onerror('Sesi tidak ditemukan')`. Berlaku juga untuk sesi `exited` (tetap membuka pane).
   - Sesi `exited`: kelas `exited` (`opacity: 0.5`), tetap bisa diklik; `StatusBadge` tetap dirender apa adanya.
   - Menu `…` per baris (tombol terpisah, `stopPropagation` agar tidak memicu klik baris; popover kecil, tutup dengan Esc/klik luar seperti langkah 2): item "Buka di layout" (memanggil `focusOrOpenSession`, sama dengan klik) dan "Akhiri sesi". "Akhiri sesi" memakai konfirmasi inline: klik pertama mengganti isi menu menjadi "Akhiri sesi ini?" dengan tombol "Akhiri" (danger) dan "Batal"; "Akhiri" memanggil `killSession(session.id)` (try/catch -> `onerror`) lalu menutup menu. "Akhiri sesi" dinonaktifkan bila `session.status === 'exited'`. Menutup pane tidak pernah memanggil `killSession`.

4. **`SessionGroups.svelte`** (props `{ onerror }`). Turunkan `groups = $derived(groupSessionsByProject($sessions, $activeWorkspace?.projects ?? [], $activeWorkspaceId))`. Himpunan sesi yang tampil di layout: `$derived` dari `$layout` dengan `findSessionPane`-setara tanpa memicu efek samping, mis. `const shown = $derived(new Set($leaves.filter(l => l.kind==='terminal' && l.sessionId).map(l => l.sessionId!)))` memakai `layout.leaves` (jangan memanggil `findSessionPane` per baris bila reaktivitasnya tidak terjamin; `findSessionPane` boleh dipakai di handler klik). Tiap grup: header project (nama + jumlah sesi, tombol lipat/buka; state lipat `$state<Record<string, boolean>>` di komponen, tidak dipersist), lalu `SessionRow` per sesi (`{#each ... (s.id)}`). Grup kosong menampilkan teks redup "Belum ada sesi". Bila workspace tanpa project dan tanpa sesi, komponen menampilkan teks "Belum ada project" (ajakan membuka folder ada di `NewSessionMenu`). Area ini `overflow:auto; min-height:0` agar daftar bisa di-scroll dan tombol New Session tetap di bawah.

5. **`NewSessionMenu.svelte`** (props `{ onerror }`). Tombol "New Session" lebar penuh di bawah sidebar; klik membuka popover ke atas (`bottom: 100%`). Isi:
   - Bila `($activeWorkspace?.projects.length ?? 0) === 0`: teks "Belum ada project" dan tombol "Open Folder…" yang memanggil `openFolder()` lalu menutup popover (tidak ada pemilihan provider).
   - Selain itu: select/daftar project (nilai awal = `$activeProjectId` bila ada di project workspace, selain itu project pertama; `$state selectedProjectId` disinkronkan lewat `$effect` saat project aktif berubah dan popover dibuka) dan daftar provider dari `$providers` (tombol per provider: `name`; `available === false` dinonaktifkan dengan `title="Tidak tersedia"`). Klik provider memanggil `createAndOpenSession(provider.id, selectedProjectId)` (helper itu sendiri memanggil `selectProject` lalu `createSession` lalu membuka pane; frontend tidak menyusun argumen provider atau flag izin). Selama berjalan `busy=true` (nonaktifkan tombol, cegah klik ganda); galat -> `onerror(errMessage(e))`; sukses -> tutup popover.
   - Esc/klik di luar menutup popover (pola sama dengan langkah 2).

6. **`AgentSidebar.svelte`** (tanpa props wajib). Kolom flex penuh tinggi (`height:100%; min-height:0; display:flex; flex-direction:column; background: var(--bg-elev); border-right: 1px solid var(--border)`). Dari atas: label kecil `AGENT` (huruf kapital, `--fg-muted`), `WorkspaceHeader`, `SessionGroups` (`flex:1; overflow:auto`), baris galat singkat (`$state err`; tampil di atas tombol bila ada, hilang sendiri saat aksi berikutnya berhasil atau dengan tombol tutup kecil; tanpa timer berulang), `NewSessionMenu`. Satu callback `onerror = (m) => err = m` diteruskan ke anak. Komponen harus aman bila kolom disembunyikan lewat lebar 0 (`min-width:0; overflow:hidden`), karena foundation membiarkannya tetap ter-mount.

7. **`index.ts`**: `export { default as AgentSidebar } from './AgentSidebar.svelte';` saja. Tidak ada `registerPanel`/registri apa pun (modul dimuat otomatis oleh glob `main.ts`, jadi harus bebas efek samping).

8. **Guard.** `cd frontend && npm install` (bila `node_modules` belum ada, mis. di worktree), lalu `npm run build`; usahakan `make build` dari root repo bila sempat. Perbaiki semua galat tipe/kompilasi di berkas milik fitur ini. Tampilan aplikasi belum berubah sampai wiring; itu wajar. Grep akhir: tidak ada impor dari `../topbar`, `../sidebar-right`, `../palette`, `../notifications`, dan tidak ada `console.log`, `setInterval`, atau pemanggilan `window.go`/`lib/ipc` di `modules/sidebar-left/`.

## Perubahan

Dibuat (semuanya di `frontend/src/modules/sidebar-left/`):
- `groups.ts`
- `WorkspaceHeader.svelte`
- `SessionRow.svelte`
- `SessionGroups.svelte`
- `NewSessionMenu.svelte`
- `AgentSidebar.svelte`
- `index.ts`

Tidak ada file yang diubah di luar folder itu.

## Wiring

Yang diekspos untuk langkah `shell-wiring` (JANGAN disambungkan di plan ini):
- `frontend/src/modules/sidebar-left/AgentSidebar.svelte` (default export, tanpa props wajib) untuk snippet `left` pada `<ShellFrame>` di `App.svelte` (`{#snippet left()}<AgentSidebar />{/snippet}`; import `./modules/sidebar-left/AgentSidebar.svelte`).
- `modules/sidebar-left/index.ts` mengekspor `AgentSidebar` dan tidak punya efek samping, sehingga tidak perlu masuk `modules/shell/register.ts`.

## Selesai bila

- Semua berkas di bagian Perubahan ada; hanya `frontend/src/modules/sidebar-left/` yang berubah.
- Header menampilkan nama workspace, "N project · M sesi" yang benar, dan dropdown mengganti workspace lewat `selectWorkspace`.
- Sesi terkelompok per project dengan `StatusBadge` yang memperbarui real-time (dari `statusStore` lewat `StatusBadge`, tanpa polling); sesi yang tampil sebagai pane diberi penanda "di layout".
- Klik sesi memanggil `focusOrOpenSession` (memfokuskan pane yang ada atau menambahkan ke layout); sesi `exited` tampil redup dan tetap bisa dibuka.
- Menu `…` menyediakan "Buka di layout" dan "Akhiri sesi" dengan konfirmasi inline yang memanggil `killSession`; menutup pane tidak membunuh sesi.
- New Session membuat sesi lewat `createAndOpenSession(providerId, projectId)` dan membuka pane-nya; tanpa project tampil ajakan "Open Folder…" yang memanggil `openFolder()`.
- Tidak ada duplikasi IPC, tidak ada penyusunan argumen/flag provider, tidak ada impor antar fitur paralel, tidak ada dependency npm baru.
- Guard: `cd frontend && npm run build` sukses (dan `make build` bila dijalankan); tanpa unit test.
