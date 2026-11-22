# Plan: Dukungan badge pada item rail

## Metadata

- Tiket: `docs/tickets/agent-dashboard-ticket-01-agent-dashboard.md` (agent-dashboard-ticket-01-agent-dashboard)
- Fitur: `rail-badge` (tahap B)
- ownedPaths: `frontend/src/modules/shell/Rail.svelte`, `frontend/src/modules/shell/RailBadge.svelte`
- Depends on foundation: ya. Memerlukan field `badge?: Readable<number | boolean>` pada `RailItemSpec` di `frontend/src/modules/shell/registry.ts` (dikerjakan `dashboard-foundation`, tahap A, sudah di-merge sebelum plan ini dijalankan). Jangan menambah field itu sendiri; bila field belum ada, laporkan.

## Tujuan

`Rail.svelte` merender `RailItemSpec.badge`: titik absolut di sudut tombol rail (gaya titik di `modules/notifications/Bell.svelte`) bila nilai `> 0` atau `true`, hilang saat `0`/`false`/`undefined`. Saat badge aktif, `aria-label` tombol menjadi `"<title> (ada sesi menunggu)"`. Item tanpa `badge` berperilaku identik dengan sebelumnya (label, `title`, `aria-current`, klik). Tanpa timer; subscription dibersihkan saat unmount; gaya di blok `<style>` komponen (`shell.css` tidak disentuh).

## Path yang dimiliki

HANYA path ini yang boleh disentuh:

- `frontend/src/modules/shell/Rail.svelte` (diubah)
- `frontend/src/modules/shell/RailBadge.svelte` (dibuat)

Jangan menyentuh `registry.ts`, `shell.css`, `App.svelte`, `register.ts`, `dashboard/*`, `status/*`, atau file wiring. Jangan menyentuh perubahan belum di-commit di `sidebar-right/FilesTab.svelte`.

## Kontrak

Dipakai (dari overview bagian 2.5, disediakan `dashboard-foundation`):

```ts
// shell/registry.ts
export interface RailItemSpec {
  id: string; title: string; icon: Component; component: Component<any>;
  order?: number; visible?: Readable<boolean>;
  badge?: Readable<number | boolean>; // titik tampil bila > 0 atau true
}
```

Disediakan (internal shell, bukan kontrak lintas fitur):

```ts
// shell/RailBadge.svelte, props
{ badge: Readable<number | boolean>; onchange?: (on: boolean) => void }
```

Pola yang dipilih: `RailBadge.svelte` subscribe ke store badge, merender titik, dan melaporkan status aktif ke `Rail.svelte` lewat callback `onchange` agar `aria-label` dapat dihitung di tombol. Nilai aktif: `v === true || (typeof v === 'number' && v > 0)`.

Fakta kode yang ada: `.rail-btn` di `shell.css` sudah `position: relative`, sehingga titik absolut di dalam tombol langsung menempel di sudut tombol. Svelte 5 (runes) dipakai di seluruh shell.

## Langkah

1. Buat `frontend/src/modules/shell/RailBadge.svelte`:

   ```svelte
   <script lang="ts">
     import type { Readable } from 'svelte/store';

     let { badge, onchange }: { badge: Readable<number | boolean>; onchange?: (on: boolean) => void } = $props();

     let on = $state(false);

     function isOn(v: number | boolean): boolean {
       return v === true || (typeof v === 'number' && v > 0);
     }

     // Subscribe hanya lewat store; tanpa timer. Cleanup: unsubscribe + laporkan nonaktif saat unmount.
     $effect(() => {
       const unsub = badge.subscribe((v) => {
         on = isOn(v);
         onchange?.(on);
       });
       return () => {
         unsub();
         onchange?.(false);
       };
     });
   </script>

   {#if on}
     <span class="rail-badge" aria-hidden="true"></span>
   {/if}

   <style>
     .rail-badge {
       position: absolute; top: 6px; right: 6px; width: 9px; height: 9px;
       border-radius: 50%; background: var(--danger, #e5484d);
       pointer-events: none;
     }
   </style>
   ```

   Titik `aria-hidden` (info disampaikan lewat `aria-label` tombol). `onchange` dibaca di dalam callback, bukan dependensi `$effect` reaktif yang memicu ulang subscribe; bila svelte-check atau runtime mengeluh, bungkus pembacaan `onchange` agar tidak membuat effect berjalan ulang berlebihan (efek hanya bergantung pada `badge`).

2. Ubah `frontend/src/modules/shell/Rail.svelte`:
   - Tambah impor `import RailBadge from './RailBadge.svelte';`.
   - Tambah state `let badged = $state<Record<string, boolean>>({});` dan fungsi `setBadged(id: string, on: boolean)` yang hanya menulis bila nilai berubah (`if (badged[id] === on) return; badged = { ...badged, [id]: on };`) untuk menghindari pembaruan berlebih.
   - Di tombol: `aria-label={badged[spec.id] ? `${spec.title} (ada sesi menunggu)` : spec.title}`. `title={spec.title}`, `class`, `aria-current`, `onclick` TIDAK diubah.
   - Setelah `<Icon />` di dalam tombol tambahkan:
     ```svelte
     {#if spec.badge}
       <RailBadge badge={spec.badge} onchange={(on) => setBadged(spec.id, on)} />
     {/if}
     ```
   - Biarkan sisa file (`railItems()`, `active`, `{#each}`, `onclick`) apa adanya. Jangan tambah blok `<style>` bila tidak perlu (gaya titik sudah di `RailBadge.svelte`).
   - Item tanpa `badge`: `badged[spec.id]` undefined sehingga `aria-label` = `spec.title`, tidak ada komponen anak; identik dengan sebelumnya.

3. Jalankan guard: `npm run build` di `frontend/`, lalu `npm run check` (svelte-check). Perbaiki error tipe/Svelte hanya di dua file milik fitur ini.

## Perubahan

- Dibuat: `frontend/src/modules/shell/RailBadge.svelte`
- Diubah: `frontend/src/modules/shell/Rail.svelte`

## Wiring

Fitur ini hanya mengekspos dukungan render: setiap `registerRailItem({... badge: <Readable<number | boolean>> })` otomatis menampilkan titik dan mengubah `aria-label`. JANGAN mendaftarkan item apa pun atau membuat badge sendiri. Langkah `dashboard-wiring` kelak memberi `badge: waitingCount` pada item `dashboard` di `dashboard/index.ts`.

## Selesai bila

- Item dengan `badge` bernilai `> 0`/`true` menampilkan titik di sudut tombol; hilang saat `0`/`false`.
- `aria-label` menjadi `"<title> (ada sesi menunggu)"` saat badge aktif; selain itu `title` saja.
- Item tanpa `badge` identik dengan perilaku sebelumnya (label, `aria-current`, klik).
- Tidak ada timer; subscription di-unsubscribe di cleanup `$effect` saat unmount.
- Hanya `Rail.svelte` dan `RailBadge.svelte` yang berubah; `shell.css` tidak disentuh.
- Guard: `npm run build` di `frontend/` sukses (dan `npm run check` bersih untuk path fitur ini). Tanpa unit test.
