# Plan: Pohon wiki lazy di panel

## Metadata
- Tiket: `lark-connector-ticket-04-lark-wiki` (`docs/tickets/lark-connector-ticket-04-lark-wiki.md`)
- Fitur: `lark-wiki-tree-ui` (Pohon wiki lazy di panel)
- ownedPaths: `frontend/src/modules/lark/sections/WikiTree.svelte`, `frontend/src/modules/lark/wikiTreeClient.ts`
- Depends on foundation: tidak (frontend mandiri; klien punya tipe dan `parseLarkError` lokal, lihat overview 1.9 dan 4)
- Prasyarat: tiket 02 sudah merge (ada `frontend/src/modules/lark/sections/WikiTree.svelte` placeholder dan `sections/WikiSection.svelte`). Bila `frontend/src/modules/lark/` belum ada, HENTIKAN dan laporkan; jangan membuat ulang berkas tiket 02.

Baca dulu `docs/plans/lark-connector-ticket-04-lark-wiki/overview.md` bagian 1 (keputusan 1.6, 1.7, 1.9), 2.2 (IPC), 2.4 (frontend). Plan ini tidak mengulangnya.

## Tujuan
Menimpa placeholder `sections/WikiTree.svelte` menjadi pohon wiki lazy per level untuk satu sumber (`source`), dengan cache memori saja, keadaan memuat/kosong/galat per node, tombol segarkan di akar dan per node, ikon per `objType`, keterangan `hiddenReason` untuk Pribadi, catatan `hasMore`, reset saat `source`/workspace berganti dan saat event `configChanged`, ARIA tree/treeitem dan keyboard. Tambah `wikiTreeClient.ts` sebagai pembungkus IPC `lark.wiki.list`. Props `{source, onOpen(nodeToken)}` TIDAK berubah.

## Path yang dimiliki
HANYA boleh disentuh:
- `frontend/src/modules/lark/sections/WikiTree.svelte` (timpa penuh)
- `frontend/src/modules/lark/wikiTreeClient.ts` (baru)

Jangan sentuh: `WikiSection.svelte`, `LarkIcon.svelte`, `client.ts`, `store.ts`, `source.ts`, `actions.ts`, `index.ts`, `doc/*`, `board/*`, `frontend/package.json`, berkas Go, atau file wiring lain. Jangan mengimpor `board/boardClient.ts`, `client.ts`, atau `doc/docClient.ts`. Tanpa dependensi baru.

## Kontrak
Disediakan:
- `wikiTreeClient.ts` mengekspor:
  ```ts
  export type LarkError = { code: string; message: string };
  export type WikiNode = { token: string; title: string; objType: string; objToken: string; hasChild: boolean };
  export type WikiListResult = { nodes: WikiNode[]; hasMore?: boolean; hidden?: boolean; hiddenReason?: string };
  export function parseLarkError(e: unknown): LarkError;
  export function listWikiNodes(workspaceId: string, sourceId: string, parentToken?: string): Promise<WikiListResult>;
  ```
- `WikiTree.svelte`: props `{ source, onOpen }`, `source: { id: string; name: string; spaceId?: string; rootToken?: string; host?: string }` (cukup `id` yang dipakai; mencakup `WikiSource` dan sumber Pribadi `{id:'personal', name:'Pribadi', spaceId:'my_library', host}`), `onOpen: (nodeToken: string) => void`. Semua node (docx, sheet, lainnya) memanggil `onOpen(node.token)` saat diklik; panah hanya membuka/menutup.

Dipakai:
- IPC `lark.wiki.list` `{workspaceId, sourceId, parentToken?}` -> `{nodes, hasMore?, hidden?, hiddenReason?}` (overview 2.2). `nodes` selalu array, maks 50 per level. Galat berupa string `"[<code>] <pesan>"`.
- `call`, `onEvent` dari `../../lib/ipc` (relatif dari `modules/lark/`: `../../lib/ipc`; dari `sections/`: `../../../lib/ipc`). Signature: `call<T>(method, params?)`, `onEvent(topicPrefix, fn(topic, payload)) => unsubscribe`.
- `activeWorkspaceId: Writable<string | null>` dari `lib/stores/workspaces` (`../../lib/stores/workspaces` dari `modules/lark/`).
- Event `workspace.changed` dengan payload `{kind: 'configChanged', workspaceId, workspace}`.
- Opsional: `openSettings('connectors')` dari `../../settings` bila fungsi itu diekspor (periksa dengan grep; bila tidak ada, tampilkan hanya teks petunjuk "Buka Pengaturan > Connector").
- Token tema CSS yang sudah ada dengan fallback: `--fg`, `--fg-muted`, `--border`, `--bg-elev`, `--accent`.

