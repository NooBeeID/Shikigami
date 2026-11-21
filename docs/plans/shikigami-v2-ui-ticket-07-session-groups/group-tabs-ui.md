# Plan: Tab grup dan pemindahan sesi

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-07-session-groups.md` (shikigami-v2-ui-ticket-07-session-groups)
- Fitur: `group-tabs-ui` (Tab grup dan pemindahan sesi)
- ownedPaths: `frontend/src/modules/groups/`
- Depends on foundation: ya (`00-foundation.md`: stub `lib/stores/groups.ts`, registri `modules/shell/registry.ts` dan `modules/sidebar-left/registry.ts`). Foundation sudah ada sebelum fitur ini dikerjakan. Fitur ini berjalan paralel dengan `groups-store` (yang menimpa stub) dan `group-daemon`, jadi hanya boleh bergantung pada kontrak di bagian "Kontrak", bukan detail dalam `groups.ts`.

Rujukan: `overview.md` bagian 2.3 (store), 2.4 (registri slot), 2.5 (komponen UI, konvensi DOM), 2.6 (penamaan), 7 (aturan paralel).

## Tujuan

Membangun modul UI `modules/groups`: deretan tab grup di atas area layout (nama + jumlah sesi, `+`, menu konteks Ganti nama/Hapus, edit nama inline, klik ganda, drag urut dengan pointer events), menu "Pindah ke grup" per sesi, daftar sesi sidebar kiri yang mengutamakan grup aktif (grup lain dilipat), serta drag sesi (baris sidebar dan header pane) ke tab grup untuk memindahkannya. Semua state grup berasal dari `lib/stores/groups.ts`; modul ini hanya menyimpan state UI sementara di memori komponen. Modul terpasang hanya lewat registrasi slot di `index.ts`.

## Path yang dimiliki

HANYA `frontend/src/modules/groups/` (berkas baru: `GroupTabs.svelte`, `GroupSessionMenu.svelte`, `GroupedSessionList.svelte`, `sessionDrag.ts`, `index.ts`).

Dilarang menyentuh: `modules/shell/*` (termasuk `ShellFrame.svelte`), `modules/sidebar-left/*` (termasuk `SessionRow.svelte`, `SessionGroups.svelte`, `AgentSidebar.svelte`), `modules/layout/*`, `modules/persist/*`, `lib/stores/*` (termasuk `groups.ts`, `types.ts`), `main.ts`, `App.svelte`, `internal/`, `cmd/`, `app/`, dan path fitur lain. Tidak ada dependency baru, tidak ada `setInterval`/polling, tidak ada `alert`, tidak ada `localStorage`/`sessionStorage`, tidak ada warna hardcode (pakai variabel tema `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok`; ukuran `rem`/`em`).

## Kontrak

Dipakai (tidak diubah), semua dari `overview.md`:

- Store `frontend/src/lib/stores/groups.ts` (overview 2.3), impor LANGSUNG dari berkasnya, bukan dari `lib/stores/index.ts`:
  `SessionGroup {id,name,order,main}`, `MAIN_GROUP_NAME`, store `groups`, `mainGroupId`, `activeGroupId`, `activeGroup`, `sessionGroupMap`, `groupSessionCounts`, `groupsError`; fungsi `initGroups(): () => void`, `createGroup(name?): Promise<string>`, `renameGroup(id,name): Promise<void>`, `deleteGroup(id): Promise<void>`, `reorderGroup(id,toIndex): Promise<void>`, `setActiveGroup(id): Promise<void>`, `moveSession(sessionId,toGroupId): Promise<void>`, `groupOfSession(sessionId): string | undefined`. Semua aksi bisa melempar galat; galat daemon juga muncul di `groupsError`. Komponen menangkap galat (`try/catch`, abaikan karena sudah tampil lewat `groupsError`).
- Registri shell: `registerLayoutHeader({id, component, order?})` dari `../shell/registry`.
- Registri sidebar kiri: `registerSessionListWrapper({id, component})`, `registerSessionMenuItem({id, component, order?})`, tipe `SessionListWrapperProps {onerror: (msg:string)=>void; list: Snippet<[filter:(s:Session)=>boolean]>}`, `SessionMenuItemProps {sessionId:string; onDone?:()=>void}` dari `../sidebar-left/registry` (BUKAN dari `../shell/index` atau `../sidebar-left/index`).
- `sessions` dan tipe `Session` dari `../../lib/stores` (`Session {id, workspaceId, projectId, cwd, providerId, status, createdAt, exitCode?}`).
- `focusOrOpenSession(sessionId): boolean` dan `sessionTitle(sessionId): string` dari `../shell/helpers` (impor dari berkas helpers langsung bila `../shell` index memicu siklus; `modules/sidebar-left/SessionRow.svelte` memakai `../shell`, keduanya valid).
- `layout` (`subscribe`) dari `../../lib/stores/layout`; `leaves`, `findLeaf`, tipe `Leaf` dari `../layout/model` (hanya impor, jangan ubah). `Leaf` terminal punya `kind === 'terminal'` dan `sessionId`.
- `dragState: Readable<DragState|null>` dari `../layout/dnd` (`DragState {paneId, title, targetId, zone, rect}`; TIDAK membawa `sessionId`, jadi `sessionId` dicari dari leaf `paneId` di dokumen `layout`; pane non-terminal diabaikan).
- Konvensi DOM bersama dengan Wiring: tab grup memakai `data-group-tab-id="<groupId>"` (disediakan modul ini); elemen akar baris sesi sidebar kiri punya `data-session-id="<sessionId>"` (ditambahkan Wiring ke `SessionRow.svelte`; modul ini hanya membacanya).

Disediakan:

- `GroupTabs.svelte`: tanpa props wajib.
- `GroupSessionMenu.svelte`: props `SessionMenuItemProps` (`{sessionId, onDone?}`).
- `GroupedSessionList.svelte`: props `SessionListWrapperProps`.
- `index.ts`: efek samping saat dimuat (glob `main.ts`): `initGroups()` dan tiga registrasi slot di bagian Wiring.
- `sessionDrag.ts` (internal modul): `startSessionDrag(e: PointerEvent, sessionId: string, title: string): void` dan `hoveredGroupTabId(x: number, y: number): string | null`; `onGroupTabDragState: Readable<{ sessionId: string; overGroupId: string | null } | null>` (state UI sementara agar tab bisa menyala sebagai zona drop).

## Langkah

Semua berkas Svelte 5 runes (`$props`, `$state`, `$derived`, `$effect`), `<script lang="ts">`. Teks UI Indonesia, identifier Inggris. Sebelum menulis, baca `modules/layout/PaneMenu.svelte` dan `modules/sidebar-left/SessionRow.svelte` untuk gaya menu pop-up (tutup saat klik luar / Esc) dan `modules/sidebar-left/SessionGroups.svelte` untuk gaya judul bagian; ikuti gaya itu tanpa mengimpor internalnya.

### 1. `sessionDrag.ts` (drag sesi ke tab, dan deteksi drag header pane)

1. Store internal `writable<{sessionId: string; overGroupId: string | null} | null>(null)` yang diekspor read-only sebagai `groupDropState` (ekspor `Readable`). Dipakai `GroupTabs` untuk menyalakan tab sebagai zona drop (`overGroupId` = id tab yang sedang dilewati; tab lain yang valid tetap diberi gaya "dapat dijatuhkan" ringan selama `groupDropState` tidak null).
2. `hoveredGroupTabId(x, y): string | null`: `document.elementsFromPoint(x, y)`, cari elemen pertama yang `closest('[data-group-tab-id]')`, kembalikan `dataset.groupTabId` atau `null`.
3. `startSessionDrag(e, sessionId, title)`: pola dari `modules/layout/dnd.ts` (baca saja, jangan impor internalnya, jangan ubah):
   - Abaikan bila `e.button !== 0` atau sudah ada drag aktif (flag modul).
   - Catat posisi awal; drag baru mulai setelah pergeseran >= 4px (`Math.hypot`), supaya klik biasa pada baris tetap berfungsi.
   - Saat mulai: buat ghost `div` (`position:fixed; pointer-events:none; z-index:10000`, teks `title`, gaya variabel tema `--bg-elev`/`--border`/`--fg`), `document.body.style.userSelect='none'`, set `groupDropState` ke `{sessionId, overGroupId: null}`.
   - `pointermove` (window): gerakkan ghost; `overGroupId = hoveredGroupTabId(...)`; set store bila berubah. Bila `overGroupId` tab non-aktif, jadwalkan `setTimeout` sekali jalan 600 ms yang memanggil `setActiveGroup(id)` (batalkan/ganti timeout saat tab yang dilewati berubah atau pointer keluar; bukan polling).
   - `pointerup`: bila sudah mulai dan `overGroupId` ada dan berbeda dari `groupOfSession(sessionId) ?? get(mainGroupId)`, panggil `moveSession(sessionId, overGroupId)` (tangkap galat). Lalu bersihkan. Setelah drag yang sudah mulai, telan satu `click` berikutnya (listener `click` capture sekali pakai + `setTimeout(...,0)` untuk melepasnya) agar baris tidak terbuka.
   - `Escape`, `pointercancel`, `blur`: batal tanpa pemindahan. Cleanup selalu: hapus semua listener, ghost, timeout, kembalikan `userSelect`, `groupDropState.set(null)`.
4. Drag header pane (tanpa mengubah modul layout): ekspor `watchPaneDrag(): () => void` yang `GroupTabs` panggil di `$effect` (cleanup dikembalikan). Isinya: `dragState.subscribe`. Saat berubah dari `null` ke non-null: ambil leaf `findLeaf(get(layout), state.paneId)`; bila `leaf?.kind === 'terminal' && leaf.sessionId`, simpan `sessionId` dan pasang listener `pointermove` (capture, pada `window`) yang memperbarui `lastOverGroupId = hoveredGroupTabId(...)` dan `groupDropState` (`{sessionId, overGroupId}`) dengan timeout hover 600 ms yang sama; pasang juga `keydown` Escape (capture) dan `pointercancel` yang mengosongkan `lastOverGroupId`. Saat `dragState` kembali `null`: bila `lastOverGroupId` ada dan bukan grup sesi saat ini, panggil `moveSession(sessionId, lastOverGroupId)`; lepas listener, `groupDropState.set(null)`, batalkan timeout. Pane non-terminal (file) tidak menyala dan tidak dipindah. Karena ghost dan pointer capture dikelola layout, modul ini tidak membuat ghost kedua untuk jalur ini. Jangan pernah memanggil `layout.movePane`.

### 2. `GroupTabs.svelte`

Komponen akar: `<div class="group-tabs" role="tablist" aria-label="Grup sesi">`, tinggi ringkas, `display:flex`, `overflow-x:auto` tanpa pemecahan baris (`flex-wrap: nowrap`, tab `flex:0 0 auto`), latar `--bg-elev`, garis bawah `--border`. Impor store dari `../../lib/stores/groups` (`groups`, `activeGroupId`, `groupSessionCounts`, `groupsError`, aksi), `watchPaneDrag`, `groupDropState` dari `./sessionDrag`.

State UI sementara (memori komponen): `editingId: string | null`, `draft: string`, `menuFor: string | null` (id tab yang menunya terbuka, plus koordinat `menuPos {x,y}`), `confirmDeleteId: string | null`, `reorderDrag: {id, overIndex} | null`, `localError: string | null`.

Per tab (`{#each $groups as g (g.id)}`), tombol `role="tab"`, `aria-selected={g.id === $activeGroupId}`, `tabindex` roving (0 untuk aktif, -1 lainnya), `data-group-tab-id={g.id}`, kelas `active` (gaya aksen: garis bawah/teks `--accent`), kelas `drop` bila `$groupDropState` non-null (garis putus `--border`) dan `drop-over` bila `$groupDropState?.overGroupId === g.id` (latar `--bg-hover`, kontur `--accent`):
- Isi: nama (`text-overflow: ellipsis`, `max-width` dalam `rem`), lencana jumlah `$groupSessionCounts[g.id] ?? 0` (kecil, `--fg-muted`), dan tombol `...` (`aria-label="Menu grup"`, `aria-haspopup="menu"`, tampil saat hover/fokus/aktif).
- Klik: `setActiveGroup(g.id)` (tangkap galat). Klik diabaikan bila baru selesai drag urut atau sedang edit.
- Klik ganda: `startEdit(g.id)`.
- Klik kanan (`oncontextmenu`, `preventDefault`) dan klik `...`: buka menu konteks.
- Keyboard: panah kiri/kanan memindahkan fokus ke tab sebelumnya/berikutnya (siklik) dan mengaktifkannya (`setActiveGroup`); Home/End ke ujung; F2 mulai edit; Delete pada grup non-`main` membuka konfirmasi hapus; Shift+F10/`ContextMenu` membuka menu.
- Tab aktif digulir ke tampilan: `$effect` yang bergantung pada `$activeGroupId` dan `$groups`, memanggil `el.scrollIntoView({inline: 'nearest', block: 'nearest'})` pada tab aktif (lewat `bind:this` ke map elemen atau `querySelector('[data-group-tab-id="..."]')` di akar).

Edit nama inline: bila `editingId === g.id`, ganti label dengan `<input>` (`bind:value={draft}`, `maxlength` tidak dipasang karena batas 40 diputuskan daemon; fokus dan `select()` otomatis lewat `$effect`/action). `Enter`: `commitEdit()`. `Esc`: batal (`editingId = null`, tanpa panggilan). `blur`: bila `draft.trim()` kosong atau sama dengan nama lama, batal; selain itu `commitEdit()`. `commitEdit()`: bila `draft.trim()` kosong tampilkan `localError = 'Nama grup tidak boleh kosong'` dan pertahankan mode edit (Enter) atau batal (blur); selain itu `await renameGroup(id, draft)` (tangkap galat) lalu `editingId = null`. Nama final yang tampil adalah yang dari store (sudah memuat akhiran daemon bila bentrok); jangan menulis nama lokal. Hentikan propagasi `keydown`/`pointerdown` input agar panah/Spasi tidak mengubah tab dan drag urut tidak mulai.

Tombol `+` di ujung (setelah semua tab, `aria-label="Grup baru"`, `title="Grup baru"`): `const id = await createGroup(); startEdit(id)` (`createGroup` sudah mengaktifkan grup baru; `startEdit` memakai nama dari `$groups` yang menemukan `id`; bila belum muncul di store tunggu lewat `$effect` yang memantau `pendingEditId`: saat `$groups` memuat id itu, set `editingId` dan kosongkan `pendingEditId`). Tangkap galat.

Menu konteks (`role="menu"`, posisi `fixed` di dekat tab/titik klik, dijepit dalam viewport; tutup lewat klik di luar `pointerdown` window, `Escape`, dan setelah memilih; ikuti pola `PaneMenu`/`SessionRow` agar konsisten):
- `Ganti nama` (`role="menuitem"`): `startEdit(menuFor)`.
- `Hapus` (`role="menuitem"`, kelas `danger`, warna `--danger`): TIDAK dirender sama sekali untuk grup `main` (`g.main`). Untuk grup lain: bila `($groupSessionCounts[id] ?? 0) > 0`, set `confirmDeleteId = id` dan tampilkan konfirmasi ringkas di tempat (blok kecil di bawah tab/pada menu): "Hapus grup ini? Sesi di dalamnya dipindah ke "Utama" dan tidak dihentikan." dengan tombol `Hapus` dan `Batal` (nama "Utama" boleh dari `MAIN_GROUP_NAME`). Bila tidak ada sesi, langsung `deleteGroup(id)`. Saat konfirmasi: `await deleteGroup(id)` (tangkap galat), tutup konfirmasi.

Drag urut tab (pointer events sendiri):
- `onpointerdown` pada tab (tombol 0, bukan dari input atau tombol `...`): catat `startX`, `pointerId`, `id`. `pointermove` window: setelah >= 4px mulai `reorderDrag`; hitung indeks sisip dari titik tengah tab lain (`getBoundingClientRect` per tab dengan `[data-group-tab-id]`, bandingkan `clientX` dengan titik tengah): `overIndex` dalam 0..n-1 (posisi akhir yang diinginkan setelah pencabutan). Tampilkan penanda sisip: garis vertikal `--accent` (2px) sebagai pseudo-elemen/elemen absolut pada tepi kiri tab target (atau tepi kanan tab terakhir) via kelas `insert-before`/`insert-after`; tab yang diseret diberi `opacity: 0.5`. Gunakan `setPointerCapture` pada tab sumber; tanpa ghost (tab itu sendiri terlihat redup).
- `pointerup`: bila `overIndex` valid dan berbeda dari indeks asal, `await reorderGroup(id, overIndex)` (tangkap galat); setelah drag yang sudah mulai telan `click` berikutnya (pola sama dengan sessionDrag). `Escape`/`pointercancel`: batal. Tidak ada `setInterval`; auto-scroll tepi tidak diperlukan.
- Drag urut tidak boleh bentrok dengan drop sesi: `groupDropState` hanya diset oleh drag sesi/pane; drag urut tidak menyentuhnya.

Galat: tampilkan `$groupsError ?? localError` ringkas di ujung kanan area tab (`span role="status"`, warna `--danger`, `text-overflow: ellipsis`, `title` berisi pesan penuh) dengan tombol kecil tutup untuk `localError`; bukan `alert`. `$effect` memanggil `watchPaneDrag()` dan mengembalikan cleanup-nya.

### 3. `GroupSessionMenu.svelte`

Props `{ sessionId, onDone }: SessionMenuItemProps` (`$props()`). Tampil sebagai satu item menu bergaya sama dengan item di menu `...` `SessionRow` (tombol penuh lebar, `role="menuitem"`, teks "Pindah ke grup", tanda panah kanan). Klik/Enter/Panah kanan membuka submenu inline (daftar vertikal di bawah item, atau flyout di kanan bila ruang cukup; pilih inline agar tidak terpotong `overflow` menu induk). Daftar: `{#each $groups}` semua grup; grup asal (`$sessionGroupMap[sessionId] ?? $mainGroupId`) ditandai "(saat ini)" dan dinonaktifkan (`disabled`, `aria-disabled`). Memilih grup lain: `await moveSession(sessionId, g.id)` (tangkap galat, tampil lewat `groupsError`) lalu `onDone?.()`. Bila hanya satu grup, item tetap tampil tetapi submenu berisi teks "Tidak ada grup lain" (`--fg-muted`). Esc menutup submenu. Warna dari variabel tema. Tanpa state selain `open` (UI sementara).

### 4. `GroupedSessionList.svelte`

Props `SessionListWrapperProps`: `let { onerror, list }: SessionListWrapperProps = $props();`. Struktur:

```
<div class="grouped-list" bind:this={root} onclickcapture={onRowClickCapture} onpointerdown={onRowPointerDown}>
  {#each ordered as g (g.id)}
    <section class="gl-section" data-group-section-id={g.id}>
      <button class="gl-head" aria-expanded={isOpen(g.id)} onclick={() => toggle(g.id)}>
        <span class="chev">...</span> <span class="name">{g.name}</span> <span class="count">{count}</span>
      </button>
      {#if isOpen(g.id)}
        <div class="gl-body">{@render list(filterFor(g.id))}</div>
      {/if}
    </section>
  {/each}
</div>
```

- `ordered` ($derived): grup aktif lebih dulu, lalu grup lain urut `order` (urutan `$groups`).
- `filterFor(gid)`: `(s: Session) => ((($sessionGroupMap[s.id]) ?? $mainGroupId) === gid)`. Sesi dengan entri ke grup yang tidak dikenal dianggap `main` (hanya pengaman: `known.has(entry) ? entry : $mainGroupId`). Fungsi dibuat per render dan reaktif karena membaca store lewat `$` di dalam closure (hindari kapturan nilai basi: baca `get(sessionGroupMap)` di dalam filter atau jadikan `$derived` map yang dirujuk filter).
- Lipat: `collapsed = $state<Record<string, boolean>>({})` (hanya UI). Default: grup aktif terbuka, grup lain terlipat (nilai `undefined` dihitung sebagai default sesuai `g.id === $activeGroupId`); klik judul membalik. Saat grup aktif berganti, grup aktif baru otomatis terbuka (hapus override miliknya lewat `$effect` pada `$activeGroupId`). Hitungan di judul: `$groupSessionCounts[g.id] ?? 0`. Grup tanpa sesi tetap menampilkan judul (isi kosong: teks "Belum ada sesi" `--fg-muted` kecil bila terbuka dan hitungan 0).
- Klik baris di bagian grup LAIN: `onclickcapture` pada akar: ambil `const row = (e.target as HTMLElement).closest('[data-session-id]')`; bila tidak ada, biarkan. Cari `section` terdekat (`closest('[data-group-section-id]')`) -> `gid`. Bila `gid !== $activeGroupId` dan klik bukan pada tombol/menu di dalam baris (`closest('button, [role=menu]')` yang bukan akar baris itu sendiri; bila ragu, hanya tangani klik yang target-nya di badan baris dan abaikan kontrol `...`): `e.stopPropagation(); e.preventDefault();` lalu `await setActiveGroup(gid); focusOrOpenSession(sessionId);` (tangkap galat; bila `setActiveGroup` melempar, jangan fokus). Klik di grup aktif dibiarkan ke handler `SessionRow` seperti biasa.
- Drag sesi sidebar: `onRowPointerDown(e)`: `const row = target.closest('[data-session-id]')`; abaikan bila tidak ada, bila `e.button !== 0`, bila target di dalam `button`/`input`/menu `...` yang bukan badan baris (`closest('button, input, [role=menu]')` dan elemen itu berbeda dari `row` sendiri), lalu `startSessionDrag(e, sessionId, sessionTitle(sessionId))` (dari `./sessionDrag`; `sessionTitle` dari `../shell/helpers`). Jangan memanggil `preventDefault` pada `pointerdown` (agar klik biasa tetap jalan). Tidak memakai HTML5 drag.
- `onerror` diteruskan hanya bila komponen sendiri melaporkan galat (mis. `setActiveGroup` gagal pada klik baris: `onerror(errMessage-ish)`); gunakan `e instanceof Error ? e.message : String(e)` agar tidak mengimpor modul lain.
- Gaya: judul bagian kecil, huruf kapital opsional, warna `--fg-muted`, hover `--bg-hover`, grup aktif judul `--fg` dengan penanda `--accent` tipis; padding `rem`.

### 5. `index.ts`

```ts
import { initGroups } from '../../lib/stores/groups';
import { registerLayoutHeader } from '../shell/registry';
import { registerSessionListWrapper, registerSessionMenuItem } from '../sidebar-left/registry';
import GroupTabs from './GroupTabs.svelte';
import GroupedSessionList from './GroupedSessionList.svelte';
import GroupSessionMenu from './GroupSessionMenu.svelte';

initGroups();
registerLayoutHeader({ id: 'groups', component: GroupTabs, order: 0 });
registerSessionListWrapper({ id: 'groups', component: GroupedSessionList });
registerSessionMenuItem({ id: 'move-to-group', component: GroupSessionMenu, order: 50 });
```

`initGroups()` mengembalikan cleanup; modul dimuat sekali seumur aplikasi sehingga cleanup tidak dipakai (boleh disimpan di variabel modul `void`-kan). Jangan ekspor komponen dari `index.ts` kecuali perlu (opsional: `export { GroupTabs, GroupedSessionList, GroupSessionMenu }`). Jangan mengimpor `index.ts` modul `shell`/`sidebar-left` (hindari siklus muat; impor registri langsung).

### 6. Verifikasi build

1. `cd frontend && npm run build` (dan `npm run check` bila skripnya ada). Perbaiki semua galat tipe/svelte-check yang berasal dari `modules/groups/`.
2. Periksa grep di `frontend/src/modules/groups/`: tidak ada `localStorage`, `sessionStorage`, `alert(`, `setInterval`, warna hardcode (`#`, `rgb(`, nama warna) kecuali fallback dalam `var(--x, ...)` yang tidak dipakai, dan tidak ada impor ke `lib/stores/index`.
3. `git status` (atau daftar berkas): hanya `frontend/src/modules/groups/` yang berubah.

## Perubahan

Dibuat (semua di `frontend/src/modules/groups/`):
- `GroupTabs.svelte`
- `GroupSessionMenu.svelte`
- `GroupedSessionList.svelte`
- `sessionDrag.ts`
- `index.ts`

Tidak ada berkas lain yang diubah.

## Wiring

Yang diekspos modul ini untuk disambungkan Wiring (jangan disambungkan oleh fitur ini):
- `modules/groups/index.ts` dimuat otomatis oleh glob `main.ts`; saat dimuat memanggil `initGroups()` dan mendaftarkan: `registerLayoutHeader({id:'groups', component: GroupTabs, order: 0})`, `registerSessionListWrapper({id:'groups', component: GroupedSessionList})`, `registerSessionMenuItem({id:'move-to-group', component: GroupSessionMenu, order: 50})`.
- Host yang harus dirender Wiring: `LayoutHeaderHost` di `ShellFrame.svelte` (merender `GroupTabs`), `AgentSidebar.svelte` (merender `GroupedSessionList` dengan `list` snippet yang memanggil `SessionGroups` ber-prop `filter`), dan `SessionRow.svelte` (merender `GroupSessionMenu` lewat `sessionMenuItems()` di menu `...`, dan menambah `data-session-id={session.id}` pada elemen akar baris).
- Kebutuhan DOM dari Wiring: `data-session-id` pada akar baris sesi (dibaca `GroupedSessionList` untuk klik di grup lain dan drag sidebar). Modul ini menyediakan `data-group-tab-id` pada tab (dibaca oleh `sessionDrag.ts`).
- Batasan yang perlu dicatat Wiring: menu `...` header pane (`modules/layout/PaneMenu.svelte`) tidak punya titik ekstensi, jadi "Pindah ke grup" hanya lewat menu sesi sidebar kiri dan drag header pane ke tab.

## Selesai bila

- Guard: `cd frontend && npm run build` sukses (dan `npm run check` bila ada) tanpa galat tipe/svelte pada `modules/groups/`. Tanpa unit test.
- Lima berkas di atas ada dan hanya `frontend/src/modules/groups/` yang berubah; tidak ada dependency baru, `setInterval`, `alert`, `localStorage`/`sessionStorage`, atau warna hardcode.
- `GroupTabs` memakai `role="tablist"`/`role="tab"` dengan `aria-selected`, panah kiri/kanan berpindah tab, tab menampilkan nama dan jumlah sesi, `+` membuat grup lalu membuka edit inline (Enter simpan, Esc batal, kosong ditolak), klik ganda dan menu konteks "Ganti nama" berfungsi, "Hapus" tidak dirender untuk grup `main` dan memakai konfirmasi bila grup punya sesi, drag tab memanggil `reorderGroup` dengan penanda sisip, dan `groupsError` tampil ringkas di area tab.
- `GroupSessionMenu` menampilkan daftar grup dengan grup asal ditandai dan memanggil `moveSession`.
- `GroupedSessionList` menampilkan grup aktif terbuka dulu, grup lain terlipat dengan hitungan, merender baris lewat `list(filter)`, mengaktifkan grup lalu `focusOrOpenSession` saat baris grup lain diklik, dan memulai drag sesi lewat pointer events.
- Drag sesi (sidebar) dan drag header pane terminal (lewat `dragState`) ke tab grup menyalakan tab sebagai zona drop dan memanggil `moveSession` saat dilepas; hover ~600 ms mengaktifkan tab (timeout sekali jalan).
- `index.ts` hanya memanggil `initGroups()` dan tiga registrasi dari registri langsung (`../shell/registry`, `../sidebar-left/registry`).

Asumsi yang dicatat: (1) `groups-store` memakai kontrak 2.3 persis; komponen tidak bergantung pada detail dalam stub/implementasi. (2) `Leaf` terminal memiliki `sessionId`; leaf pane file diabaikan untuk drag ke tab. (3) `dragState` tidak membawa `sessionId`, jadi `sessionId` dicari dari `layout` memakai `paneId`; bila layout mengubah leaf pada saat drag selesai, jalur ini aman karena `sessionId` disimpan di awal drag. (4) Bila `../shell` index memicu siklus impor saat memuat modul, impor `focusOrOpenSession`/`sessionTitle` langsung dari `../shell/helpers`.
