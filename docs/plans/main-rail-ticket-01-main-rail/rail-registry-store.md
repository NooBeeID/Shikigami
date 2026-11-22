# Plan: Registry item rail dan state activeRail

## Metadata
- Tiket: `docs/tickets/main-rail-ticket-01-main-rail.md` (id `main-rail-ticket-01-main-rail`)
- Fitur: `rail-registry-store`
- Tahap: A (paralel dengan `rail-state-persist-go`)
- ownedPaths:
  - `frontend/src/modules/shell/registry.ts`
  - `frontend/src/lib/stores/shell.ts`
  - `frontend/src/modules/shell/index.ts`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; fitur ini sendiri bagian dari tahap A)

## Tujuan
Menyediakan lapisan data untuk rail menu utama tanpa UI: registry item rail (`RailItemSpec`, `registerRailItem`, `railItems()`) dan state `activeRail` pada `leftSidebar` beserta `setActiveRail` dan `resolveActiveRail`. Fitur tahap B dan wiring mengimpor simbol ini sesuai kontrak overview.

## Path yang dimiliki
HANYA tiga berkas ini yang boleh disentuh:
- `frontend/src/modules/shell/registry.ts`
- `frontend/src/lib/stores/shell.ts`
- `frontend/src/modules/shell/index.ts`

Jangan menyentuh berkas wiring (`App.svelte`, `register.ts`, `sidebar-left/*`), path fitur lain (`Rail.svelte`, `RailPanelHost.svelte`, `ShellFrame.svelte`, `shell.css`, `persist/*`, `internal/workspace/*`), maupun `layout/*`, `lib/stores/layout.ts`, `provider/*`, `thread/*`, `app/*`. Jangan mengubah `overview.md`. Tanpa unit test dan tanpa dependensi baru.

## Kontrak
Mengikuti overview bagian 2.1 dan 2.2 (jangan diubah). Yang disediakan fitur ini:

`registry.ts`:
```ts
export interface RailItemSpec {
  id: string; title: string; icon: Component; component: Component<any>;
  order?: number; visible?: Readable<boolean>;
}
export function registerRailItem(spec: RailItemSpec): void;
export function railItems(): Readable<RailItemSpec[]>; // hanya visible, urut order lalu id
```

`lib/stores/shell.ts`:
```ts
export interface SidebarState { visible: boolean; width: number; activeRail?: string }
export const DEFAULT_RAIL = 'agents';
export function setActiveRail(id: string): void;
export function resolveActiveRail(items: { id: string }[], id: string | undefined): string;
```

Dipakai: `byOrder`, `derived`, `writable` yang sudah ada. Tidak ada pesan IPC.

## Langkah

### 1. `frontend/src/lib/stores/shell.ts`
1. Ubah baris 3 menjadi:
   `export interface SidebarState { visible: boolean; width: number; activeRail?: string }`
   `RightSidebarState extends SidebarState` dibiarkan; `rightSidebar` tidak berubah.
2. Tambah `export const DEFAULT_RAIL = 'agents';` di dekat konstanta `LEFT_DEFAULT` (sebelum `leftSidebar`).
3. Ubah default `leftSidebar` menjadi `writable<SidebarState>({ visible: true, width: LEFT_DEFAULT, activeRail: DEFAULT_RAIL })`.
4. Tambah:
   ```ts
   export function setActiveRail(id: string): void {
     leftSidebar.update((s) => ({ ...s, activeRail: id }));
   }
   export function resolveActiveRail(items: { id: string }[], id: string | undefined): string {
     return id && items.some((i) => i.id === id) ? id : DEFAULT_RAIL;
   }
   ```
   `resolveActiveRail` mengasumsikan `items` sudah hanya yang visible (hasil `railItems()`); id kosong/undefined/tak dikenal/tak visible menghasilkan `'agents'`.
5. `toggleLeft` dan `setLeftWidth` tidak diubah (sudah memakai spread sehingga `activeRail` tidak hilang).

### 2. `frontend/src/modules/shell/registry.ts`
1. Tambah `RailItemSpec` di dekat interface lain (impor `Readable` sudah ada):
   ```ts
   export interface RailItemSpec {
     id: string; title: string; icon: Component; component: Component<any>;
     order?: number; visible?: Readable<boolean>;
   }
   ```