Periksa nama dan path impor persis di kode (tiket 02 sudah merge); laporkan selisih, jangan mengarang.

## Langkah

### 1. Periksa kode tiket 02
- Baca `sections/WikiTree.svelte` (placeholder) dan `sections/WikiSection.svelte` untuk memastikan bentuk props dan cara `WikiSection` memakai `WikiTree` (mis. apakah dibungkus `{#key}`). Jangan mengubah `WikiSection`.
- `grep -rn "export function openSettings" frontend/src/modules/settings` untuk tahu apakah tersedia.
- Pastikan Svelte 5 (runes: `$props`, `$state`, `$derived`, `$effect`); `files/viewer/MarkdownView.svelte` memakai runes, ikuti gaya itu.

### 2. Buat `frontend/src/modules/lark/wikiTreeClient.ts`
- Impor `call` dari `../../lib/ipc`.
- Tipe seperti bagian Kontrak.
- `parseLarkError(e: unknown): LarkError`: ambil string dari `e instanceof Error ? e.message : String(e)`; cocokkan regex `^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$` (flag `s`); bila cocok `{code: m[1], message: m[2] || m[1]}`; bila tidak `{code: 'unknown', message: raw}`.
- `listWikiNodes(workspaceId, sourceId, parentToken?)`: `try { const r = await call<WikiListResult>('lark.wiki.list', { workspaceId, sourceId, parentToken: parentToken || undefined }); return { ...r, nodes: r?.nodes ?? [] }; } catch (e) { throw parseLarkError(e); }`. Melempar `LarkError` (objek biasa), bukan `Error`.

### 3. Timpa `frontend/src/modules/lark/sections/WikiTree.svelte`
Gunakan `<script lang="ts">` dengan runes. Struktur:

**State (komponen, memori saja):**
```ts
type NodeState = { status: 'idle'|'loading'|'ready'|'error'; nodes: WikiNode[]; hasMore: boolean; error?: LarkError };
let { source, onOpen }: { source: {...}; onOpen: (nodeToken: string) => void } = $props();
let root = $state<NodeState & { hidden?: boolean; hiddenReason?: string }>(...idle...);
let children = $state<Record<string, NodeState>>({});   // kunci: token node induk
let expanded = $state<Record<string, boolean>>({});      // kunci: token node
let focusedKey = $state<string | null>(null);            // token node yang menerima tabindex=0
let seq = 0;                                             // penghitung generasi (stale guard)
```
Kunci root cache: gunakan string tetap `''` hanya internal; `root` dipisah dari `children`.

**Pemuatan:**
- `async function load(parent: string | null, force = false)`: ambil `ws = get(activeWorkspaceId)`; bila kosong -> set galat `{code:'no_workspace', message:'Workspace belum dipilih'}`. Tangkap `const gen = seq`; set `status='loading'`; panggil `listWikiNodes(ws, source.id, parent ?? undefined)`; setelah await, bila `gen !== seq` abaikan hasil (respons usang). Sukses -> `status='ready'`, `nodes`, `hasMore = !!r.hasMore`; untuk akar simpan juga `hidden`, `hiddenReason`. Gagal -> `status='error'`, `error = parseLarkError(e)` (aman: tidak melempar keluar komponen). `force=false` dan status sudah `ready` -> tidak memuat ulang (cache).
- Akar: dimuat otomatis saat mount dan saat reset (efek di bawah). Anak: dimuat saat node pertama kali dibuka (`status==='idle'`); membuka lagi memakai cache.
- Segarkan akar: tombol di header kecil bar atas (ikon refresh + `aria-label="Segarkan pohon"`) -> `resetAll(false)`-nya cukup: kosongkan `children` dan `expanded`, `seq++`, `load(null, true)`. Segarkan per node (tombol ikon kecil di baris node yang terbuka/punya anak, `aria-label="Segarkan {title}"`): `seq`-nya tidak dinaikkan; `load(node.token, true)` hanya untuk node itu (anak dari node itu yang sudah di-cache dibiarkan; boleh dihapus dengan menghapus kunci anak-anaknya dari `children` agar dimuat ulang saat dibuka).
- Tombol "Coba lagi" pada keadaan galat (akar atau node) memanggil `load(...,true)` untuk level itu.

