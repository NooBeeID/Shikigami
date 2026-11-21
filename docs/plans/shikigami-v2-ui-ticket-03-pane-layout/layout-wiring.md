# Plan: Wiring: ganti TabContent dengan LayoutArea

## Metadata

- Tiket: `shikigami-v2-ui-ticket-03-pane-layout` (`docs/tickets/shikigami-v2-ui-ticket-03-pane-layout.md`)
- Fitur: `layout-wiring` (Wiring, dikerjakan setelah foundation dan `layout-renderer`, `pane-header-dnd`, `pane-kinds` tergabung)
- ownedPaths: `frontend/src/App.svelte`, `frontend/src/modules/tabs/`, `frontend/src/modules/shell/`
- Depends on foundation: ya (`layout-model`: `model.ts`, `registry.ts`, `lib/stores/layout.ts`; stub sudah harus tertimpa fitur paralel saat plan ini jalan)
- Rujukan kontrak: `overview.md` bagian 2 (tipe, store, host), bagian 5 (checklist wiring), bagian 6 (guard). Plan ini tidak mengulangnya.

## Tujuan

Area tengah aplikasi memakai `LayoutArea` (bukan `TabBar`/`TabContent`). Kode v1 yang memanggil `openTab`/`activateTab`/`closeTab` (`SessionList`, `NewSession`, `files/shared/shellBridge.ts`) tetap tidak diubah; sebuah shim di `modules/tabs/index.ts` menerjemahkan store `tabs` dan `activeTabId` ke `layout.openPane`/`setFocus`/`closePane` dan sebaliknya (fokus dan penutupan pane disinkronkan balik). `MentionComposer` membaca `activeTabId` untuk sesi aktif, jadi sinkron balik fokus pane ke `activeTabId` wajib benar.

## Path yang dimiliki

HANYA boleh disentuh:

- `frontend/src/App.svelte`
- `frontend/src/modules/tabs/` (BARU `index.ts`; `TabBar.svelte`, `TabContent.svelte`, `tab-title.ts` dihapus bila tidak ada impor tersisa)
- `frontend/src/modules/shell/` (hanya bila ada rujukan langsung ke tab; saat ini `SidebarArea.svelte`/`StatusBar.svelte` tidak perlu diubah)

Dilarang menyentuh: `lib/stores/tabs.ts`, `lib/stores/sessions.ts`, `lib/stores/layout.ts`, `lib/registry.ts`, `main.ts`, `package.json`, `Makefile`, `modules/workspace/`, `modules/files/`, `modules/layout/` (semua file, termasuk `index.ts`, stub, dan `panes/`), `modules/terminal/`, `modules/zoom/`, `modules/appmenu/`, `modules/status/`, `docs/perf/`, kode Go. Bila ada yang perlu diubah di sana, hentikan dan laporkan sebagai blocker.

## Kontrak

Dipakai (tidak diubah):

- `lib/stores` (`tabs`, `activeTabId`, `closeTab`, `sessions`, tipe `Tab`, `Session`) dan `lib/stores/layout.ts` (`layout` dengan `openPane(leaf, opts?)`, `setFocus(id)`, `closePane(id)`, `subscribe`; doc `LayoutDoc` memuat `focusId`; helper `leaves()` dari `modules/layout/model.ts`). Impor `layout` langsung dari `../../lib/stores/layout` (bukan `lib/stores/index.ts`).
- Tipe `Leaf`/`NewLeaf` dari `modules/layout/model.ts` (`workspaceId?` ada pada leaf).
- `LayoutArea.svelte` (prop `store?` bawaan `layout`; sudah memasang `installLayoutShortcuts` di `onMount`).
- Format tab v1: id `session:<sessionId>` (`refId` = sessionId) dan `file:<refId>` dengan `refId` = `JSON.stringify({path, projectId, workspaceId})` urutan kunci tetap (lihat `shellBridge.ts`).
- Zoom: `modules/zoom/index.ts` satu-satunya pemilik `menu:zoom`; ia memanggil `setTerminalFontScale` dari `modules/terminal` (sudah ada `font-scale.ts`, `registerTerminalInstance`, `currentTerminalFontSize`).
- Pane kind `terminal` dan `file` sudah terdaftar oleh `modules/layout/index.ts` (milik `pane-kinds`).

Disediakan: shim tanpa ekspor publik (`modules/tabs/index.ts` hanya efek samping saat dimuat oleh glob `main.ts`). Pemetaan internal:

