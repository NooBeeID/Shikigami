# Plan: Halaman Connector dan dialog Connect

## Metadata
- Tiket: `settings-framework-ticket-02-connector-framework` (`docs/tickets/settings-framework-ticket-02-connector-framework.md`)
- Fitur: `connector-ui`
- ownedPaths: `frontend/src/modules/settings/connectors/`, `frontend/src/modules/settings/ConnectorsPlaceholder.svelte`
- Depends on foundation: tidak (tidak ada `00-foundation.md`). Prasyarat luar: tiket 1 sudah ada di basis kode (`modules/settings/Modal.svelte`, `pages.ts`, `ConnectorsPlaceholder.svelte`). Kontrak lengkap ada di `overview.md` bagian 2.2 (IPC) dan 2.5 (frontend); plan ini tidak mengulanginya.

## Tujuan
Membuat halaman Pengaturan > Connector yang menampilkan daftar connector beserta badge status berbahasa Indonesia, tombol **Connect** untuk connector yang belum tersambung, dan dialog Connect (langkah bernomor, perintah dengan tombol **Salin**, **Cek ulang**, sukses lalu menutup sendiri). Seluruh data lewat IPC `connector.list|status|guide` memakai `call()`. Tanpa tombol "Buka di terminal". Panggilan IPC tidak boleh mengubah fokus atau ukuran pane terminal.

## Path yang dimiliki
HANYA path ini yang boleh disentuh:
- `frontend/src/modules/settings/connectors/` (folder baru: `client.ts`, `store.ts`, `ConnectorsPage.svelte`, `ConnectDialog.svelte`)
- `frontend/src/modules/settings/ConnectorsPlaceholder.svelte` (hanya mengganti isi)

Jangan sentuh: `Modal.svelte`, `SettingsModal.svelte`, `pages.ts`, `store.ts`, `index.ts` di `modules/settings/`, `McpPage.svelte`, `SettingsButton.svelte`, `App.svelte`, `main.ts`, `internal/appconfig`, kode Go apa pun, dan path fitur lain. JANGAN membuat `index.ts` di `connectors/` (folder ini bukan modul; `main.ts` memuat modul lewat glob `modules/*/index.ts`).

## Kontrak
Nama ekspor dan signature mengikuti `overview.md` 2.5 persis.

Dipakai (sudah ada):
- `call<T>(method: string, params?: unknown): Promise<T>` dari `frontend/src/lib/ipc.ts`. Dari folder `connectors/` impor sebagai `import { call } from '../../../lib/ipc';`. `call` menolak dengan `Error` (mis. "bukan di dalam Wails" saat di luar Wails).
- `Modal` dari `'../Modal.svelte'`: props Svelte 5 `open: boolean`, `onclose: () => void`, `title?: string`, `children: Snippet`. Saat `open` false tidak merender apa pun. Esc dan klik backdrop memanggil `onclose`; ada tombol x bawaan ber-`aria-label="Tutup"`; fokus dikelola Modal.
- Variabel CSS dengan fallback: `--bg-elev`, `--border`, `--fg`, `--fg-muted`, `--bg-hover` (mis. `var(--bg-elev, #222)`).

Disediakan (diekspos modul ini):
- `client.ts`: tipe `ConnectorState`, `ConnectorStatus`, `ConnectorGuide`, `ConnectorInfo`; fungsi `listConnectors`, `getConnectorStatus`, `getConnectorGuide`.
- `store.ts`: `connectors`, `connectorsLoading`, `connectorsError`, `loadConnectors`, `refreshConnector`.
- Komponen `ConnectorsPage.svelte` (tanpa props), `ConnectDialog.svelte` (props `{ connector: ConnectorInfo; open: boolean; onclose: () => void }`).
- `ConnectorsPlaceholder.svelte` tetap bernama sama, tanpa props, kini membungkus `ConnectorsPage`.