**Reset:**
- `$effect` yang bergantung pada `source.id` dan `$activeWorkspaceId` (gunakan store `$activeWorkspaceId` auto-subscribe): `seq++`, kosongkan `children`, `expanded`, `focusedKey`, set `root` ke idle lalu `load(null, true)`. Pastikan efek tidak membuat loop (jangan membaca `root`/`children` secara reaktif di dalam efek selain menulis; gunakan `untrack` bila perlu).
- Event: `onMount` pasang `const off = onEvent('workspace.changed', (_t, p) => { const pl = p as any; if (pl?.kind !== 'configChanged') return; const ws = get(activeWorkspaceId); if (pl.workspaceId && ws && pl.workspaceId !== ws) return; resetAll(); })`; `return off` (cleanup). `resetAll` = sama dengan reset di atas (kosongkan cache dan muat ulang akar; node yang terbuka ikut tertutup, boleh).
- Tanpa polling otomatis.

**Tampilan:**
- Kontainer: `<div class="wikitree">` berisi bar atas kecil (nama `source.name` TIDAK ditampilkan ulang bila `WikiSection` sudah menampilkannya; cukup tombol segarkan di kanan) lalu `<ul role="tree" aria-label="Pohon wiki {source.name}">`.
- Keadaan akar: `loading` -> baris teks "Memuat..." (`role="status"`); `error` -> pesan galat + "Coba lagi"; bila kode `cli_missing` atau `not_logged_in` tampilkan petunjuk "Hubungkan Lark di Pengaturan > Connector" dan, bila `openSettings` ada, tombol yang memanggil `openSettings('connectors')`; `ready` + `hidden` -> keterangan bergaya redup berisi `hiddenReason` (bukan galat; bila kosong pakai "Wiki ini tidak dapat ditampilkan"); `ready` + `nodes.length===0` dan tidak hidden -> "Belum ada isi".
- Node dirender rekursif lewat `{#snippet nodeList(nodes: WikiNode[], level: number)}` (snippet bisa memanggil dirinya sendiri; hindari komponen rekursif terpisah karena tidak boleh membuat berkas baru). Tiap node `<li role="none">` berisi `<div role="treeitem" aria-level={level} aria-expanded={hasChild ? expanded : undefined} aria-selected="false" tabindex={focusedKey===token?0:-1} data-token=...>` dengan: tombol/ikon panah (hanya bila `hasChild`; kalau tidak, spacer selebar sama), ikon objType, judul (`title || 'Tanpa judul'`, ellipsis, `title` atribut penuh), tombol segarkan per node (muncul saat hover/fokus, hanya bila `hasChild`). Anak dibungkus `<ul role="group">` saat `expanded`.
- Klik pada panah: `toggle(node)` dengan `e.stopPropagation()`; TIDAK memanggil `onOpen`. Klik pada baris (di luar panah dan tombol segarkan) -> `onOpen(node.token)`.
- Keadaan anak (di bawah node terbuka): `loading` -> "Memuat..."; `error` -> pesan + "Coba lagi"; `ready` + kosong -> "Belum ada isi"; indentasi mengikuti `level` (`padding-left: calc(var(--indent) * level)`).
- `hasMore` (akar atau anak): baris catatan redup "Menampilkan 50 pertama" di akhir daftar level itu (tanpa tombol muat lagi, overview 1.6).

**Ikon (inline SVG di dalam file, ukuran 14px, `currentColor`):**
- `objType === 'docx'` (juga `doc`): ikon dokumen; `'sheet'`: ikon tabel/grid; selain itu (bitable, mindnote, file, slides, dst): ikon generik (berkas/tautan). Panah: chevron yang diputar 90 derajat saat terbuka (transisi singkat). Ikon refresh: panah melingkar. Semua ikon `aria-hidden="true"`; tombol punya `aria-label`.

