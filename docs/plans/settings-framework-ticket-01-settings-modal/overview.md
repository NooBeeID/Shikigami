# Overview: settings-framework-ticket-01-settings-modal (Modal Pengaturan)

Sumber: tiket `docs/tickets/settings-framework-ticket-01-settings-modal.md` dan spec `docs/superpowers/specs/2026-11-22-settings-framework-design.md`. Dokumen ini adalah sumber kebenaran untuk kontrak bersama; plan per fitur hanya merinci langkah kerja. Seluruh pekerjaan di frontend (Svelte 5 runes, TypeScript); tidak ada perubahan Go, IPC, CLI, atau `internal/appconfig`.

## 1. Ringkasan epic dan urutan eksekusi

Tiket ini membangun wadah UI pengaturan: modal penuh yang dibuka dari entri "Semua pengaturan…" di dropdown gear, sidebar kiri (Connector, MCP) dan konten di kanan. Tiket 02 (`settings-framework-ticket-02-connector-framework`) kelak menimpa halaman `connectors` lewat `registerSettingsPage` dan memakai `Modal.svelte`, tanpa mengubah file tiket ini.

Urutan:
1. **Foundation** (`00-foundation.md`): stub berkompilasi untuk `store.ts`, `pages.ts`, `Modal.svelte`, dan barrel `index.ts` dengan signature final, agar tiga fitur dapat dibangun paralel dan `make build` lulus di tiap worktree.
2. **Fitur paralel**: `settings-foundation`, `settings-shell-pages`, `settings-gear-entry`. Masing-masing hanya menyentuh `ownedPaths`-nya.
3. **Wiring**: satu langkah akhir menyambungkan host modal ke `App.svelte` dan verifikasi manual (bagian 5).

Temuan kode yang memengaruhi desain: `registerPanel({ area: 'statusbar' })` hanya menyimpan spec di `lib/registry.ts`; saat ini TIDAK ada komponen shell yang me-render `panels('statusbar')` (panel `status-summary`, `mention-queue`, `openfolder-toast` terdaftar tetapi `App.svelte` menaruh `Toast`/`Palette` secara langsung). Jadi registrasi panel saja tidak menampilkan modal; host harus dipasang eksplisit di `App.svelte` pada tahap Wiring. Registrasi `registerPanel` tetap dilakukan sesuai tiket.

## 2. Kontrak bersama

Folder modul: `frontend/src/modules/settings/`. Modul dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts', { eager: true })` di `frontend/src/main.ts` (jangan diubah). Teks UI bahasa Indonesia; identifier bahasa Inggris. Gaya hanya memakai variabel CSS yang ada dengan fallback: `--bg-elev`, `--border`, `--fg`, `--fg-muted`, `--bg-hover` (mis. `var(--bg-elev, #222)`).

### 2.1 `pages.ts` (tanpa impor dari `store.ts`)
```ts
import type { Component } from 'svelte';
import type { Readable } from 'svelte/store';
export interface SettingsPageSpec { id: string; title: string; order?: number; component: Component<any>; }
export function registerSettingsPage(spec: SettingsPageSpec): void; // daftar/timpa menurut id
export const settingsPages: Readable<SettingsPageSpec[]>;            // urut (order ?? 0) naik, lalu id.localeCompare
```
Pola sama dengan `panelStore`/`panels()` di `lib/registry.ts`.

### 2.2 `store.ts` (boleh mengimpor `pages.ts`)
```ts
import type { Writable } from 'svelte/store';
export const settingsOpen: Writable<boolean>;   // awal false
export const activePage: Writable<string>;      // awal 'connectors'
export function openSettings(pageId?: string): void;
export function closeSettings(): void;
```
`openSettings`: set `settingsOpen=true`; `activePage = pageId` bila diberikan dan terdaftar di `get(settingsPages)`, selain itu id halaman pertama terurut (bila daftar kosong, biarkan nilai sekarang). `closeSettings`: `settingsOpen=false`. Pengembalian fokus ke pemicu dikelola `Modal.svelte` (ia menyimpan `document.activeElement` saat `open` berubah ke true); store tidak perlu menyimpan elemen.

