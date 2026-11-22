# Overview: main-rail-ticket-01-main-rail

Tiket: `docs/tickets/main-rail-ticket-01-main-rail.md`
Spec: `docs/superpowers/specs/2026-11-22-08-main-rail-design.md`

## 1. Ringkasan epic dan urutan eksekusi

Sidebar kiri sekarang hanya `AgentSidebar`. Epic ini menambah rail ikon 48px di tepi kiri `ShellFrame` sebagai menu utama, plus panel kiri yang isinya mengikuti item rail aktif. Item didaftarkan lewat registry `registerRailItem` (pola `registerRightTab`). Pada tiket ini hanya item `agents` yang terdaftar; Threads tetap section di dalam `AgentSidebar` (dipindah di tiket 02).

Tidak ada fase Foundation terpisah (`hasFoundation=false`, tidak ada `00-foundation.md`). Tipe bersama sudah didefinisikan penuh di bagian 2, sehingga tiap agent bisa menulis kodenya terhadap kontrak itu. Urutan eksekusi mengikuti dependensi kompilasi:

1. Tahap A (paralel): `rail-registry-store`, `rail-state-persist-go`.
2. Tahap B (paralel, setelah A selesai dan di-merge): `rail-state-persist-ui`, `rail-ui`.
3. Tahap C (Wiring, terakhir): `agents-rail-item`.

Catatan: tahap A diperlakukan sebagai "foundation" oleh tiket, tetapi tetap fitur biasa dengan ownedPaths sendiri. Tahap B mengimpor simbol dari `lib/stores/shell.ts` dan `modules/shell/registry.ts`, jadi baru dikerjakan setelah A ada.

Prasyarat worktree: perubahan belum di-commit di `layout/*`, `lib/stores/layout.ts`, `provider/*`, `thread/participants/terminal.ts`, `app/*` tidak boleh disentuh oleh fitur manapun.

## 2. Kontrak bersama

### 2.1 Registry (`frontend/src/modules/shell/registry.ts`)

```ts
export interface RailItemSpec {
  id: string;
  title: string;                 // juga dipakai sebagai aria-label
  icon: Component;               // komponen Svelte SVG inline, tanpa props wajib
  component: Component<any>;     // panel; dirender dengan props {}
  order?: number;                // default 0
  visible?: Readable<boolean>;   // tanpa ini item selalu tampil
}
export function registerRailItem(spec: RailItemSpec): void;
export function railItems(): Readable<RailItemSpec[]>; // hanya visible, urut order lalu id
```

Aturan implementasi:
- Simpan di `writable<Record<string, RailItemSpec>>` seperti `rightTabStore`; mendaftar id yang sama menimpa (idempoten).
- `railItems()` menggabungkan store `visible` semua item: turunkan dari store record, lalu `derived` atas array store `visible` (item tanpa `visible` dianggap `true`). Perubahan salah satu `visible` harus memicu hasil baru. Sort memakai `byOrder` yang sudah ada di file itu (`order` naik lalu `id.localeCompare`).
- Ekspor ulang di `modules/shell/index.ts`: `registerRailItem`, `railItems`, tipe `RailItemSpec`, serta `Rail` dan `RailPanelHost` (komponen yang dibuat `rail-ui`; lihat 2.5 untuk siapa yang mengedit `index.ts`).

### 2.2 State UI (`frontend/src/lib/stores/shell.ts`)

```ts
export interface SidebarState { visible: boolean; width: number; activeRail?: string }
export const DEFAULT_RAIL = 'agents';
export const leftSidebar: Writable<SidebarState>; // default { visible: true, width: LEFT_DEFAULT, activeRail: 'agents' }
export function setActiveRail(id: string): void;  // hanya mengubah activeRail
export function resolveActiveRail(items: { id: string }[], id: string | undefined): string;
```