## Langkah
Gunakan Svelte 5 runes (`$state`, `$derived`, `$props`, `$effect`) dan TypeScript (`<script lang="ts">`). Semua teks UI Indonesia; identifier Inggris.

1. Buat `frontend/src/modules/settings/connectors/client.ts`:
   - Ekspor tipe persis seperti overview 2.5.
   - `export function listConnectors(): Promise<ConnectorInfo[]>` -> `call<ConnectorInfo[] | null>('connector.list').then((r) => r ?? [])` (jaga agar selalu array).
   - `export function getConnectorStatus(id: string): Promise<ConnectorStatus>` -> `call<ConnectorStatus>('connector.status', { id })`.
   - `export function getConnectorGuide(id: string): Promise<ConnectorGuide>` -> `call<ConnectorGuide>('connector.guide', { id })`, lalu normalisasi `steps ?? []` dan `commands ?? []`.

2. Buat `connectors/store.ts` (impor `writable` dari `svelte/store`, dan dari `./client`):
   - `export const connectors = writable<ConnectorInfo[]>([]);`
   - `export const connectorsLoading = writable<boolean>(false);`
   - `export const connectorsError = writable<string | null>(null);`
   - `export async function loadConnectors(): Promise<void>`: set loading true dan error null; `try { connectors.set(await listConnectors()); } catch (e) { connectorsError.set(e instanceof Error ? e.message : String(e)); } finally { connectorsLoading.set(false); }`. Jangan mengosongkan daftar lama saat gagal. Beri penjaga balapan sederhana (counter `loadSeq` modul-lokal; hanya hasil panggilan terbaru yang diterapkan).
   - `export async function refreshConnector(id: string): Promise<ConnectorStatus>`: `const status = await getConnectorStatus(id);` lalu `connectors.update((list) => list.map((c) => (c.id === id ? { ...c, status } : c)));` return `status`. HANYA entri `id` yang berubah (entri lain tetap referensi objek yang sama); tidak menyentuh `connectorsLoading`/`connectorsError` (galat dilempar ulang ke pemanggil, yaitu dialog).

3. Buat `connectors/ConnectorsPage.svelte` (tanpa props):
   - Impor `onMount` dari `svelte`, store dan `loadConnectors` dari `./store`, tipe dari `./client`, dan `ConnectDialog` dari `./ConnectDialog.svelte`.
   - `onMount(() => { void loadConnectors(); });`
   - State lokal `let dialogId = $state<string | null>(null);` dan `const dialogConnector = $derived($connectors.find((c) => c.id === dialogId) ?? null);` (dialog selalu memakai entri terkini dari store sehingga Cek ulang tercermin di badge).
   - Fungsi `badgeText(s: ConnectorStatus): string`: `cli_missing` -> "CLI belum terpasang"; `not_logged_in` -> "Belum login"; `connected` -> `s.user ? \`Tersambung sebagai ${s.user}\` : 'Tersambung'`. Fungsi `badgeClass(s)` mengembalikan `'is-missing' | 'is-pending' | 'is-ok'` untuk pewarnaan.
   - Fungsi `canConnect(s)`: `s.state === 'cli_missing' || s.state === 'not_logged_in'`.
   - Markup: judul `<h2>Connector</h2>`; cabang berurutan:
     - memuat dan daftar kosong (`$connectorsLoading && $connectors.length === 0`): teks "Memuat...".
     - galat (`$connectorsError`): pesan galat (`role="alert"`) + tombol "Coba lagi" yang memanggil `loadConnectors()`. Bila daftar lama masih ada, tampilkan galat di atas daftar, bukan menggantikannya.
     - kosong (`!$connectorsLoading && $connectors.length === 0`): "Belum ada connector".
     - daftar: `<ul>`, satu `<li>` per connector berisi nama, badge (`<span class="badge {badgeClass}">`), `status.message` bila ada (teks `--fg-muted`), dan tombol **Connect** (`type="button"`, `aria-label="Connect {name}"`) hanya bila `canConnect(status)`; klik: `dialogId = c.id`.
   - Di bawah daftar: `{#if dialogConnector}<ConnectDialog connector={dialogConnector} open={true} onclose={() => (dialogId = null)} />{/if}`.
   - Gaya `<style>` memakai variabel CSS yang ada dengan fallback; baris berbatas `1px solid var(--border, #333)`, latar `var(--bg-elev, #222)`, tombol hover `var(--bg-hover, #2c2c2c)`. Badge memakai warna aman di tema gelap (hijau/kuning/merah redup dengan teks tetap terbaca). Jangan memakai `autofocus` dan jangan memanggil `.focus()` pada elemen di luar komponen (agar tidak mencuri fokus dari pane terminal).

