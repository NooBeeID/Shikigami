# Plan: Palet perintah Cmd+K

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-04-shell-sidebars.md` (fitur `command-palette`)
- Fitur: `command-palette`
- ownedPaths: `frontend/src/modules/palette/`
- Depends on foundation: ya (`shell-frame` sudah tergabung: `modules/shell/helpers.ts`, `lib/stores/shell.ts`, `modules/shell/index.ts`). Prasyarat luar: tiket 02 dan 03 (ada `modules/appmenu`, `lib/stores/layout.ts`). Bila `modules/shell/helpers.ts` atau `lib/stores/shell.ts` tidak ada, hentikan dan laporkan blocker; jangan menulis stub di luar path sendiri.
- Kontrak lengkap: `overview.md` bagian 2 (terutama 2.4, 2.5, 2.9, bagian 3 catatan `command-palette`, bagian 7). Tidak diulang di sini.

## Tujuan

Overlay palet perintah yang dibuka Cmd+K dari mana pun (termasuk saat fokus di terminal atau input), untuk lompat ke project/sesi/file dan menjalankan aksi. Modul lain dapat menambah aksi/sumber lewat `registerPaletteAction`/`registerPaletteSource` tanpa mengubah kode palet. Tanpa dependency npm baru, tanpa polling, tanpa `console.log`.

## Path yang dimiliki

HANYA `frontend/src/modules/palette/`. Jangan sentuh `modules/shell/` (termasuk `register.ts`), `App.svelte`, `main.ts`, `lib/stores/*`, modul `files`/`workspace`/`layout`/dll, maupun path fitur lain. Butuh perubahan di luar path -> laporkan sebagai blocker. Jangan impor dari `sidebar-left`, `sidebar-right`, `topbar`, `notifications`.

## Kontrak

Disediakan (persis overview 2.4), `modules/palette/registry.ts`:

```ts
export interface PaletteAction { id: string; title: string; hint?: string; keywords?: string[]; run: () => void | Promise<void> }
export type PaletteSourceKind = 'project' | 'session' | 'file' | 'action';
export interface PaletteItem { id: string; kind: PaletteSourceKind; title: string; subtitle?: string; run: () => void | Promise<void> }
export interface PaletteSource { id: string; kind: PaletteSourceKind; list: (query: string) => PaletteItem[] | Promise<PaletteItem[]> }
export function registerPaletteAction(a: PaletteAction): void;   // idempoten per id (timpa)
export function registerPaletteSource(s: PaletteSource): void;   // idempoten per id (timpa)
```

Tambahan internal registry (dipakai `Palette`/`sources`, juga diekspor dari `registry.ts`, bukan dari `index.ts`): `paletteActions: Readable<PaletteAction[]>` dan `paletteSources: Readable<PaletteSource[]>` (derived terurut `id`).

`modules/palette/index.ts` mengekspor: `Palette` (komponen default dari `Palette.svelte`), `paletteOpen: Readable<boolean>`, `openPalette()`, `closePalette()`, `registerPaletteAction`, `registerPaletteSource`, tipe `PaletteAction`, `PaletteItem`, `PaletteSource`, `PaletteSourceKind`. `index.ts` juga mendaftarkan aksi/sumber bawaan saat dimuat (idempoten; lihat langkah 8).

Dipakai (sudah ada, jangan diubah):
- Dari `../shell` (barrel foundation): `focusOrOpenSession(sessionId): boolean`, `createAndOpenSession(providerId, projectId?): Promise<Session>`, `openFolder()`, `zoom('in'|'out'|'reset')`, `tidyGrid()`, `openFilePane({projectId, filePath, workspaceId?, title?})`, `basename(path)`, `sessionTitle(sessionId)`. Jangan menulis ulang logika itu.
- `../../lib/stores/shell`: `toggleLeft()`, `toggleRight()`.
- `../../lib/stores`: `sessions`, `activeWorkspace`, `activeWorkspaceId`, `activeProject`, `activeProjectId`, `selectProject(id)`, `providers`. Tipe `Session`, `Project`.
- `../files/tree/treeModel.svelte`: `TreeModel` (`setProjects`, `rows`, `toggle`, tipe `TreeRow`). Hanya baca antarmukanya; `modules/files` tidak diubah.
- `../status/StatusBadge.svelte` boleh dipakai di baris sesi (props `{sessionId, compact?}`); opsional.

## Langkah

1. **`registry.ts`.** Salin kontrak di atas. Dua `writable<Record<string,...>>` (actions, sources); `registerPaletteAction(a)`/`registerPaletteSource(s)` menimpa per `id` (`update(m => ({...m, [a.id]: a}))`). Ekspor `paletteActions` dan `paletteSources` sebagai `derived` yang mengurutkan `id.localeCompare`. Tanpa efek samping lain.

2. **`fuzzy.ts`.** Tanpa dependency.
   ```ts
   export interface FuzzyResult { score: number; indices: number[] }
   export function fuzzyMatch(query: string, text: string): FuzzyResult | null;
   export function fuzzyFilter<T>(items: T[], query: string, text: (t: T) => string, limit?: number): T[];
   ```
   `fuzzyMatch`: normalisasi lowercase + trim; query kosong -> `{score:0, indices:[]}`. Jika `text` mengandung query sebagai substring: skor tinggi (`1000 - indexAwal`, bonus +200 bila di awal kata/awal string, +100 bila prefix), `indices` = rentang substring. Selain itu cocokkan urutan karakter (subsequence, greedy kiri-ke-kanan); gagal -> `null`; skor = `500 - (rentang total) - (posisi cocok pertama)` ditambah bonus +10 per karakter yang jatuh di batas kata (setelah spasi, `/`, `-`, `_`, `.`). Query multi-kata (dipisah spasi): semua kata harus cocok (AND), skor dijumlah. `fuzzyFilter`: hitung skor, buang `null`, urut skor turun (stabil, urutan asli sebagai tie-break), potong ke `limit` bila diberikan; query kosong mengembalikan `items` asli terpotong `limit`. Fungsi murni, tanpa state.

3. **`actions.ts`.** `export function registerBuiltinActions(): void` memanggil `registerPaletteAction` untuk (id stabil, judul Indonesia kecuali label baku):
   - `session.new.claude`, `session.new.codex`, `session.new.shell`: judul "Sesi baru: claude" dst., `keywords:['new','session','sesi','agent']`, `run`: bila `get(activeProjectId)` kosong -> `openFolder()` lalu return (tidak ada project target); selain itu `await createAndOpenSession(providerId, get(activeProjectId))`. Sertakan hanya provider yang ada: daftar tetap `['claude','codex','shell']`; jika `get(providers)` terisi dan provider tidak ada di dalamnya atau `available === false`, `run` tetap tidak melempar (tangkap error `createAndOpenSession` dengan try/catch dan abaikan; jangan `console.log`; pesan galat tampil lewat jalur toast milik modul lain bila ada, jika tidak diam). `hint` = `'⌘K'` tidak dipakai; hint opsional teks pendek seperti nama provider.
   - `folder.open`: "Open Folder…", `run: openFolder`.
   - `zoom.in`/`zoom.out`/`zoom.reset`: "Zoom in", "Zoom out", "Zoom reset", `run: () => zoom('in'|'out'|'reset')`, `hint` `⌘+`/`⌘−`/`⌘0`.
   - `sidebar.left.toggle`: "Toggle sidebar kiri" (`hint:'⌘B'`), `run: toggleLeft`; `sidebar.right.toggle`: "Toggle sidebar kanan" (`hint:'⌘⌥B'`), `run: toggleRight`.
   - `layout.tidy`: "Susun grid", `run: tidyGrid`.
   Fungsi idempoten (aman dipanggil dua kali).

4. **`sources.ts`.** `export function registerBuiltinSources(): void` mendaftarkan tiga `PaletteSource`. Setiap `list(query)` mengembalikan item yang SUDAH disaring/diurutkan dengan `fuzzyFilter` dan dipotong `GROUP_LIMIT = 8` (konstanta diekspor dari `fuzzy.ts` atau `sources.ts`, dipakai juga `Palette`):
   - `project` (`id:'builtin.projects'`, `kind:'project'`): dari `get(activeWorkspace)?.projects`; item `{id:'project:'+p.id, kind:'project', title:p.name, subtitle:p.path, run: () => selectProject(p.id)}`; cocokkan terhadap `name + ' ' + path`.
   - `session` (`id:'builtin.sessions'`, `kind:'session'`): dari `get(sessions)` yang `workspaceId === get(activeWorkspaceId)`; judul `sessionTitle(s.id)`; `subtitle` = `${s.providerId} · ${nama project dari workspace aktif atau cwd basename} · ${s.status}`; `run: () => { focusOrOpenSession(s.id); }`. Sesi `exited` tetap muncul (klik tetap membuka pane).
   - `file` (`id:'builtin.files'`, `kind:'file'`): pencarian nama file di project AKTIF. Batasan (wajib dicatat di komentar kode): tidak ada metode cari/daftar rekursif di `lib/ipc`/`fsClient`, jadi hanya file yang sudah dimuat `TreeModel` yang dicari. Implementasi: satu `TreeModel` tingkat modul (`const model = new TreeModel()`) yang `setProjects([project aktif])` hanya bila signature `id:path` project aktif berubah (simpan signature terakhir; tanpa project aktif -> `setProjects([])`, hasil kosong). Agar ada isi, `list` memastikan akar project terbuka (`if (!rootRow.expanded) model.toggle(rootRow)`) dan, sekali per project, membuka direktori tingkat pertama (maks 40 direktori; lewati nama `node_modules`, `.git`, `dist`, `build`, `vendor`) dengan `model.toggle`. Isi `rows` memuat asinkron sehingga hasil muncul bertahap; `Palette` mengulang `list` saat query berubah dan sekali lagi setelah timer sekali jalan (langkah 7). Baris yang dicari: `rows.filter(r => !r.placeholder && (r.kind==='file' || r.kind==='symlink'))`; teks pencocokan = path relatif ke `r.root` (hapus prefiks `root + '/'`). Query kosong -> kembalikan array kosong (kelompok File hanya muncul saat mengetik). Item: `{id:'file:'+r.path, kind:'file', title:r.name, subtitle:relatif path, run: () => openFilePane({projectId: project.id, filePath: r.path, workspaceId: get(activeWorkspaceId) ?? undefined, title: r.name})}`. `filePath` yang dikirim ke `openFilePane` adalah path absolut yang sama dengan yang dipakai `FilesTab`/`FilesPanel` (`row.path`) agar `findFilePane` mengenali pane yang sama.
   Impor `TreeModel` dari `../files/tree/treeModel.svelte` (ekstensi `.svelte` pada nama berkas sesuai pola `FilesPanel`). Jangan mengimpor `FilesPanel` atau `shellBridge` untuk menembak `openFileTab`.

5. **`index.ts` (state palet).** Store modul: `const open = writable(false)`; `export const paletteOpen: Readable<boolean> = { subscribe: open.subscribe }`; `openPalette()` = `open.set(true)`; `closePalette()` = `open.set(false)`; `togglePalette()` internal (diekspor dari berkas state, bukan wajib dari `index.ts`). Agar `Palette.svelte` dan `index.ts` tidak saling impor melingkar, letakkan store ini di berkas `state.ts` (`paletteOpen`, `openPalette`, `closePalette`, `togglePalette`) dan re-export dari `index.ts`. `index.ts` juga: `import './...'` tidak diperlukan; panggil `registerBuiltinActions(); registerBuiltinSources();` pada tingkat modul (idempoten; kini aman karena registri menimpa per id), `export { default as Palette } from './Palette.svelte'`, re-export registry API dan tipe. Tanpa efek samping lain (tidak ada listener global di `index.ts`; listener ada di komponen).

6. **`Palette.svelte` bagian logika.** Svelte 5 runes, tanpa props wajib.
   - State: `query = $state('')`, `selected = $state(0)`, `groups = $state<{kind, label, items: PaletteItem[]}[]>([])`, `inputEl`, `previousFocus: HTMLElement | null`.
   - Handler Cmd+K: `$effect` (atau `onMount`) memasang `window.addEventListener('keydown', onGlobalKey, true)` (fase capture), dilepas pada cleanup. `onGlobalKey`: bila `e.metaKey && !e.altKey && !e.shiftKey && !e.ctrlKey && e.code === 'KeyK'` -> `e.preventDefault(); e.stopPropagation(); togglePalette()`. Tetap aktif saat fokus di input/terminal (tanpa pengecualian target). Tanpa dedupe tambahan (tidak ada event menu untuk Cmd+K).
   - Buka: berlangganan `paletteOpen` lewat `$paletteOpen` dalam `$effect`. Saat berubah `false -> true`: simpan `previousFocus = document.activeElement as HTMLElement | null` (sebelum input diberi fokus), reset `query=''`, `selected=0`, jalankan pencarian awal, `await tick()` lalu `inputEl.focus()`, dan pasang timer sekali jalan 250 ms yang menjalankan ulang pencarian (agar file yang baru dimuat `TreeModel` muncul; bersihkan timer bila palet ditutup/di-destroy). Saat `true -> false`: kembalikan fokus: `previousFocus?.isConnected ? previousFocus.focus() : null`; (untuk xterm, elemen aktif sebelumnya adalah `textarea.xterm-helper-textarea`, jadi fokus kembali ke pane terminal terakhir). Kosongkan `previousFocus`.
   - Pencarian `async function search()`: token naik (`const my = ++token`); baca `$paletteActions` dan `$paletteSources`. Kelompok Aksi: gabungkan `PaletteAction` -> `PaletteItem` (`id:'action:'+a.id, kind:'action', title:a.title, subtitle:a.hint, run:a.run`) dengan `fuzzyFilter(items, query, it => it.title + ' ' + (keywords.join(' ')))`, dibatasi `GROUP_LIMIT`. Kelompok lain: panggil `source.list(query)` tiap sumber (`Promise.allSettled`; sumber yang melempar diabaikan), kelompokkan per `kind` (sumber kustom dengan `kind:'action'` bergabung ke Aksi), potong `GROUP_LIMIT` per kelompok. Abaikan hasil bila `my !== token`. Urutan kelompok tetap: Aksi, Sesi, Project, File; kelompok kosong dibuang. Label: "Aksi", "Sesi", "Project", "File". Query kosong: Aksi (semua sampai limit), Sesi, Project tampil; File tidak (sumber file mengembalikan kosong).
   - `flat = $derived(groups.flatMap(g => g.items))`; `selected` di-clamp ke `[0, flat.length-1]` setelah tiap pencarian dan direset ke 0 saat `query` berubah (`oninput`).
   - Keyboard pada input (`onkeydown` lokal): `ArrowDown`/`ArrowUp` (siklik, `preventDefault`) mengubah `selected` dan `scrollIntoView({block:'nearest'})` pada baris terpilih; `Enter` -> `choose(flat[selected])`; `Escape` -> `closePalette()`; `Tab` ditahan di dalam overlay (`preventDefault`). Semua `stopPropagation` agar tidak sampai ke terminal. `choose(item)`: `closePalette()` DULU (agar fokus kembali, lalu aksi dapat memindahkan fokus ke pane baru), kemudian `await Promise.resolve(item.run())` dalam try/catch (galat ditelan; tanpa `console.log`). Catatan urutan: karena `run` sering memfokuskan pane/terminal baru, kembalikan-fokus dijalankan sinkron saat tutup lalu `run` dipanggil di `queueMicrotask`/setelah `tick()` sehingga fokus yang ditetapkan `run` yang menang.
   - Klik baris: `onclick` -> `choose(item)`; `onmousemove` pada baris mengatur `selected` ke indeks baris itu; klik backdrop (`onpointerdown` pada backdrop saja, `e.target === e.currentTarget`) -> `closePalette()`.

7. **`Palette.svelte` bagian markup dan gaya.** Hanya dirender bila `$paletteOpen` (`{#if}`); tidak ada cost saat tertutup selain listener.
   - Struktur: `<div class="backdrop" role="presentation">` -> `<div class="panel" role="dialog" aria-modal="true" aria-label="Palet perintah">` -> `<input class="input" placeholder="Ketik perintah, sesi, project, atau file…" role="combobox" aria-expanded="true" aria-controls="palette-list" aria-activedescendant=...>` -> `<div id="palette-list" role="listbox">` berisi per kelompok `<div class="group-label">` dan baris `<div role="option" id="palette-opt-{i}" aria-selected=...>` dengan judul, `subtitle` redup (ellipsis), lencana kecil `hint` di kanan; baris sesi boleh menampilkan `StatusBadge compact` bila `kind==='session'` dan id sesi dapat diturunkan (opsional; lewati bila mempersulit, tidak wajib). Keadaan kosong: "Tidak ada hasil" (teks redup).
   - Gaya (tanpa warna hardcode selain fallback, hanya variabel tema): backdrop `position:fixed; inset:0; z-index:1000; background: rgba(0,0,0,.35); display:flex; justify-content:center; align-items:flex-start; padding-top:12vh`; panel `width:min(560px, calc(100vw - 32px)); max-height:60vh; background:var(--bg-elev); color:var(--fg); border:1px solid var(--border); border-radius:10px; box-shadow:0 12px 40px rgba(0,0,0,.45); display:flex; flex-direction:column; overflow:hidden`; input tanpa border atas, padding 12px 14px, border-bawah `var(--border)`, `outline:none`; daftar `overflow-y:auto`; baris terpilih `background:var(--bg-hover)` + garis aksen kiri `var(--accent)`; label kelompok kapital kecil `var(--fg-muted)`. Elemen kontrol interaktif tidak memakai `--wails-draggable`. Tanpa animasi berat.

8. **Pendaftaran bawaan dan keamanan tipe.** Pastikan `registerBuiltinActions`/`registerBuiltinSources` hanya dipanggil dari `index.ts` (tidak dari `Palette.svelte`, agar aksi terdaftar walau komponen belum dipasang). Jalankan `cd frontend && npm run build` (dan `make build` dari root bila sempat); perbaiki semua galat tipe/svelte-check pada berkas di `modules/palette/`. Pastikan tidak ada `console.log`, tidak ada `setInterval`, timer hanya sekali jalan (250 ms), dan `package.json` tidak berubah.

## Perubahan

Dibuat (semua di `frontend/src/modules/palette/`):
- `registry.ts`
- `fuzzy.ts`
- `actions.ts`
- `sources.ts`
- `state.ts`
- `Palette.svelte`
- `index.ts`

Tidak ada file yang diubah di luar path ini.

## Wiring

Yang diekspos untuk langkah `shell-wiring` (JANGAN disambungkan di sini):
- `modules/palette/Palette.svelte` (default export, tanpa props wajib): dirender `App.svelte` sebagai sibling lapisan atas di dalam cabang shell, sehingga handler Cmd+K aktif selama aplikasi tampil.
- `modules/palette/index.ts`: `import '../palette'` di `modules/shell/register.ts` menjamin aksi dan sumber bawaan terdaftar (idempoten; juga dimuat otomatis oleh glob `main.ts`).
- API untuk modul lain/tiket kelak: `registerPaletteAction`, `registerPaletteSource`, `openPalette`, `closePalette`, `paletteOpen` dari `modules/palette`.

## Selesai bila

- Semua simbol kontrak 2.4 ada: `registry.ts` (`registerPaletteAction`, `registerPaletteSource`, tipe), `index.ts` mengekspor `Palette`, `paletteOpen`, `openPalette`, `closePalette`.
- Saat `Palette` dipasang: Cmd+K (capture, `code==='KeyK'`) membuka/menutup dari mana pun termasuk saat fokus di terminal atau input; Esc menutup; fokus kembali ke `document.activeElement` semula; panah/Enter/klik berfungsi.
- Pencarian fuzzy menyaring lintas Aksi, Sesi, Project, File (maks 8 per kelompok); memilih project -> `selectProject`, sesi -> `focusOrOpenSession`, file -> `openFilePane`; semua aksi bawaan (sesi baru claude/codex/shell, Open Folder…, Zoom in/out/reset, Toggle sidebar kiri/kanan, Susun grid) berjalan; aksi yang didaftarkan modul lain muncul tanpa mengubah kode palet.
- Batasan pencarian file (hanya file yang sudah dimuat `TreeModel`) tercatat di komentar `sources.ts`.
- Tanpa dependency baru, tanpa polling, tanpa `console.log`; tidak ada file di luar `frontend/src/modules/palette/` yang berubah.
- Guard: `cd frontend && npm run build` sukses (dan `make build` bila sempat). Tanpa unit test.
