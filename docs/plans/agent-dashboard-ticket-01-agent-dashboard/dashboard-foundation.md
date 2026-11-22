# Plan: Kontrak tipe dan kerangka modul dashboard

## Metadata

- Tiket: `agent-dashboard-ticket-01-agent-dashboard` (`docs/tickets/agent-dashboard-ticket-01-agent-dashboard.md`)
- Fitur: `dashboard-foundation`
- ownedPaths:
  - `frontend/src/modules/dashboard/types.ts`
  - `frontend/src/modules/dashboard/store.ts`
  - `frontend/src/modules/shell/registry.ts`
- Depends on foundation: tidak (fitur ini adalah kerja bersama tahap A; tidak ada fase Foundation terpisah). Berjalan paralel dengan `status-list-fields`.

## Tujuan

Menyediakan kontrak frontend bersama agar fitur tahap B (`dashboard-store`, `dashboard-panel-ui`, `rail-badge`) dan wiring dapat dibangun paralel tanpa saling menunggu: tipe dan konstanta dashboard, stub store dengan `groupRows` yang sudah fungsional, dan field opsional `badge` pada `RailItemSpec`. Tidak ada perubahan perilaku aplikasi; build harus tetap lulus.

## Path yang dimiliki

HANYA tiga path ini yang boleh disentuh:

- `frontend/src/modules/dashboard/types.ts` (baru)
- `frontend/src/modules/dashboard/store.ts` (baru)
- `frontend/src/modules/shell/registry.ts` (ubah)

Jangan membuat `frontend/src/modules/dashboard/index.ts` (itu file wiring milik `dashboard-wiring`). Jangan menyentuh `Rail.svelte`, `modules/status/store.ts`, komponen Svelte dashboard, `internal/status/*`, `App.svelte`, `shell/register.ts`, `shell.css`, maupun perubahan belum di-commit di `sidebar-right/FilesTab.svelte`. Folder `frontend/src/modules/dashboard/` belum ada; dibuat otomatis oleh `types.ts`.

## Kontrak

Semua bentuk persis mengikuti `overview.md` bagian 2.2, 2.3, dan 2.5. Nama, ekspor, dan tipe tidak boleh diubah; bila tampak keliru, laporkan, jangan ubah.

Disediakan fitur ini:

- `types.ts`: `AgentStatus`, `AgentRow`, `STATUS_ORDER`, `STATUS_LABEL` (overview 2.2).
- `store.ts` (stub): `agentRows: Readable<AgentRow[]>` = `readable<AgentRow[]>([])`, `waitingCount: Readable<number>` = `readable(0)`, `groupRows(rows: AgentRow[]): Record<AgentStatus, AgentRow[]>` fungsional (overview 2.3). `dashboard-store` kelak menggantikan isi file ini; ekspor dan tipe harus tetap sama.
- `registry.ts`: `RailItemSpec.badge?: Readable<number | boolean>` (overview 2.5).

Dipakai: hanya `Readable`/`readable` dari `svelte/store`. Tidak ada impor dari modul lain.

## Langkah

1. Buat `frontend/src/modules/dashboard/types.ts` dengan isi persis:

   ```ts
   export type AgentStatus = 'waiting' | 'running' | 'idle' | 'done';

   export interface AgentRow {
     sessionId: string;
     title: string;
     providerId: string;
     projectName: string;
     status: AgentStatus;
     since: number; // ms epoch
     reason?: string;
     lastActivity?: string;
   }

   export const STATUS_ORDER: AgentStatus[] = ['waiting', 'running', 'idle', 'done'];

   export const STATUS_LABEL: Record<AgentStatus, string> = {
     waiting: 'Menunggu',
     running: 'Berjalan',
     idle: 'Idle',
     done: 'Selesai',
   };
   ```

2. Buat `frontend/src/modules/dashboard/store.ts`:

   ```ts
   import { readable, type Readable } from 'svelte/store';
   import { STATUS_ORDER, type AgentRow, type AgentStatus } from './types';

   // Stub foundation: diganti implementasi derived oleh fitur dashboard-store.
   // Ekspor dan tipe tidak boleh berubah.
   export const agentRows: Readable<AgentRow[]> = readable<AgentRow[]>([]);
   export const waitingCount: Readable<number> = readable(0);

   // Fungsional: keempat kunci selalu ada, urutan input dipertahankan.
   export function groupRows(rows: AgentRow[]): Record<AgentStatus, AgentRow[]> {
     const groups = {} as Record<AgentStatus, AgentRow[]>;
     for (const s of STATUS_ORDER) groups[s] = [];
     for (const r of rows) groups[r.status].push(r);
     return groups;
   }
   ```

   Jangan menambah `setInterval`, subscription, atau impor lain.

3. Ubah `frontend/src/modules/shell/registry.ts`: pada `interface RailItemSpec`, tambahkan satu baris setelah `visible?: Readable<boolean>;`:

   ```ts
   badge?: Readable<number | boolean>; // titik tampil bila > 0 atau true
   ```

   `Readable` sudah diimpor di file itu. Jangan mengubah `registerRailItem`, `railItems()`, atau fungsi lain; `railItems()` hanya membaca `visible` dan tetap meneruskan seluruh objek spec apa adanya.

4. Jalankan guard (lihat "Selesai bila") dan perbaiki kesalahan tipe hanya di tiga path di atas.

## Perubahan

- Baru: `frontend/src/modules/dashboard/types.ts`
- Baru: `frontend/src/modules/dashboard/store.ts`
- Ubah: `frontend/src/modules/shell/registry.ts` (hanya penambahan satu field opsional pada `RailItemSpec`)

## Wiring

Fitur ini tidak menyambungkan apa pun sendiri. Yang diekspos:

- Dari `dashboard/types.ts` dan `dashboard/store.ts`: dikonsumsi `dashboard-panel-ui` (impor `agentRows`/`groupRows` dari `./store`, tipe dari `./types`) dan, lewat penggantian isi `store.ts`, oleh `dashboard-store`.
- `waitingCount` dari `dashboard/store.ts`: dipakai langkah wiring sebagai `badge: waitingCount` pada `registerRailItem` di `dashboard/index.ts` (overview bagian 5).
- `RailItemSpec.badge`: dibaca oleh `rail-badge` di `Rail.svelte`.

Pemanggilan `registerRailItem` untuk dashboard dan pembuatan `dashboard/index.ts` BUKAN bagian fitur ini.

## Selesai bila

- `types.ts` mengekspor `AgentRow`, `AgentStatus`, `STATUS_ORDER`, `STATUS_LABEL` sesuai kontrak.
- `store.ts` mengekspor `agentRows`, `waitingCount`, `groupRows` dengan tipe kontrak; `groupRows` selalu mengembalikan keempat kunci dan mempertahankan urutan input.
- `RailItemSpec.badge` opsional ada; `railItems()` dan item rail lain tidak berubah perilaku.
- Hanya tiga file di ownedPaths yang berubah (`git status` tidak menunjukkan file lain dari fitur ini).
- Guard: dari `frontend/`, `npm run build` sukses; jalankan juga `npm run check` dan pastikan tidak ada error baru pada tiga path ini. Tanpa unit test.
