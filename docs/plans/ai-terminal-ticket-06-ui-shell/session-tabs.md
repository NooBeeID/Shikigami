# Plan: Banyak tab sesi di atas terminal tiket 02

## Metadata

- Tiket: `ai-terminal-ticket-06-ui-shell` (`docs/tickets/ai-terminal-ticket-06-ui-shell.md`)
- Fitur: `session-tabs`
- ownedPaths: `frontend/src/modules/tabs/`, `frontend/src/modules/terminal/index.ts`, `frontend/src/modules/terminal/QuickTerminal.svelte`
- Depends on foundation: ya (`00-foundation.md` sudah membuat `lib/stores/*` dan stub `modules/tabs/{TabBar,TabContent}.svelte`; plan ini MENGGANTI isi stub itu, bukan membuat file bernama lain)

## Tujuan

Mengganti stub `TabBar.svelte` dan `TabContent.svelte` dengan implementasi final: banyak tab sesi dengan judul dari provider dan project/cwd, penanda sesi `exited`, hanya tab aktif yang di-mount, `TerminalTab` (tiket 02) menerima `sessionId = refId`, menutup tab tidak membunuh sesi, tipe tab tak terdaftar menampilkan placeholder "tipe tab tidak tersedia". Lalu membersihkan panel sementara tiket 02: hapus pendaftaran `terminal-quick` dan file `QuickTerminal.svelte`. Tidak ada kode xterm/attach/subscribe/base64/resize baru di modul ini.

## Path yang dimiliki

HANYA path berikut yang boleh disentuh:

- `frontend/src/modules/tabs/` (ganti `TabBar.svelte`, `TabContent.svelte`; boleh tambah `tab-title.ts`)
- `frontend/src/modules/terminal/index.ts` (edit: hapus panel `terminal-quick`)
- `frontend/src/modules/terminal/QuickTerminal.svelte` (hapus)

Dilarang menyentuh: `lib/stores/`, `lib/registry.ts`, `lib/ipc.ts`, `main.ts`, `App.svelte`, `modules/shell/`, `modules/workspace/`, `styles/`, `app/*`, `package.json`, `modules/terminal/TerminalTab.svelte`, `modules/terminal/terminal-client.ts`, `internal/*`. Bila butuh ekspor store yang kurang, jangan edit store; pakai `call` dari `lib/ipc.ts` dan catat di laporan.

## Kontrak

Rujukan: overview bagian 2.2 (tipe), 2.3 (store), 2.4 (komponen lintas fitur), 2.5 (CSS variables), 2.7 (teks UI). Ringkas yang dipakai fitur ini:

Dipakai (impor store SELALU lewat barrel, path relatif dari `modules/tabs/`):

```ts
import { tabs, activeTabId, activateTab, closeTab, sessions, providers, activeWorkspace } from '../../lib/stores';
import type { Tab, Session, Provider, Workspace } from '../../lib/stores';
import { tabTypes } from '../../lib/registry';
```

- `tabs: Writable<Tab[]>`, `activeTabId: Writable<string | null>`, `activateTab(id)`, `closeTab(id)` (logika tetangga sudah di store; TIDAK memanggil `killSession`).
- `sessions: Writable<Session[]>` (`status: 'running' | 'exited'`, `providerId`, `projectId`, `cwd`), `providers: Writable<Provider[]>` (`id`, `name`), `activeWorkspace: Readable<Workspace | null>` (`projects[{id,path,name}]`).
- `tabTypes(): Readable<Record<string, TabTypeSpec>>` dari registry. Panggil SEKALI di level skrip komponen (`const types = tabTypes();` lalu `$types`); fungsi ini mengembalikan objek store baru tiap panggilan.
- Komponen tipe `session` = `TerminalTab` (prop `sessionId: string`), didaftarkan `modules/terminal/index.ts`.

Disediakan (diimpor `App.svelte` oleh `shell-layout`):

- `modules/tabs/TabBar.svelte`: default export, TANPA prop.
- `modules/tabs/TabContent.svelte`: default export, TANPA prop.
- `modules/tabs/tab-title.ts` (internal, pelengkap): `export function tabLabel(tab: Tab, sessions: Session[], providers: Provider[], workspace: Workspace | null): { title: string; subtitle: string; exited: boolean }`.

Teks UI yang harus persis: "tipe tab tidak tersedia".

## Langkah

### 1. Verifikasi prasyarat

