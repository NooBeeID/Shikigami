# Indeks tiket: Rail menu utama (sidebar kiri dua lapis)

Sumber: `docs/superpowers/specs/2026-11-22-08-main-rail-design.md`

## Ringkasan tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|-------|-------|------|------------|--------------|
| main-rail-ticket-01-main-rail | Rail menu utama dan panel kiri | 1 | - | 5 |
| main-rail-ticket-02-threads-rail | Threads sebagai item rail sendiri | 2 | main-rail-ticket-01-main-rail | 3 |

Fitur per tiket:

- **main-rail-ticket-01-main-rail**: rail-registry-store, rail-state-persist-go, rail-state-persist-ui, rail-ui, agents-rail-item
- **main-rail-ticket-02-threads-rail**: threads-panel-state, agents-sidebar-cleanup, threads-rail-item

## Urutan eksekusi per wave

Tiket dalam wave yang sama bisa dijalankan paralel. Wave berikutnya menunggu wave sebelumnya selesai dan di-merge.

- **Wave 1**: main-rail-ticket-01-main-rail
  - Internal, urutan:
    1. rail-registry-store dan rail-state-persist-go (foundation, bisa paralel)
    2. rail-state-persist-ui dan rail-ui (paralel, setelah foundation)
    3. agents-rail-item (wiring, terakhir)
- **Wave 2**: main-rail-ticket-02-threads-rail (menunggu main-rail-ticket-01-main-rail)
  - Internal, urutan:
    1. threads-panel-state dan agents-sidebar-cleanup (paralel)
    2. threads-rail-item (wiring, terakhir)

## Prasyarat sebelum implementasi

Perubahan belum di-commit berikut bukan bagian dari tiket ini dan tidak boleh disentuh oleh tiket manapun. Harus di-commit atau dipindah dulu agar worktree tiket mulai dari dasar yang bersih:

- `frontend/src/modules/layout/*`
- `frontend/src/lib/stores/layout.ts`
- `internal/provider/*`
- `frontend/src/modules/thread/participants/terminal.ts`

## Kontrak bersama

### 1) Registry

File: `frontend/src/modules/shell/registry.ts`

```ts
interface RailItemSpec {
  id: string;
  title: string;
  icon: Component;
  component: Component<any>;
  order?: number;
  visible?: Readable<boolean>;
}

registerRailItem(spec: RailItemSpec): void
railItems(): Readable<RailItemSpec[]>  // hanya visible, urut order lalu id
```

- Panel dirender dengan props `{}` dan mengambil state dari store.
- Hanya panel aktif yang di-mount.
- Tanpa `visible`, item selalu tampil.

### 2) State UI

- Store `leftSidebar` (`frontend/src/lib/stores/shell.ts`) berbentuk `{ visible, width, activeRail: string }`, default `activeRail = 'agents'`.
- `setActiveRail(id)` mengatur item aktif.
- Id tak dikenal, atau item yang tidak visible, di-resolve ke `'agents'`.
- Toggle kiri (`toggleLeft`) hanya menyembunyikan panel. Rail 48px tetap tampil.

### 3) Persist

- Go: `SidebarState.ActiveRail string` dengan tag `json:"activeRail,omitempty"` di `internal/workspace/types.go`.
- Frontend mengirim nilai lewat patch `leftSidebar` yang sudah ada.
- Restore memperlakukan nilai kosong atau non-string sebagai `'agents'`.

### 4) Urutan item rail

| Item | Order |
|------|-------|
| agents | 0 |
| lark | 10 |
| git-github | 20 |
| git-gitlab | 21 |
| threads | 30 |

Tiket Lark dan Git cukup memanggil `registerRailItem` di modul masing-masing tanpa mengubah shell.

### 5) Ikon dan teks

- Ikon berupa komponen Svelte SVG inline. Tidak ada dependensi baru.
- Setiap item memiliki `aria-label`.
- Teks UI berbahasa Indonesia: "Agen", "Thread", "Lark", "Git".

### 6) Subscriber event thread

- Subscriber event thread didaftarkan saat modul thread diimpor, yaitu `subscribeThreadEvents` di `modules/thread/index.ts`.
- Tidak didaftarkan saat komponen mount.
- Tidak boleh dipindah ke komponen panel.
