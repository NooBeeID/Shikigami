# Plan: Halaman Settings Agent

## Metadata

- Tiket: `docs/tickets/global-agents-ticket-01-agents-registry.md` (fitur `agents-settings-page`)
- Overview: `docs/plans/global-agents-ticket-01-agents-registry/overview.md` (kontrak tipe di 2.1, IPC di 2.2, frontend di 2.5)
- Spec: `docs/superpowers/specs/2026-11-22-global-agents-design.md`
- ownedPaths: `frontend/src/modules/settings/agents/`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; hanya bergantung pada kontrak IPC di overview 2.2, bukan pada implementasi Go)

## Tujuan

Membangun modul frontend halaman Settings "Agent": kategori di kiri, peran kategori dan daftar agent di kanan, dialog form agent, dialog hapus kategori (pindahkan/hapus agent), dan konfirmasi hapus agent. Semua data lewat IPC `agents.*` dan event `agents.changed`. Teks UI bahasa Indonesia; identifier bahasa Inggris. Tidak mengubah `SettingsModal.svelte` dan tidak mengubah `frontend/src/modules/settings/index.ts` (itu wiring).

## Path yang dimiliki

HANYA boleh menyentuh `frontend/src/modules/settings/agents/` (semua berkas baru). Dilarang: `frontend/src/modules/settings/index.ts`, `SettingsModal.svelte`, `Modal.svelte`, `pages.ts`, `lib/*`, `connectors/*`, dan seluruh path Go/fitur lain. Boleh MENGIMPOR (baca saja) dari: `../../../lib/ipc` (`call`, `onEvent`), `../../../lib/stores/providers` (`providers`, `loadProviders`), `../../../lib/stores/types` (`Provider`), `../pages` (`registerSettingsPage`), `../Modal.svelte`. Jika butuh perubahan di luar path, catat di laporan, jangan edit.

## Kontrak

Dipakai (lihat overview 2.1 dan 2.2, tidak diulang di sini):
- IPC via `call<T>(method, params)` dari `lib/ipc.ts`: `agents.list`, `agents.categoryUpsert`, `agents.categoryDelete`, `agents.upsert`, `agents.delete`, `agents.setDefault`.
- Event: `onEvent('agents.changed', fn)` dari `lib/ipc.ts` mengembalikan fungsi unsubscribe; fn dipanggil `(topic, payload)`.
- Provider: `providers` (writable `Provider[]`) dan `loadProviders()` dari `lib/stores/providers.ts`; tipe `Provider { id, name, capabilities?: { mcp?: boolean ... }, available?: boolean, path?, error? }`.
- `registerSettingsPage({ id, title, order, component })` dari `../pages`.
- `Modal.svelte` props: `{ open: boolean; onclose: () => void; title?: string; children: Snippet }`. Catatan: dialog berukuran tetap 760x520 dengan `.body { overflow: hidden }`, jadi isi form harus muat atau punya wadah scroll sendiri (`overflow: auto; max-height: 100%`). Fokus awal, Escape, focus trap, `aria-modal` sudah ditangani `Modal`.

Disediakan (hanya internal modul; modul lain DILARANG mengimpor dari `modules/settings/agents`):
- `client.ts`: tipe `Category`, `Agent`, `AgentsSnapshot`, `CategoryUpsertInput`, `AgentUpsertInput`, `CategoryDeleteInput`; fungsi `listAgents`, `upsertCategory`, `deleteCategory`, `upsertAgent`, `deleteAgent`, `setDefaultAgent`.
- `store.ts`: `categories`, `agents`, `agentsLoading`, `agentsError`, `selectedCategoryId`, `loadAgents()`, `startAgentsSync()`.
- `index.ts`: efek samping `registerSettingsPage({ id: 'agents', title: 'Agent', order: 15, component: AgentsPage })`.

## Langkah

Semua berkas di `frontend/src/modules/settings/agents/`. Ikuti gaya `connectors/` (Svelte 5 runes, store Svelte klasik, CSS variabel `--bg-elev`, `--border`, `--fg`, `--fg-muted`, `--bg-hover` dengan fallback seperti di `ConnectorsPage.svelte`). Komentar dan teks UI bahasa Indonesia.

### 1. `client.ts`