### 2.3 `Modal.svelte`
Props (Svelte 5): `open: boolean`, `onclose: () => void`, `title?: string`, `children: Snippet`. Bila `open` false: tidak merender apa pun dan tidak ada listener global. `role="dialog"`, `aria-modal="true"`, `aria-label={title}`. Esc (listener `keydown` di `window`, capture, hanya saat terbuka) memanggil `onclose` + `stopPropagation`; klik backdrop (`pointerdown`, `e.target === e.currentTarget`) memanggil `onclose`. Backdrop `position: fixed; inset: 0; z-index: 2000`. Fokus: simpan elemen aktif saat buka, fokus ke dialog (`tabindex="-1"`) atau elemen fokusable pertama, Tab/Shift+Tab berputar dalam dialog, kembalikan fokus saat tutup/unmount bila elemen masih di DOM. Tombol tutup x dengan `aria-label="Tutup"`.

### 2.4 Komponen halaman
- `ConnectorsPlaceholder.svelte`: judul "Connector", teks "Daftar connector segera hadir". Tanpa props.
- `McpPage.svelte`: judul "MCP", teks "Pengelolaan server MCP segera hadir". Tanpa props.
- `SettingsModal.svelte`: tanpa props; memakai `Modal` dengan `open={$settingsOpen}`, `onclose={closeSettings}`, `title="Pengaturan"`; sidebar `role="tablist"` `aria-orientation="vertical"`, item `role="tab"` dengan roving tabindex; panah atas/bawah (wrap), Home/End; konten `role="tabpanel"` merender komponen halaman `id === $activePage`, fallback halaman pertama. Komponen ini juga menjadi komponen host panel (tidak ada berkas `SettingsModalHost.svelte` baru) karena `Modal` sudah tidak merender apa pun saat tertutup.

### 2.5 Barrel `index.ts`
Re-export: `openSettings`, `closeSettings`, `settingsOpen`, `activePage` (dari `./store`), `registerSettingsPage`, `settingsPages`, tipe `SettingsPageSpec` (dari `./pages`), `Modal` (default dari `./Modal.svelte`), dan `SettingsModal` (default dari `./SettingsModal.svelte`). Side effect: `registerSettingsPage` untuk `connectors` (title "Connector", order 10, `ConnectorsPlaceholder`) dan `mcp` (title "MCP", order 20, `McpPage`); `registerPanel({ id: 'settings-modal', area: 'statusbar', order: 95, component: SettingsModal })`.

### 2.6 Konvensi
- Impor lintas modul lewat barrel (`'../settings'`); topbar boleh mengimpor settings, settings TIDAK boleh mengimpor topbar (hindari siklus). `pages.ts` tidak mengimpor `store.ts`.
- Jangan menyimpan token/kredensial; tidak ada pemanggilan fungsi terminal dari modul ini; modal hanya overlay (tidak meng-unmount pane).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `settings-foundation` | Store (`settingsOpen`, `activePage`, `openSettings`, `closeSettings`), registri halaman, komponen `Modal.svelte` generik dengan Esc/backdrop/fokus. Menimpa stub foundation. | `frontend/src/modules/settings/store.ts`, `frontend/src/modules/settings/pages.ts`, `frontend/src/modules/settings/Modal.svelte` |
| `settings-shell-pages` | `SettingsModal.svelte` (sidebar + konten), `McpPage.svelte`, `ConnectorsPlaceholder.svelte`, dan `index.ts` (registrasi halaman bawaan, panel `settings-modal`, re-export). Menimpa stub barrel. | `frontend/src/modules/settings/SettingsModal.svelte`, `frontend/src/modules/settings/McpPage.svelte`, `frontend/src/modules/settings/ConnectorsPlaceholder.svelte`, `frontend/src/modules/settings/index.ts` |
| `settings-gear-entry` | Grup menu baru "Semua pengaturan…" di dropdown gear; fokus dikembalikan ke tombol gear. Perluasan terbatas: impor `openSettings` dari `'../settings'`, ref tombol gear (`bind:this`), array `groups`. | `frontend/src/modules/topbar/SettingsButton.svelte` |