4. Buat `connectors/ConnectDialog.svelte`:
   - Props: `let { connector, open, onclose }: { connector: ConnectorInfo; open: boolean; onclose: () => void } = $props();`
   - Impor `Modal` dari `'../Modal.svelte'`, `getConnectorGuide` dan tipe dari `./client`, `refreshConnector` dari `./store`.
   - State: `guide: ConnectorGuide | null`, `guideLoading`, `guideError: string | null`, `checking: boolean`, `checkError: string | null`, `copiedIndex: number | null`, `copyError: string | null`, `success: boolean`.
   - Muat panduan: `$effect` yang, saat `open` true, reset state (`success=false`, `checkError=null`, `copyError=null`) lalu memanggil `getConnectorGuide(connector.id)` (tangani galat ke `guideError`; abaikan hasil bila `open`/`connector.id` sudah berubah). Hanya bergantung pada `open` dan `connector.id` (bukan pada status, agar Cek ulang tidak memuat ulang panduan).
   - `async function copy(cmd: string, i: number)`: `try { await navigator.clipboard.writeText(cmd); copiedIndex = i; setTimeout(() => { if (copiedIndex === i) copiedIndex = null; }, 1500); copyError = null; } catch { copyError = 'Gagal menyalin ke clipboard. Salin perintah secara manual.'; }`. Tombol menampilkan "Salin", lalu "Tersalin" selama ~1,5 detik.
   - `async function recheck()`: bila `checking` return; `checking = true; checkError = null; try { const s = await refreshConnector(connector.id); if (s.state === 'connected') { success = true; closeTimer = setTimeout(onclose, 1200); } } catch (e) { checkError = e instanceof Error ? e.message : String(e); } finally { checking = false; }`. Simpan `closeTimer` dan bersihkan di `onDestroy`/`$effect` cleanup serta saat dialog ditutup manual, agar `onclose` tidak terpanggil dua kali.
   - Markup di dalam `<Modal {open} {onclose} title={\`Hubungkan ${connector.name}\`}>`:
     - ringkasan status terkini: `connector.status` (teks sama dengan badge halaman: salin logika `badgeText` secara lokal, jangan mengimpor dari komponen lain) dan `connector.status.message` bila ada.
     - memuat panduan: "Memuat panduan..."; galat panduan: pesan galat.
     - `<ol>` untuk `guide.steps`.
     - bila `guide.commands.length > 0`: tiap perintah dalam `<pre><code>` dengan tombol **Salin** di sampingnya; pesan `copyError` (`role="alert"`) bila ada.
     - bila `success`: penanda sukses (`role="status"`), mis. "Berhasil tersambung" (dengan nama user bila ada), tombol Cek ulang disembunyikan atau dinonaktifkan.
     - bila belum sukses: tampilkan status/`message` terbaru (sudah lewat ringkasan status yang reaktif terhadap store) dan `checkError`.
     - baris tombol: **Cek ulang** (`disabled={checking || success}`, teks "Memeriksa..." saat `checking`) dan **Tutup** (`onclick={onclose}`). Tidak ada tombol "Buka di terminal".
   - Gaya: blok kode memakai latar `var(--bg-elev, #222)`, border `var(--border, #333)`, font monospace, `overflow-x: auto`, `white-space: pre-wrap` atau scroll horizontal agar tidak melebarkan modal.