```ts
import { call } from '../../../lib/ipc';

export interface Category { id: string; name: string; role: string; builtin: boolean }
export interface Agent {
  id: string; name: string; categoryId: string; providerId: string; model: string; isDefault: boolean;
}
export interface AgentsSnapshot { categories: Category[]; agents: Agent[] }
export interface CategoryUpsertInput { id?: string; name: string; role: string }
export interface AgentUpsertInput {
  id?: string; name: string; categoryId: string; providerId: string; model: string; isDefault: boolean;
}
export interface CategoryDeleteInput { id: string; agents?: 'move' | 'delete'; moveTo?: string }

export function listAgents(): Promise<AgentsSnapshot>
  // call<AgentsSnapshot | null>('agents.list'); normalisasi: categories ?? [], agents ?? []
export function upsertCategory(input: CategoryUpsertInput): Promise<Category>   // 'agents.categoryUpsert'
export function deleteCategory(input: CategoryDeleteInput): Promise<void>       // 'agents.categoryDelete'
export function upsertAgent(input: AgentUpsertInput): Promise<Agent>            // 'agents.upsert'
export function deleteAgent(id: string): Promise<void>                          // 'agents.delete', { id }
export function setDefaultAgent(id: string): Promise<void>                      // 'agents.setDefault', { id }
```

Fungsi bertipe `Promise<void>` cukup `.then(() => undefined)`. Jangan membungkus/menerjemahkan galat: pesan backend (bahasa Indonesia) diteruskan apa adanya.

### 2. `store.ts`

```ts
import { get, writable } from 'svelte/store';
import { onEvent } from '../../../lib/ipc';
import { listAgents, type Agent, type Category } from './client';

export const categories = writable<Category[]>([]);
export const agents = writable<Agent[]>([]);
export const agentsLoading = writable<boolean>(false);
export const agentsError = writable<string | null>(null);
export const selectedCategoryId = writable<string | null>(null);

let loadSeq = 0;
export async function loadAgents(): Promise<void>
```

- `loadAgents`: pola `connectors/store.ts`: `const seq = ++loadSeq`; set loading true dan error null; `await listAgents()`; hanya bila `seq === loadSeq` set `categories`, `agents`, lalu jaga seleksi: bila `get(selectedCategoryId)` null atau tidak ada di `categories` baru, set ke `categories[0]?.id ?? null`. Catch: hanya bila `seq === loadSeq` set `agentsError` (pesan `e.message`). Finally: hanya bila `seq === loadSeq` set loading false. Respons basi tidak boleh menimpa yang lebih baru.
- `startAgentsSync(): () => void`: berlangganan `onEvent('agents.changed', () => { void loadAgents(); })` dan mengembalikan fungsi unsubscribe dari `onEvent`. Dipanggil dari `AgentsPage` saat mount, unsubscribe saat destroy (tidak ada langganan saat startup, tidak bocor).
- Komponen TIDAK mengubah store langsung; semua mutasi lewat fungsi `client.ts`, lalu `await loadAgents()` setelah sukses (jaga-jaga bila event terlambat; `loadSeq` menangani duplikat). Satu-satunya set langsung yang diizinkan dari komponen: `selectedCategoryId.set(id)` untuk memilih kategori (itu keadaan UI, ekspos lewat fungsi `selectCategory(id: string)` di `store.ts` dan pakai itu).
- Ekspor juga `selectCategory(id: string): void`.

### 3. `AgentFormDialog.svelte` (form agent)

Props: `{ open: boolean; onclose: () => void; categoryId: string; agent: Agent | null }` (`agent` null = baru). Memakai `Modal` dengan title `Agent baru` atau `Ubah agent`.