```ts
function tabIdOfLeaf(l: Leaf): string | null;   // terminal -> `session:${sessionId}`; file -> `file:${JSON.stringify({path: filePath, projectId, workspaceId})}`
function leafOfTab(tab: Tab, doc: LayoutDoc): Leaf | null;  // cari di leaves(doc): terminal by sessionId === tab.refId; file by tabIdOfLeaf(l) === tab.id
function newLeafOfTab(tab: Tab): NewLeaf | null;            // session -> {kind:'terminal', sessionId, projectId, provider, title}; file -> decode refId -> {kind:'file', filePath, projectId, workspaceId, title}
```

## Langkah

1. **Prasyarat (hanya baca).** Pastikan fitur paralel sudah tergabung: ada `frontend/src/modules/layout/{LayoutArea.svelte,Splitter.svelte,shortcuts.ts,PaneHeader.svelte,PaneMenu.svelte,dnd.ts,DropOverlay.svelte,index.ts}` dan `panes/{hosts.ts,terminal.ts,file.ts}`. Periksa `modules/layout/index.ts` mengimpor `./panes/terminal` dan `./panes/file` (checklist 5.1). Bila ada yang hilang, hentikan dan laporkan blocker (jangan membuatnya).
2. **`App.svelte`.** Hapus impor `TabBar` dan `TabContent`; tambah `import LayoutArea from './modules/layout/LayoutArea.svelte';`. Ganti isi `<main class="main">` menjadi `<div class="content"><LayoutArea /></div>` (hapus `<TabBar />`). Biarkan CSS (`.main`, `.content`) apa adanya; perubahan sekecil mungkin.
3. **`modules/tabs/index.ts` (BARU).** Dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts')` di `main.ts` (jangan edit `main.ts`). Isi:
   - Impor `get` dari `svelte/store`; `tabs`, `activeTabId`, `closeTab`, `sessions` dari `../../lib/stores`; `layout` dari `../../lib/stores/layout`; `leaves` dan tipe `Leaf`, `NewLeaf`, `LayoutDoc` dari `../layout/model`; tipe `Tab`.
   - `let syncing = false;` dan `let known = new Set<string>()` (tabId yang pernah punya pane). Helper `run(fn)` yang set `syncing = true`, jalankan `fn` dalam try/finally lalu `syncing = false`.
   - Implementasikan `tabIdOfLeaf`, `leafOfTab`, `newLeafOfTab` (lihat Kontrak). `newLeafOfTab` untuk sesi: cari sesi di `get(sessions)` untuk `projectId` dan `provider` (`providerId`); judul = `tab.title`. Untuk file: `JSON.parse(tab.refId)` dibungkus try/catch (rusak: `null`, abaikan tab); judul = `tab.title`.
   - `ensurePane(tab)`: bila `leafOfTab(tab, get(layout))` ada, `layout.setFocus(leaf.id)`; selain itu `const nl = newLeafOfTab(tab); if (nl) layout.openPane(nl)` (`openPane` idempoten dan memfokuskan pane baru).
   - **Langganan `tabs`** (tab baru): untuk tiap tab di daftar yang `leafOfTab` null dan belum ada pane, `run(() => layout.openPane(...))` lalu, bila tab itu adalah `get(activeTabId)`, pastikan fokus pane itu. Jangan menutup pane di sini (penutupan tab tidak terjadi dari UI karena TabBar dihapus; bila `closeTab` dipanggil pihak lain, tab yang hilang dari daftar dan masih punya pane: `layout.closePane(leaf.id)` dalam `run`, lalu hapus dari `known`).
   - **Langganan `activeTabId`** (`id`): bila `syncing` atau `id` null, kembali. Cari tab di `get(tabs)`; tidak ada: kembali. `run(() => ensurePane(tab))`.
   - **Langganan `layout`** (`doc`, sinkron balik): hitung `present = new Set(leaves(doc).map(tabIdOfLeaf).filter(Boolean))`. Selalu perbarui `known` di akhir. Bila `!syncing`:
     - Fokus: `const f = doc.focusId && leaves(doc).find(l => l.id === doc.focusId)`; `tid = f ? tabIdOfLeaf(f) : null`; bila `tid` ada di `get(tabs)` dan `get(activeTabId) !== tid`, `run(() => activeTabId.set(tid))`.
     - Penutupan: untuk tiap `tid` di `known` yang tidak ada di `present` dan masih ada di `get(tabs)`: kumpulkan; setelah itu `run(() => { for (tid of closed) closeTab(tid); activeTabId.set(<tabId pane fokus saat ini, atau null>); })` supaya fokus tidak dibelokkan oleh tetangga pilihan `closeTab` (v1) dan klik ulang sesi di sidebar membuka pane lagi. Tidak memanggil `killSession`.
   - Pastikan `known` hanya berisi tab yang memang ada di `tabs` agar pane hasil `openPane` internal tidak dianggap tertutup. Tab hilang dari `tabs` tetapi pane masih ada ditangani di langganan `tabs`.
   - Tanpa polling, tanpa `setInterval`, tanpa `console.log`. Simpan fungsi unsubscribe tidak perlu (modul hidup selama aplikasi).
   - Bila ada pemanggil `openTab` di luar `SessionList`, `NewSession`, `shellBridge` (grep `openTab` di `frontend/src`) yang tidak bisa dijembatani shim, hentikan dan laporkan blocker.