- Pastikan ada `frontend/src/lib/stores/index.ts` yang mengekspor nama pada bagian Kontrak, dan `modules/tabs/{TabBar,TabContent}.svelte` stub dari Foundation. Bila tidak ada, hentikan dan laporkan blocker (jangan membuat store sendiri).
- Baca `modules/terminal/TerminalTab.svelte` (hanya baca) untuk memastikan prop bernama `sessionId`. Jangan diubah.

### 2. `modules/tabs/tab-title.ts`

Fungsi murni tanpa impor runtime (hanya `import type` dari `../../lib/stores`):

```ts
import type { Tab, Session, Provider, Workspace } from '../../lib/stores';

export function tabLabel(tab: Tab, sessions: Session[], providers: Provider[], workspace: Workspace | null) {
  title, subtitle, exited
}
```

Aturan untuk `tab.type === 'session'`:

- `s = sessions.find(x => x.id === tab.refId)`.
- Bila `s` ada: `title = providers.find(p => p.id === s.providerId)?.name ?? s.providerId`; `project = workspace?.projects.find(p => p.id === s.projectId)`; `subtitle = project?.name ?? basename(s.cwd)` (basename = segmen terakhir path, abaikan `/` di akhir; kosong -> `s.cwd`); `exited = s.status === 'exited'`.
- Bila `s` tidak ada (sesi belum termuat atau milik workspace lain): `title = tab.title || tab.refId`, `subtitle = ''`, `exited = false`.

Untuk tipe lain (mis. `file`): `title = tab.title || tab.refId`, `subtitle = ''`, `exited = false`.

### 3. `modules/tabs/TabBar.svelte` (ganti stub)

Svelte 5, tanpa prop. Skrip:

- Impor store/registry seperti Kontrak, plus `tabLabel`.
- `const items = $derived($tabs.map((t) => ({ tab: t, ...tabLabel(t, $sessions, $providers, $activeWorkspace) })))`.

Markup:

- Wadah `<div class="tabbar" role="tablist">`, tinggi `var(--tabbar-h, 34px)`, `overflow-x: auto`, `display: flex`, latar `var(--bg-elev, #f3f3f3)`, border bawah `1px solid var(--border, #ddd)`.
- Per item `{#each items as it (it.tab.id)}`: `<div class="tab" class:active={it.tab.id === $activeTabId} class:exited={it.exited} role="tab" aria-selected=...>` berisi tombol utama `<button class="label" onclick={() => activateTab(it.tab.id)} title={it.title + (it.subtitle ? ' · ' + it.subtitle : '')}>`:
  - `<span class="dot">` penanda exit: tampil hanya bila `it.exited` (abu-abu/`var(--danger)`), plus teks kecil `(exit)` atau `aria-label="sesi berakhir"`.
  - `<span class="title">{it.title}</span>` dan `{#if it.subtitle}<span class="sub">{it.subtitle}</span>{/if}`.
  - Tombol tutup terpisah `<button class="close" aria-label="Tutup tab" onclick={(e) => { e.stopPropagation(); closeTab(it.tab.id); }}>×</button>`. HANYA memanggil `closeTab`; tidak ada `killSession`, tidak ada `call('session.kill')`.
- Klik tengah (`onauxclick` dengan `e.button === 1`) juga memanggil `closeTab`.
- Tanpa tab: bar kosong (tidak ada error, tidak ada teks wajib).
- Gaya lokal di `<style>`; semua warna lewat `var(--x, fallback)` (`--fg`, `--fg-muted`, `--bg`, `--bg-hover`, `--border`, `--accent`, `--danger`); tab aktif punya garis bawah `var(--accent)`; tab exited teks `var(--fg-muted)`. Lebar tab maksimum ~200px dengan `text-overflow: ellipsis`. Tanpa `setInterval`/`setTimeout`.

### 4. `modules/tabs/TabContent.svelte` (ganti stub)

Svelte 5, tanpa prop. Skrip:

```ts
const types = tabTypes();
const activeTab = $derived($tabs.find((t) => t.id === $activeTabId) ?? null);
const spec = $derived(activeTab ? $types[activeTab.type] : undefined);
```

Markup:

- `<div class="tabcontent">` mengisi sisa tinggi (`flex: 1; min-height: 0; position: relative`; latar `var(--term-bg, var(--bg, #fff))` agar tidak berkedip).
- `{#if activeTab}` lalu `{#key activeTab.id}`:
  - `{#if spec}` `{@const Comp = spec.component}` `<Comp sessionId={activeTab.refId} refId={activeTab.refId} tabId={activeTab.id} />` (prop ekstra diabaikan `TerminalTab`; komponen tab `file` tiket 07 kelak memakai `refId`). Gunakan `{@const}` langsung di dalam blok `{#if}` (syarat Svelte 5), atau `<spec.component .../>` bila `svelte-check` lolos.
  - `{:else}` placeholder: `<div class="placeholder">tipe tab tidak tersedia</div>` (teks persis, di tengah, warna `var(--fg-muted)`), tanpa error/throw dan tanpa `console.error`.