- State lokal: `name`, `providerId`, `model`, `isDefault`, `saving`, `error`, `submitted`. Inisialisasi dalam `$effect` yang bereaksi pada `open`/`agent` (isi dari `agent` bila ada; bila baru: nama kosong, providerId = provider valid pertama bila ada, model kosong, isDefault false).
- Saat dibuka, panggil `void loadProviders()` bila `$providers.length === 0` (pembungkus `try/catch`; galat tampil di `error`).
- Daftar provider (derived): dari `$providers` ambil yang `available !== false` dan `id !== 'shell'`; bila `categoryId === 'orchestrator'` saring lagi `capabilities?.mcp === true`. Bila agent yang diedit punya `providerId` yang tidak ada di daftar tersaring, tambahkan opsi itu di awal dengan label `<nama atau id> (tidak valid untuk kategori ini)` dan tampilkan peringatan `role="alert"` di bawah select: "Provider saat ini tidak valid atau tidak tersedia. Pilih provider lain sebelum menyimpan." Simpan dinonaktifkan selama providerId terpilih tidak ada di daftar tersaring.
- Bila daftar tersaring kosong tampilkan teks "Tidak ada provider yang tersedia untuk kategori ini." dan nonaktifkan Simpan.
- Field (label terhubung lewat `for`/`id` unik, mis. prefix `agent-form-`): `Nama` (input teks, `autocomplete="off"`, petunjuk "Huruf kecil, angka, dan tanda hubung; spasi menjadi tanda hubung."), `Provider` (select), `Model` (input teks, `placeholder="default provider"`), checkbox `Jadikan default`.
- Validasi klien ringan: nama kosong setelah trim = galat "Nama wajib diisi." (tampil `role="alert"` dengan `aria-describedby`/`aria-invalid` pada input). Jangan menormalisasi slug di klien; backend yang melakukannya.
- Simpan (`<form onsubmit>` dengan `preventDefault`, tombol submit "Simpan" dan tombol "Batal"): panggil `upsertAgent({ id: agent?.id, name: name.trim(), categoryId, providerId, model: model.trim(), isDefault })`; saat `saving` tombol nonaktif dan teks "Menyimpan...". Sukses: `await loadAgents()` lalu `onclose()`. Gagal: tampilkan `e.message` apa adanya di `error` (`role="alert"`), dialog tetap terbuka.
- Layout: kolom flex dengan `gap`, wadah `overflow: auto; max-height: 100%; padding` agar muat di `Modal` 760x520.

### 4. `CategoryDeleteDialog.svelte`

Props: `{ open: boolean; onclose: () => void; category: Category; agentCount: number; otherCategories: Category[] }`. Title `Hapus kategori`.

- Bila `agentCount === 0`: teks konfirmasi "Hapus kategori <nama>?" dan tombol "Hapus" yang memanggil `deleteCategory({ id })`.
- Bila `agentCount > 0`: teks "Kategori <nama> masih punya N agent." dan dua radio (grup `role="radiogroup"`, label jelas): "Pindahkan agent ke kategori lain" (dengan `<select>` tujuan dari `otherCategories`, wajib dipilih; dinonaktifkan bila tidak ada kategori lain) dan "Hapus agent juga". Tombol "Hapus" memanggil `deleteCategory({ id, agents: 'move', moveTo })` atau `deleteCategory({ id, agents: 'delete' })`. Mode default: `move` bila `otherCategories.length > 0`, selain itu `delete`. Tombol nonaktif bila mode `move` tanpa tujuan.
- Sukses: `await loadAgents()` lalu `onclose()`. Gagal (mis. guard thread aktif, aturan orchestrator/MCP di tujuan): tampilkan `e.message` apa adanya (`role="alert"`), dialog tetap terbuka.

### 5. `ConfirmDialog.svelte`

Dialog konfirmasi generik kecil untuk hapus agent. Props: `{ open: boolean; onclose: () => void; title: string; message: string; confirmLabel: string; onconfirm: () => Promise<void> }`. Memakai `Modal`; tombol konfirmasi menjalankan `onconfirm` dengan status `busy`; galat ditampilkan `role="alert"` apa adanya dan dialog tetap terbuka; sukses menutup lewat `onclose()`. Tombol "Batal" menutup.

### 6. `CategoryNameDialog.svelte`

Dialog untuk "Kategori baru" dan "Ubah nama". Props: `{ open: boolean; onclose: () => void; category: Category | null }` (null = baru). Satu input `Nama` (wajib, validasi klien "Nama wajib diisi."). Simpan: baru = `upsertCategory({ name, role: '' })` lalu setelah `loadAgents()` pilih kategori hasil (`selectCategory(result.id)`); ubah nama = `upsertCategory({ id: category.id, name, role: category.role })`. Galat backend ditampilkan apa adanya. (Peran tidak diubah di dialog ini; peran diedit di panel kanan.)