- `RightSidebarState extends SidebarState` tetap; `activeRail` opsional sehingga kode kanan tidak berubah.
- `resolveActiveRail`: kembalikan `id` bila ada di `items` (items sudah hanya yang visible); selain itu (kosong, undefined, tak dikenal, tak visible) kembalikan `'agents'`.
- `toggleLeft`, `setLeftWidth` tidak berubah dan tidak boleh menghapus `activeRail` (sudah memakai spread).

### 2.3 Persist

Go (`internal/workspace/types.go`):
```go
type SidebarState struct {
    Visible    bool   `json:"visible"`
    Width      int    `json:"width"`
    ActiveTab  string `json:"activeTab,omitempty"`  // hanya sidebar kanan
    ActiveRail string `json:"activeRail,omitempty"` // hanya sidebar kiri
}
func defaultLeftSidebar() SidebarState { return SidebarState{Visible: true, Width: 260, ActiveRail: "agents"} }
```

Hasil pemeriksaan kode yang ada (jangan diubah ulang):
- `internal/workspace/layout.go` `SetUI` menyalin `*p.LeftSidebar` utuh lalu hanya mengosongkan `ActiveTab`; `ActiveRail` ikut tersimpan. Tidak perlu edit.
- `internal/workspace/config.go` baris ~118 mengganti sidebar kiri dengan default hanya bila `Width == 0`; baris ~127 hanya mengosongkan `ActiveTab`. `ActiveRail` tidak dijatuhkan. Karena itu `store.go` biasanya tidak perlu disentuh; `layout.go` dan `config.go` bukan ownedPaths dan tidak boleh diedit. Bila ternyata ada kasus penjatuhan, laporkan, jangan edit file di luar ownedPaths.
- JSON lama tanpa `activeRail` terbaca sebagai string kosong; frontend memetakannya ke `'agents'`.

Frontend:
- `restore.ts` (`applyUi`): `activeRail: typeof l.activeRail === 'string' && l.activeRail ? l.activeRail : 'agents'`; `visible` dan `width` tidak diubah logikanya. Nilai tak dikenal dibiarkan; fallback dilakukan `resolveActiveRail` di `RailPanelHost`/`Rail`.
- `autosave.ts`: `uiPayload()` sudah menyalin `{ ...get(leftSidebar) }` dan `leftSidebar.subscribe(scheduleSave)` sudah ada, jadi `activeRail` otomatis ikut dan perubahannya memicu simpan (perbandingan lewat `JSON.stringify(uiPayload())`). Agent cukup memverifikasi hal ini dan hanya mengedit bila ada penyaringan field yang menjatuhkannya.

### 2.4 Komponen dan nama berkas

| Berkas | Isi |
|--------|-----|
| `frontend/src/modules/shell/Rail.svelte` | Kolom 48px, tanpa props. Memakai `railItems()`, `leftSidebar`, `setActiveRail`, `resolveActiveRail`. |
| `frontend/src/modules/shell/RailPanelHost.svelte` | Tanpa props. Me-mount hanya komponen item aktif, props `{}`. Efek reset ke `'agents'` bila item aktif tak lagi visible. |
| `frontend/src/modules/shell/ShellFrame.svelte` | Grid `rail | panel | center | right`; merender `<Rail />`. |
| `frontend/src/modules/sidebar-left/rail.ts` | `registerRailItem` untuk `agents`. |
| `frontend/src/modules/sidebar-left/AgentRailIcon.svelte` | Ikon agen (dibuat oleh `agents-rail-item`; lihat ownedPaths di bagian 3). |

Penting: `Rail.svelte` dan `RailPanelHost.svelte` impor dari `./registry` dan `../../lib/stores/shell` langsung (bukan lewat `./index`), supaya tidak ada siklus impor.

Kontrak `ShellFrame`: tetap menerima props `{ topbar, left, center, right }` (semua Snippet), tetapi `left` menjadi opsional; bila tidak diberikan, render `<RailPanelHost />` sebagai fallback. Struktur DOM:

```
<div class="body">           grid-template-columns: 48px var(--left-w) minmax(0,1fr) var(--right-w)
  <nav class="rail-slot">    <Rail />                    (selalu tampil, lebar 48px tetap)
  <aside class="side left">  {left snippet | RailPanelHost} + <Resizer side="left" />
  <main class="center">
  <aside class="side right">
```
`--left-w` tetap `$leftSidebar.visible ? width : 0` (hanya panel). `Resizer` kiri tetap di dalam `aside.side.left`. Border: rail punya `border-right: 1px solid var(--border)`; hindari border ganda saat panel 0px (`.side.left` dengan `--left-w: 0` tidak boleh menambah border yang terlihat; bila perlu atur `border-right` hanya saat panel visible).

Gaya rail di `shell.css` (kelas `.rail`, `.rail-btn`, `.rail-btn.active`), memakai token CSS yang ada (`--bg-elev`, `--border`, `--fg-muted`, dst.; cek `styles/theme.css`). Fokus keyboard harus punya indikator `:focus-visible`.

Aksesibilitas rail: `<button type="button" class="rail-btn" class:active aria-label={spec.title} title={spec.title} aria-current={active ? 'true' : undefined}>`, ikon dirender `<spec.icon />`. Ikon rail 20px, `currentColor`.

### 2.5 Item bawaan dan konvensi

- Item `agents`: `{ id: 'agents', title: 'Agen', icon: AgentRailIcon, component: AgentSidebar, order: 0 }`.
- Konvensi order untuk tiket lain: agents=0, lark=10, git-github=20, git-gitlab=21, threads=30.
- Teks UI bahasa Indonesia; ikon SVG inline, tanpa dependensi baru.
- `subscribeThreadEvents` (`modules/thread/index.ts`) tidak disentuh.

### 2.6 Pesan IPC

Tidak ada pesan IPC baru. Persist memakai pesan patch UI yang ada (`UIPatch.leftSidebar` berisi `SidebarState` utuh, kini dengan `activeRail`).

## 3. Daftar fitur

| Slug | Scope | ownedPaths |
|------|-------|------------|
| `rail-registry-store` | `RailItemSpec`, `registerRailItem`, `railItems()`; `activeRail`, `setActiveRail`, `resolveActiveRail` di store; tanpa UI. | `frontend/src/modules/shell/registry.ts`, `frontend/src/lib/stores/shell.ts`, `frontend/src/modules/shell/index.ts` |
| `rail-state-persist-go` | Field `ActiveRail` di `SidebarState`, default `"agents"` pada `defaultLeftSidebar()`; verifikasi merge UI tidak menjatuhkan field. | `internal/workspace/types.go`, `internal/workspace/store.go` (hanya bila merge menjatuhkan field) |
| `rail-state-persist-ui` | Restore `activeRail` dengan migrasi state lama; pastikan autosave membawa `activeRail`. | `frontend/src/modules/persist/restore.ts`, `frontend/src/modules/persist/autosave.ts` |
| `rail-ui` | `Rail.svelte`, `RailPanelHost.svelte`, grid empat kolom `ShellFrame`, CSS. | `frontend/src/modules/shell/Rail.svelte`, `frontend/src/modules/shell/RailPanelHost.svelte`, `frontend/src/modules/shell/ShellFrame.svelte`, `frontend/src/modules/shell/shell.css` |
| `agents-rail-item` | Wiring: daftarkan item `agents`, impor efek samping, ganti snippet `left` di `App.svelte` dengan `RailPanelHost`. | `frontend/src/modules/sidebar-left/rail.ts`, `frontend/src/modules/sidebar-left/index.ts`, `frontend/src/modules/shell/register.ts`, `frontend/src/App.svelte` (plus berkas baru `frontend/src/modules/sidebar-left/AgentRailIcon.svelte`, yaitu ikon milik fitur ini; bila ingin ikon inline di `rail.ts` itu juga boleh) |