5. Ganti seluruh isi `frontend/src/modules/settings/ConnectorsPlaceholder.svelte` (baca dulu isi lama dari tiket 1 sebelum menimpa):
   ```svelte
   <script lang="ts">
     import ConnectorsPage from './connectors/ConnectorsPage.svelte';
   </script>

   <ConnectorsPage />
   ```
   Nama berkas dan tanpa props dipertahankan (dipakai `registerSettingsPage` id `connectors` dari tiket 1).

6. Verifikasi build: `cd frontend && npm run build` harus lulus tanpa error. Bila tersedia, jalankan juga `npm run check` dan pastikan tidak ada error baru pada berkas milik fitur ini. Jangan menjalankan `make build` sebagai guard fitur ini (seluruh build dilakukan tahap Wiring).

Catatan kewaspadaan (jangan memperbaikinya dengan mengubah file milik tiket 1; cukup laporkan bila terjadi): `ConnectDialog` adalah `Modal` yang bersarang di dalam `Modal` milik `SettingsModal`. Bila ternyata Esc menutup kedua modal sekaligus (listener `keydown` capture pada `window` dari keduanya), tetap selesaikan fitur sesuai plan dan sebutkan perilaku itu di laporan akhir sebagai temuan untuk tahap Wiring.

## Perubahan
Dibuat:
- `frontend/src/modules/settings/connectors/client.ts`
- `frontend/src/modules/settings/connectors/store.ts`
- `frontend/src/modules/settings/connectors/ConnectorsPage.svelte`
- `frontend/src/modules/settings/connectors/ConnectDialog.svelte`

Diubah:
- `frontend/src/modules/settings/ConnectorsPlaceholder.svelte` (isi diganti menjadi pembungkus tipis `ConnectorsPage`)

## Wiring
Fitur ini tidak menyambungkan apa pun sendiri. Yang diekspos:
- `ConnectorsPlaceholder.svelte` (nama tetap) yang sudah dirujuk `registerSettingsPage({ id: 'connectors', ... })` di `modules/settings/index.ts` milik tiket 1, sehingga Pengaturan > Connector otomatis menampilkan `ConnectorsPage` tanpa perubahan file tiket 1.
- Method IPC yang dipanggil: `connector.list`, `connector.status` `{id}`, `connector.guide` `{id}` (disediakan fitur `connector-core`). Verifikasi end-to-end (connector `fake` pada ketiga state, Connect, Salin, Cek ulang, sukses lalu tutup, terminal tidak terganggu) dilakukan tahap Wiring pada `overview.md` bagian 5; bukan tugas fitur ini.

## Selesai bila
- Keempat berkas di `connectors/` ada dengan nama ekspor dan signature sesuai overview 2.5; tidak ada `index.ts` di `connectors/`.
- `client.ts` memanggil `connector.list`, `connector.status`, `connector.guide` lewat `call()`; `steps`/`commands`/daftar selalu array.
- `refreshConnector(id)` hanya memperbarui satu entri store dan mengembalikan status baru.
- Halaman menampilkan "Memuat...", galat + "Coba lagi", "Belum ada connector"; badge Indonesia untuk tiga state (termasuk "Tersambung sebagai <user>"); tombol Connect hanya untuk `cli_missing` dan `not_logged_in`.
- Dialog menampilkan langkah bernomor dan perintah dengan Salin (umpan balik "Tersalin" dan pesan saat clipboard gagal), Cek ulang (disabled selama memeriksa), penanda sukses lalu `onclose` setelah jeda singkat, tombol Tutup, tanpa "Buka di terminal".
- `ConnectorsPlaceholder.svelte` hanya merender `<ConnectorsPage />`.
- `git diff` hanya menunjukkan perubahan pada ownedPaths fitur ini.
- Guard: `cd frontend && npm run build` sukses. Tanpa unit test.