### 7. `AgentsPage.svelte`

Tata letak dua kolom mengikuti gaya `ConnectorsPage` (variabel CSS yang sama; kolom kiri lebar tetap ~200px dengan border kanan, kanan fleksibel, `min-width: 0`).

Script:
- `onMount`: `void loadAgents(); void loadProviders().catch(() => {});` dan `const stop = startAgentsSync(); return stop;` (unsubscribe saat destroy). Data hanya dimuat saat halaman pertama dibuka.
- Derived: `selected` = kategori dengan id `$selectedCategoryId`; `categoryAgents` = `$agents` terfilter kategori itu, urut nama; `otherCategories` = kategori selain `selected`.
- State lokal: `roleText` (disinkronkan dari `selected.role` lewat `$effect` yang hanya menimpa bila kategori terpilih berubah atau tidak ada edit tertunda/`roleDirty`=false), `roleDirty`, `roleSaving`, `roleError`, `roleSaved` (status "Tersimpan"), `rowError` (galat aksi baris), dialog state: `agentDialog: { open, agent: Agent | null }`, `categoryNameDialog: { open, category: Category | null }`, `deleteCategoryOpen`, `deleteAgentTarget: Agent | null`.

Markup:
1. Kiri: `<nav aria-label="Kategori agent">` berisi `<div role="tablist" aria-orientation="vertical">` dengan satu `<button role="tab" aria-selected aria-controls="agents-panel" id=...>` per kategori (`tabindex` roving: terpilih 0, lainnya -1; ArrowUp/ArrowDown memindah seleksi dan fokus; klik/Enter/Space memilih lewat `selectCategory`). Di bawah: tombol "Kategori baru" membuka `CategoryNameDialog` mode baru.
2. Kanan: `<div role="tabpanel" id="agents-panel" aria-labelledby=...>`.
   - Status muat: `$agentsLoading && $categories.length === 0` => "Memuat..."; `$agentsError` => blok `role="alert"` berisi pesan + tombol "Coba lagi" (`loadAgents()`), bila data sudah ada tetap tampil di atas konten (pola ConnectorsPage).
   - Header: `<h2>` nama kategori; tombol "Ubah nama" (buka `CategoryNameDialog` mode ubah) dan "Hapus kategori" (disembunyikan untuk `builtin`; tampilkan teks kecil "Kategori bawaan tidak dapat dihapus" sebagai pengganti). Tombol hapus membuka `CategoryDeleteDialog`.
   - Peran: `<label for="agents-role">Peran</label>` + `<textarea id="agents-role" rows="4">` terikat `roleText`; simpan saat `blur` bila `roleDirty` dan lewat tombol "Simpan peran" (dinonaktifkan saat `!roleDirty || roleSaving`). Simpan = `upsertCategory({ id: selected.id, name: selected.name, role: roleText })`, sukses: `roleDirty=false`, `roleSaved=true`, `await loadAgents()`; status ditampilkan `role="status"` ("Menyimpan...", "Tersimpan") dan galat `role="alert"` apa adanya. Hindari simpan ganda bila blur dan klik tombol beruntun (cek `roleSaving`).
   - Agent: subjudul "Agent" + tombol "Agent baru" (buka `AgentFormDialog` dengan `agent=null`). Kosong: `<p class="muted">Belum ada agent di kategori ini.</p>`. Daftar `<ul>` gaya `li` ConnectorsPage; tiap baris: nama (tebal), lencana teks `Default` (`<span class="badge is-default">Default</span>`, boleh disertai ikon bintang `aria-hidden="true"`; status TIDAK hanya lewat warna) untuk `isDefault`, provider (cari nama di `$providers` berdasarkan `providerId`, fallback id), model (`agent.model` atau `default provider` bila kosong). Tombol baris dengan `aria-label` jelas: "Edit <nama>", "Hapus <nama>", dan "Jadikan default <nama>" (hanya untuk non-default; memanggil `setDefaultAgent(id)` lalu `await loadAgents()`; galat ke `rowError` `role="alert"`).
   - Hapus agent: buka `ConfirmDialog` (title "Hapus agent", message `Hapus agent <nama>?`, `onconfirm` = `deleteAgent(id)` lalu `await loadAgents()`). Galat backend (mis. guard) tampil apa adanya di dialog.