2. Tambah `const railStore = writable<Record<string, RailItemSpec>>({});`.
3. Tambah `registerRailItem(spec)`: `railStore.update((m) => ({ ...m, [spec.id]: spec }));` (id sama menimpa, idempoten).
4. Tambah `railItems(): Readable<RailItemSpec[]>`. Implementasi: `derived` dua tingkat.
   - Tingkat 1: `derived(railStore, (m) => Object.values(m).sort(byOrder))` menghasilkan daftar terurut.
   - Tingkat 2: `readable`/`derived` custom dari daftar itu. Gunakan pola `derived(sorted, ($items, set) => { ... }, [])` dengan callback bentuk `(values, set)`:
     - Untuk tiap item, ambil store visible-nya; item tanpa `visible` dianggap `true`.
     - Subscribe ke semua store `visible` (`item.visible.subscribe(...)`), simpan nilai terbaru di array lokal, dan setiap ada perubahan panggil `set($items.filter((_, i) => flags[i]))`.
     - Kembalikan fungsi cleanup yang memanggil semua `unsubscribe`.
     - Bila daftar kosong, langsung `set([])`.
   - Alternatif yang setara: `derived(sorted.map(...))` dengan `derived([...stores], (vals) => ...)` memakai `readable(true)` untuk item tanpa `visible`; karena jumlah store dinamis, pola callback `set` di atas lebih mudah. Pilih salah satu, asalkan perubahan satu `visible` memicu hasil baru dan hasil tetap terurut `order` lalu `id`.
   - Impor `readable`/`get` dari `svelte/store` hanya bila dipakai; jaga agar tidak ada impor tak terpakai.
5. Jangan ubah `byOrder` dan fungsi registry lain.

### 3. `frontend/src/modules/shell/index.ts`
1. Tambahkan `registerRailItem, railItems` ke baris ekspor registry yang ada.
2. Tambahkan `RailItemSpec` ke baris `export type { ... } from './registry'`.
3. JANGAN menambah ekspor `Rail` atau `RailPanelHost` (berkas `.svelte` belum ada pada tahap A; ekspor itu dilakukan `agents-rail-item`). Baris lain tidak diubah.

### 4. Verifikasi
Jalankan `npm run build` di `frontend/` (opsional `npm run check` bila ada; kegagalan yang sudah ada sebelum perubahan dilaporkan, bukan diperbaiki di luar ownedPaths).

## Perubahan
Diubah (tidak ada berkas baru):
- `frontend/src/modules/shell/registry.ts`: `RailItemSpec`, `railStore`, `registerRailItem`, `railItems`.
- `frontend/src/lib/stores/shell.ts`: `activeRail` pada `SidebarState`, `DEFAULT_RAIL`, default `leftSidebar`, `setActiveRail`, `resolveActiveRail`.
- `frontend/src/modules/shell/index.ts`: ekspor `registerRailItem`, `railItems`, tipe `RailItemSpec`.

## Wiring
Fitur ini hanya mengekspos simbol; JANGAN menyambungkannya sendiri. Dipakai nanti oleh:
- `rail-ui`: `Rail.svelte` dan `RailPanelHost.svelte` mengimpor `railItems` dari `./registry` serta `leftSidebar`, `setActiveRail`, `resolveActiveRail` dari `../../lib/stores/shell` (langsung, bukan lewat `./index`).
- `rail-state-persist-ui`: `restore.ts` menulis `activeRail` ke `leftSidebar`.
- `agents-rail-item`: `registerRailItem({ id: 'agents', ... })` dari `../shell/registry`, dan ekspor komponen `Rail`/`RailPanelHost` di `modules/shell/index.ts` bila diperlukan.

## Selesai bila
- `registerRailItem`, `railItems`, `RailItemSpec` terekspor dari `registry.ts` dan (registry + tipe) dari `modules/shell/index.ts`; komponen Rail tidak diekspor.
- `railItems()` hanya berisi item visible (tanpa `visible` selalu tampil), berubah saat store `visible` berubah, terurut `order` (default 0) lalu `id`.
- `leftSidebar` default `{ visible: true, width: LEFT_DEFAULT, activeRail: 'agents' }`; `setActiveRail` hanya mengubah `activeRail`.
- `resolveActiveRail` mengembalikan `'agents'` untuk id kosong, undefined, tak dikenal, atau tak ada di `items`.
- Hanya tiga ownedPaths yang berubah (cek `git status`/`git diff --stat` untuk path fitur ini).
- Guard: `npm run build` di `frontend/` sukses. Tanpa unit test.