Acceptance criteria per fitur ada di tiket (bagian Fitur) dan menjadi acuan plan masing-masing. Catatan: `frontend/src/modules/settings/connectors/` milik tiket 02; jangan dibuat.

## 4. Foundation

Ada (`hasFoundation=true`). Lihat `00-foundation.md`. Isi: stub dengan signature final untuk `pages.ts`, `store.ts`, `Modal.svelte`, `index.ts` (barrel saja, tanpa registrasi), masing-masing diberi komentar `STUB(foundation)` di baris pertama dan ditimpa penuh oleh fitur pemiliknya. Stub bersifat fungsional minimal (bukan melempar error) agar build dan runtime tidak rusak bila fitur lain selesai lebih dulu. Foundation tidak menyentuh `SettingsButton.svelte`, `App.svelte`, atau `main.ts`.

## 5. Wiring checklist

Dikerjakan setelah ketiga fitur selesai dan digabung. Satu-satunya berkas wiring baru: `frontend/src/App.svelte` (di luar ownedPaths fitur).
1. Pastikan `modules/settings/index.ts` (fitur shell-pages) mendaftarkan halaman `connectors` dan `mcp` serta `registerPanel({ id: 'settings-modal', area: 'statusbar', order: 95, component: SettingsModal })`, dan tidak ada sisa komentar `STUB(foundation)` di `store.ts`, `pages.ts`, `Modal.svelte`, `index.ts` (`grep -rn "STUB(foundation)" frontend/src/modules/settings` harus kosong).
2. Karena tidak ada komponen shell yang me-render `panels('statusbar')`, tambahkan di `frontend/src/App.svelte`: `import { SettingsModal } from './modules/settings';` dan render `<SettingsModal />` pada cabang `{:else}` setelah `<Palette />` (di luar `ShellFrame`, sehingga terminal tidak di-unmount; modal `position: fixed` dan tidak merender apa pun saat tertutup). Jangan mengubah bagian lain `App.svelte`. Bila kelak ada renderer `panels('statusbar')`, hapus mount langsung ini agar tidak ganda.
3. Pastikan `SettingsButton.svelte` memanggil `openSettings()` dari `'../settings'` lewat grup "Semua pengaturan…" dan menfokuskan tombol gear lebih dulu.
4. Pastikan `main.ts` tidak berubah (glob memuat `modules/settings/index.ts` otomatis) dan tidak ada impor siklus (`settings` tidak mengimpor `topbar`).
5. Verifikasi manual di aplikasi berjalan: gear -> "Semua pengaturan…" membuka modal di Connector; ganti halaman Connector/MCP dengan klik dan panah/Home/End; tutup dengan Esc, klik backdrop, dan tombol x; fokus kembali ke gear; sesi terminal yang berjalan tidak restart, tidak kehilangan output, dan menerima input setelah modal tertutup (saat tertutup tidak ada elemen modal di DOM).

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- `go build ./...` (tidak ada perubahan Go; memastikan tidak rusak).
- `make build` (termasuk `vite build` frontend dan build app).
- Bila tersedia: `cd frontend && npm run check` (svelte-check) tanpa error baru dibanding sebelum perubahan.

Selama fase paralel, minimal `cd frontend && npm run build` harus lulus di worktree tiap fitur (dimungkinkan oleh stub foundation).

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, `App.svelte`, `main.ts`, `lib/registry.ts`, atau berkas modul lain.
- Patuhi nama ekspor dan props di bagian 2 persis; jangan menambah ekspor/props yang mengubah kontrak. Jika perlu hal di luar kontrak, buat lokal di berkas milik sendiri.
- Stub foundation ditimpa utuh oleh fitur pemilik (hapus baris `STUB(foundation)`); fitur lain tidak mengedit stub tersebut.
- `settings-gear-entry` hanya mengubah impor, ref tombol gear, dan array `groups` di `SettingsButton.svelte`; markup, gaya, dan logika menu lain tidak diubah.
- Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