3. Tiga dialog dirender kondisional di akhir komponen dengan `{#if ...}` agar state form segar tiap dibuka. `AgentFormDialog` menerima `categoryId={selected.id}`.
4. Setelah kategori terpilih terhapus, `loadAgents()` otomatis menjatuhkan seleksi ke kategori pertama (logika di store).

Gaya: salin pola `button`, `li`, `.badge`, `.muted`, `.error` dari `ConnectorsPage.svelte`; `.is-default` memakai warna hijau/biru lembut dengan teks tetap terbaca. Tab terpilih diberi latar `--bg-hover` dan border kiri, plus `aria-selected` (bukan hanya warna). Fokus terlihat (`:focus-visible` outline) pada tab dan tombol.

### 8. `index.ts`

```ts
import { registerSettingsPage } from '../pages';
import AgentsPage from './AgentsPage.svelte';

// Halaman Agent; data baru dimuat saat halaman dibuka (onMount di AgentsPage), bukan saat startup.
registerSettingsPage({ id: 'agents', title: 'Agent', order: 15, component: AgentsPage });
```

Berkas ini TIDAK diimpor oleh fitur ini; penyambungan ke `settings/index.ts` adalah langkah wiring.

### 9. Verifikasi

Dari `frontend/`: `npm run build` dan `npm run check`. Perbaiki semua galat/peringatan tipe dan a11y svelte-check di berkas modul ini. Karena `index.ts` belum diimpor, pastikan modul tetap ikut diperiksa `svelte-check` (cek berkas ada di cakupan `tsconfig`); bila build tidak menyertakannya (tree-shaken), cukup `npm run check` yang menjadi bukti tipe valid, dan catat itu di laporan.

## Perubahan

Dibuat (semua di `frontend/src/modules/settings/agents/`):
- `client.ts`
- `store.ts`
- `AgentsPage.svelte`
- `AgentFormDialog.svelte`
- `CategoryDeleteDialog.svelte`
- `CategoryNameDialog.svelte`
- `ConfirmDialog.svelte`
- `index.ts`

Tidak ada berkas lain yang diubah.

## Wiring

Yang diekspos untuk disambungkan langkah wiring (JANGAN dilakukan di fitur ini):
- `frontend/src/modules/settings/agents/index.ts` yang saat diimpor mendaftarkan halaman `agents` (order 15). Wiring menambahkan tepat satu baris `import './agents';` di `frontend/src/modules/settings/index.ts` (overview bagian 5 butir 2). `SettingsModal.svelte` tidak disentuh.
- Fitur ini mengandalkan IPC `agents.*` dan event `agents.changed` dari `agents-store-ipc`; verifikasi end-to-end baru bermakna setelah backend dan wiring tersambung.

## Selesai bila

- `cd frontend && npm run build` dan `npm run check` lulus tanpa galat (guard; tanpa unit test).
- Hanya berkas di `frontend/src/modules/settings/agents/` yang dibuat/diubah (`git status` menunjukkan tidak ada path lain; `SettingsModal.svelte` dan `settings/index.ts` tidak berubah).
- Halaman memuat kategori dan agent dari `agents.list`; memilih kategori mengganti panel kanan (tablist keyboard-operable).
- Kategori baru/ubah nama/ubah peran/hapus dan agent baru/ubah/hapus/jadikan default memanggil method IPC yang benar dan UI ter-refresh (event `agents.changed` dan muat ulang setelah mutasi).
- Penanda default berupa teks "Default"; model kosong tampil "default provider" dan placeholder input sama.
- Dropdown provider tidak memuat `shell`, hanya yang tersedia; untuk kategori `orchestrator` hanya `capabilities.mcp === true`; provider tidak valid pada agent yang diedit tampil dengan peringatan dan Simpan dinonaktifkan.
- Dialog dapat dioperasikan keyboard (fokus masuk, Escape menutup, label terhubung); galat backend ditampilkan apa adanya.
- Respons `agents.list` basi tidak menimpa yang lebih baru (`loadSeq`); langganan `agents.changed` dilepas saat halaman dibongkar.
- Tidak ada impor dari `modules/settings/agents` oleh modul lain.