4. **Pensiunkan berkas tab lama.** `grep -rn "TabBar\|TabContent\|tab-title\|tabLabel" frontend/src`; bila setelah langkah 2 tidak ada impor tersisa, hapus `modules/tabs/TabBar.svelte`, `TabContent.svelte`, `tab-title.ts`. Bila masih ada impor, biarkan.
5. **Pemeriksaan shell.** `grep -n "tab" frontend/src/modules/shell/*.svelte`; ubah hanya bila ada rujukan langsung ke `TabBar`/`TabContent` (kemungkinan tidak ada, tidak perlu diubah).
6. **Zoom (checklist 5.5).** `grep -rn "menu:zoom" frontend/src` harus menunjukkan SATU handler (`modules/zoom/index.ts`); jangan menambah. Periksa dengan baca saja bahwa `TerminalTab.svelte` memakai `currentTerminalFontSize`/`registerTerminalInstance` dan pane terminal tidak memberi prop `fontSize`; bila tidak, catat sebagai temuan (bukan blocker; jangan edit `modules/terminal/`).
7. **Stub tertimpa (checklist 5.6).** Baca `modules/layout/panes/hosts.ts`, `PaneHeader.svelte`, `dnd.ts`, `DropOverlay.svelte`; pastikan tidak ada baris `STUB(foundation)` atau teks placeholder (`grep -rn "STUB(foundation)" frontend/src`). Periksa tidak ada impor melingkar di `modules/layout` (model tidak mengimpor registry; store tidak mengimpor komponen). Temuan di luar ownedPaths dilaporkan, tidak diperbaiki.
8. **Guard.** `cd frontend && npm run check && npm run build`; `grep -rni "webgl" frontend/src/modules/terminal frontend/src/modules/layout` tidak menemukan import addon; dari root repo `make build` (mencakup `go build ./...`). Bila `npm run check` punya galat bawaan, bandingkan dan pastikan tidak ada galat baru di berkas ownedPaths.
9. **Verifikasi manual** di `build/bin/shikigami` (data dir sementara bila memungkinkan): beberapa terminal dan satu file terlihat bersamaan; klik sesi/file di sidebar membuka atau memfokuskan pane; `+ sesi` membuat pane baru; Cmd+D dan Cmd+Shift+D; seret header ke 4 tepi dan tengah (terminal tidak ter-reset); seret pembatas dan klik ganda 0.5; maksimalkan/pulihkan; Susun grid (6 jadi 3x2); ganti nama; Akhiri sesi vs Tutup pane (sesi tetap di sidebar, klik lagi membuka pane); fokus pane menyorot sesi di `SessionList`; zoom (Cmd+= dan lainnya) mengubah font semua pane terminal. Catat hasil di laporan akhir agent.

## Perubahan

- Diubah: `frontend/src/App.svelte`
- Dibuat: `frontend/src/modules/tabs/index.ts`
- Dihapus (bila tanpa impor tersisa): `frontend/src/modules/tabs/TabBar.svelte`, `TabContent.svelte`, `tab-title.ts`
- Tidak diubah: `modules/shell/*` (kecuali ada rujukan tab langsung)

## Wiring

Fitur ini ADALAH langkah wiring; tidak ada yang diekspos untuk disambungkan nanti. Satu-satunya serah terima: `perf-6-pane` berikutnya memakai aplikasi hasil `make build` dari fitur ini untuk mengukur 6 pane (Susun grid).

## Selesai bila

- `App.svelte` merender `<LayoutArea />` di area tengah; `TabBar`/`TabContent` tidak dirender (dan dihapus bila tidak ada impor).
- `modules/tabs/index.ts` ada dan menjembatani dua arah dengan guard anti-loop; `lib/stores/tabs.ts`, sidebar, dan `modules/files` tidak diedit.
- Klik sesi/file di sidebar, `+ sesi`, dan fokus pane menjaga `activeTabId` konsisten; menutup pane memanggil `closeTab` dan tidak membunuh sesi.
- Satu handler `menu:zoom`; tidak ada `STUB(foundation)` tersisa; tidak ada impor melingkar; tanpa WebGL.
- Guard: `cd frontend && npm run check && npm run build` dan `make build` sukses; tidak ada galat baru di path fitur ini. Tanpa unit test.
- Verifikasi manual langkah 9 dijalankan dan hasilnya dilaporkan.