- `{:else}` (tidak ada tab aktif): area kosong dengan teks netral opsional (mis. "Belum ada tab"); tidak wajib dan tidak boleh memuat "Hello World".
- Karena `{#key activeTab.id}`, hanya tab aktif yang punya instance `TerminalTab`; berpindah tab men-destroy yang lama (cleanup oleh `TerminalTab`, sesi tidak dibunuh) dan me-mount yang baru (scrollback dipulihkan oleh attach milik `TerminalTab`). Jangan menyimpan komponen tab tidak aktif tetap ter-mount (tanpa `display:none`, tanpa cache instance).
- Dilarang: impor `@xterm/*`, `terminal-client`, `base64`, `call`/`onEvent` dari `lib/ipc` di modul `tabs/`. Penanda exit hanya lewat store `sessions` (diubah event `session.exit` oleh `initStores`).

### 5. Pembersihan `modules/terminal/`

- Edit `frontend/src/modules/terminal/index.ts` menjadi persis:

```ts
import { registerTabType } from '../../lib/registry';
import TerminalTab from './TerminalTab.svelte';

registerTabType({ type: 'session', component: TerminalTab });
```

  (hapus impor `registerPanel`, impor `QuickTerminal`, dan baris `registerPanel({ id: 'terminal-quick', ... })`).
- Hapus `frontend/src/modules/terminal/QuickTerminal.svelte` (`rm`; pastikan tidak ada impor lain: `grep -rn "QuickTerminal\|terminal-quick" frontend/src` harus kosong setelahnya).
- JANGAN ubah `TerminalTab.svelte` dan `terminal-client.ts`.

### 6. Verifikasi

- `cd frontend && npm run build` hijau; `npm run check` tanpa error pada file milik fitur ini.
- `grep -rn "@xterm" frontend/src/modules/tabs` kosong.
- `grep -rn "setInterval" frontend/src/modules/tabs` kosong.
- `grep -rn "STUB" frontend/src/modules/tabs` kosong.
- `package.json` dan `package-lock.json` tidak berubah.

## Perubahan

- Ubah: `frontend/src/modules/tabs/TabBar.svelte` (ganti stub)
- Ubah: `frontend/src/modules/tabs/TabContent.svelte` (ganti stub)
- Baru: `frontend/src/modules/tabs/tab-title.ts`
- Ubah: `frontend/src/modules/terminal/index.ts` (hapus panel `terminal-quick`)
- Hapus: `frontend/src/modules/terminal/QuickTerminal.svelte`

## Wiring

Tidak ada yang disambungkan oleh fitur ini sendiri. Yang diekspos:

- `modules/tabs/TabBar.svelte` dan `modules/tabs/TabContent.svelte` (tanpa prop) untuk diimpor `App.svelte` (milik `shell-layout`) sebagai `./modules/tabs/TabBar.svelte` dan `./modules/tabs/TabContent.svelte`.
- Tidak ada `index.ts` di `modules/tabs/` (komponen internal, tanpa `registerPanel`).
- `modules/terminal/index.ts` tetap mendaftarkan tipe tab `session`; langkah wiring memverifikasi `grep -rn "terminal-quick\|QuickTerminal" frontend/src` kosong.
- `workspace-sidebar` membuka tab lewat `openTab({type:'session', refId})` dari store; `TabContent` yang merendernya.

## Selesai bila

- `TabBar` menampilkan banyak tab dengan judul provider + project/cwd, tab aktif ditandai, penanda exit muncul saat `Session.status === 'exited'`, tombol tutup hanya memanggil `closeTab`.
- `TabContent` me-mount satu komponen tab (aktif) lewat `{#key activeTab.id}`, meneruskan `sessionId={refId}`; tipe tak terdaftar menampilkan teks persis "tipe tab tidak tersedia" tanpa error.
- Tidak ada impor `@xterm/*`, `terminal-client`, atau `lib/ipc` di `modules/tabs/`; tidak ada `setInterval`.
- `terminal-quick` dan `QuickTerminal.svelte` tidak ada lagi; `TerminalTab.svelte` dan `terminal-client.ts` tidak berubah.
- Tidak ada file di luar ownedPaths yang berubah.
- Guard: `cd frontend && npm run build` sukses (disarankan `npm run check` tanpa error pada file fitur ini); tanpa unit test.