**Keyboard (pola WAI-ARIA tree):**
- Fokus roving: satu `treeitem` ber-`tabindex=0` (default node pertama yang terlihat; perbarui `focusedKey` saat fokus/klik).
- `ArrowDown`/`ArrowUp`: pindah ke node terlihat berikutnya/sebelumnya (hitung daftar datar node terlihat via fungsi `visibleItems()` yang menelusuri `root.nodes` dan `children` untuk node `expanded`; fokuskan lewat `querySelector('[data-token="..."]')` setelah `tick()`).
- `ArrowRight`: bila `hasChild` dan tertutup -> buka (muat bila perlu); bila sudah terbuka -> pindah ke anak pertama (bila ada).
- `ArrowLeft`: bila terbuka -> tutup; bila tertutup/tanpa anak -> pindah ke node induk (perlu peta `parentOf[token]` yang diisi saat memuat anak).
- `Home`/`End`: node terlihat pertama/terakhir.
- `Enter` dan `Space`: `onOpen(node.token)` (Space mencegah scroll dengan `preventDefault`). Panah tidak memanggil `onOpen`.
- Tombol segarkan dan "Coba lagi" adalah `<button>` biasa (dapat difokus dengan Tab); `keydown` dari tombol tidak boleh memicu navigasi pohon (periksa `e.target === e.currentTarget` di handler treeitem).

**Gaya:** blok `<style>` lokal, tanpa warna keras; pakai `var(--fg, inherit)`, `var(--fg-muted, ...)`, `var(--border, ...)`, `var(--bg-elev, rgba(127,127,127,.12))`, `var(--accent, ...)` dengan fallback seperti `files/viewer/MarkdownView.svelte`. Hover dan fokus: `background: var(--bg-elev)`, outline fokus `2px solid var(--accent)`. Teks kecil (12-13px), baris tinggi sekitar 24px, tanpa scroll horizontal (ellipsis).

### 4. Verifikasi build
- Dari `frontend/`: jalankan build frontend (`npm run build`; bila tersedia `npm run check`/`svelte-check`, jalankan juga). Perbaiki galat tipe/impor di dua berkas milik fitur ini saja.
- `git status` hanya boleh menampilkan dua berkas milik fitur ini (satu baru, satu diubah).

## Perubahan
- Baru: `frontend/src/modules/lark/wikiTreeClient.ts`
- Diubah (ditimpa penuh): `frontend/src/modules/lark/sections/WikiTree.svelte`

## Wiring
Tidak ada yang disambungkan oleh fitur ini. Yang diekspos untuk langkah wiring (`lark-wiki-wiring`, hanya verifikasi):
- `WikiTree.svelte` dengan props `{source, onOpen(nodeToken: string)}` yang identik dengan placeholder, sehingga `WikiSection.svelte` (tiket 02, tidak diubah) langsung memakainya. `onOpen` dipanggil dengan `node.token` untuk semua `objType`; `WikiSection` yang membuka pane `lark-doc` (`layout.openPane({kind:'lark-doc', sourceId, nodeToken, title})`).
- `wikiTreeClient.ts` (`listWikiNodes`, `parseLarkError`) bergantung pada IPC `lark.wiki.list` milik `lark-wiki-data`; sebelum IPC itu ada, pohon menampilkan keadaan galat (bukan crash). Jangan menyambung apa pun di `index.ts`, `WikiSection`, atau daemon.

## Selesai bila
- Build frontend lulus (guard: `npm run build` di `frontend/` sukses; tanpa unit test).
- `WikiTree.svelte` memakai props `{source, onOpen}` yang tidak berubah; akar dimuat saat tampil; anak dimuat lazy saat pertama dibuka dan di-cache di memori; tanpa penyimpanan disk/localStorage.
- Keadaan memuat, kosong ("Belum ada isi"), galat dengan "Coba lagi" tersedia per level/node; segarkan di akar dan per node berfungsi; respons usang diabaikan lewat `seq`.
- Ikon berbeda untuk docx, sheet, dan lainnya (inline di `WikiTree.svelte`); `hiddenReason` tampil sebagai keterangan untuk hasil `hidden`; catatan "Menampilkan 50 pertama" muncul bila `hasMore`.
- Klik/Enter/Space pada node memanggil `onOpen(node.token)` untuk semua tipe; panah (klik dan keyboard) hanya membuka/menutup.
- Pohon di-reset dan dimuat ulang saat `source.id` atau workspace aktif berganti dan saat event `workspace.changed` ber-`kind === 'configChanged'`.
- Atribut ARIA `tree`/`group`/`treeitem` (`aria-level`, `aria-expanded`) dan navigasi keyboard (panah, Home, End, Enter) berfungsi.
- `git status` hanya menunjukkan dua berkas milik fitur ini; tidak ada perubahan di `WikiSection`, `LarkIcon`, `package.json`, atau berkas tiket lain.