Pembagian pengeditan `modules/shell/index.ts`: dimiliki `rail-registry-store`. Agent itu menambahkan ekspor `registerRailItem`, `railItems`, tipe `RailItemSpec`, dan juga `export { default as Rail } from './Rail.svelte'` serta `export { default as RailPanelHost } from './RailPanelHost.svelte'`. Berkas `.svelte` itu dibuat oleh `rail-ui` di tahap B; agar build tahap A tidak gagal, `rail-registry-store` hanya mengekspor registry dan tipe, dan **tidak** mengekspor komponen. Ekspor komponen `Rail` dan `RailPanelHost` dilakukan oleh `agents-rail-item` (Wiring) bila diperlukan; `App.svelte` boleh mengimpor `RailPanelHost` langsung dari `./modules/shell/RailPanelHost.svelte` sehingga ekspor itu tidak wajib. `agents-rail-item` mengedit `modules/shell/index.ts` hanya untuk ekspor komponen tersebut (pengecualian wiring yang diizinkan, berkas ini tidak diedit paralel lagi pada tahap C).

## 4. Foundation

Tidak ada. `hasFoundation=false`; tidak ada `00-foundation.md`. Tahap A di bagian 1 sudah cukup sebagai fondasi, dan kontraknya tertulis lengkap di bagian 2.

## 5. Wiring checklist (`agents-rail-item`, setelah semua fitur lain selesai)

1. Buat `frontend/src/modules/sidebar-left/rail.ts`:
   - impor `registerRailItem` dari `../shell/registry` (bukan `../shell/index`, hindari siklus), `AgentSidebar` dari `./AgentSidebar.svelte`, ikon `AgentRailIcon`;
   - panggil `registerRailItem({ id: 'agents', title: 'Agen', icon: AgentRailIcon, component: AgentSidebar, order: 0 })` di level modul.
2. Impor `./rail` sebagai efek samping dari `frontend/src/modules/sidebar-left/index.ts` (`import './rail';`) dan pastikan `modules/shell/register.ts` mengimpor `../sidebar-left` (tambahkan `import '../sidebar-left';` bila belum ada langsung maupun tidak langsung). Item harus terdaftar tepat sekali.
3. `frontend/src/App.svelte`: hapus impor `AgentSidebar`; ganti `{#snippet left()}<AgentSidebar />{/snippet}` dengan `{#snippet left()}<RailPanelHost />{/snippet}` dan impor `RailPanelHost` dari `./modules/shell/RailPanelHost.svelte` (atau dari `./modules/shell` bila ekspornya ditambahkan).
4. Tidak ada registrasi Go/binding Wails baru dan tidak ada rute.
5. Verifikasi akhir: build guard (bagian 6) lulus; secara manual (bila memungkinkan): rail terlihat dengan "Agen" aktif, Threads masih di panel Agen, toggle kiri hanya menyembunyikan panel, restart mempertahankan item aktif.

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` (dari root repo)
- `npm run build` di `frontend/` (opsional tambahan: `npm run check` di `frontend/` bila tersedia; kegagalan yang sudah ada sebelum perubahan dilaporkan, bukan diperbaiki di luar ownedPaths)

Fitur Go cukup `go build ./...`; fitur frontend cukup build frontend. Wiring menjalankan keduanya.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, file wiring, maupun `layout/*`, `lib/stores/layout.ts`, `provider/*`, `thread/*`, `app/*`, dan `panel kanan`.
- File wiring (`App.svelte`, `register.ts`, `sidebar-left/index.ts`, `sidebar-left/rail.ts`) hanya disentuh oleh `agents-rail-item` pada tahap C.
- Fitur tahap B boleh mengimpor simbol dari kontrak bagian 2 dan menganggapnya sudah ada karena tahap A sudah di-merge.
- Tanpa unit test dan tanpa dependensi baru. Jangan commit perubahan di luar fitur sendiri.
- Bila kontrak di overview ini tampak keliru, ikuti overview dan catat ketidaksesuaian di laporan; jangan mengubah path fitur lain.
