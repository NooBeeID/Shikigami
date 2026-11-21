# Plan: Zoom UI dan font terminal

## Metadata

- Tiket: `shikigami-v2-ui-ticket-02-native-menu` (`docs/tickets/shikigami-v2-ui-ticket-02-native-menu.md`)
- Fitur: `zoom`
- ownedPaths: `frontend/src/modules/zoom/`
- Depends on foundation: ya (`menu-bridge-contract`, modul `frontend/src/modules/appmenu/`, sudah tergabung sebelum plan ini dikerjakan)
- Kontrak lengkap: `docs/plans/shikigami-v2-ui-ticket-02-native-menu/overview.md` (bagian 2.1, 2.3, 2.4, 3 "zoom"). Dokumen ini tidak mengulangnya.

## Tujuan

Modul frontend `zoom` mendengarkan event menu `menu:zoom`, mengubah persen zoom (50..300, langkah 10, reset 100), menerapkannya ke seluruh UI dan font terminal, menyimpannya ke `config.json` (debounce ~300 ms) lewat `setAppConfig`, dan memulihkannya saat dimuat lewat `getAppConfig`. Modul tidak punya UI. Tanpa polling dan tanpa `setInterval`. Tanpa dependency baru.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `frontend/src/modules/zoom/` (file baru: `index.ts`, `zoom.css`)

Dilarang menyentuh: `frontend/src/modules/terminal/*` (wiring), `frontend/src/modules/appmenu/*` (foundation), `modules/openfolder/`, `app/*`, `internal/*`, `lib/ipc.ts`, `lib/registry.ts`, `main.ts`, `package.json`, `styles/base.css`, `index.html`. Jangan commit/merge sendiri.

## Kontrak

Dipakai (dari `../appmenu`, foundation; arah impor hanya zoom menuju appmenu):

```ts
import { onMenuEvent, getAppConfig, setAppConfig } from '../appmenu';
// onMenuEvent('menu:zoom', (p: { action: 'in' | 'out' | 'reset' }) => void): () => void
// getAppConfig(): Promise<AppConfig>   // luar Wails: {zoom: 100}
// setAppConfig(patch: Partial<AppConfig>): Promise<AppConfig>
```

Dipakai secara toleran (belum ada sampai Wiring): `setTerminalFontScale(percent: number): void` dari `../terminal` (overview 2.4). Hanya lewat impor dinamis dengan cast longgar; JANGAN impor statis dan JANGAN menyunting `modules/terminal/`.

Disediakan (ekspor `frontend/src/modules/zoom/index.ts`):

```ts
export const zoomPercent: Readable<number>;   // store baca-saja (svelte/store), nilai 50..300, awal 100
export function getZoomPercent(): number;     // nilai saat ini
```

Fakta kode yang sudah ada (diverifikasi): `index.html` punya `<div id="app">`; `styles/base.css` memberi `html, body, #app { height: 100%; margin: 0 }` dan `body { overflow: hidden }`; kontainer terminal di `TerminalTab.svelte` adalah `<div class="term-host">` dengan gaya scoped `width: 100%; height: 100%`. `main.ts` memuat `modules/*/index.ts` lewat glob (tidak perlu disunting).

## Langkah

### 1. `frontend/src/modules/zoom/zoom.css`

Gaya global (bukan scoped), diimpor dari `index.ts`:

```css
:root { --ui-scale: 1; }

/* Skala seluruh UI. Karena zoom ikut menskalakan ukuran, bagi tinggi/lebar
   agar #app tetap persis mengisi jendela. */
#app {
  zoom: var(--ui-scale);
  width: calc(100% / var(--ui-scale));
  height: calc(100% / var(--ui-scale));
}

/* Batalkan skala CSS pada kontainer terminal: xterm menghitung sel pada skala 1,
   font efektif = 13 x skala lewat setTerminalFontScale. Lebar/tinggi dikalikan
   skala agar setelah zoom 1/skala ukuran visualnya tetap mengisi induk. */
#app .term-host {
  zoom: calc(1 / var(--ui-scale));
  width: calc(100% * var(--ui-scale));
  height: calc(100% * var(--ui-scale));
}
```

Catatan: selektor `#app .term-host` (spesifisitas id) mengalahkan gaya scoped `.term-host` di `TerminalTab.svelte` tanpa menyuntingnya. Bila saat verifikasi build/manual pendekatan lebar/tinggi ini memberi ruang kosong atau terpotong, sesuaikan hanya aturan di file ini (syarat tiket: tidak ada skala ganda, `fit` benar, efek terlihat di sidebar dan terminal). Penyelarasan nama kelas terhadap `TerminalTab.svelte` dicek ulang oleh Wiring.

### 2. `frontend/src/modules/zoom/index.ts`

Struktur dan signature:

```ts
import './zoom.css';
import { readable, type Readable } from 'svelte/store';
import { onMenuEvent, getAppConfig, setAppConfig } from '../appmenu';

const MIN = 50, MAX = 300, STEP = 10, DEFAULT = 100, PERSIST_DELAY_MS = 300;

let percent = DEFAULT;
let setStore: ((v: number) => void) | null = null;
export const zoomPercent: Readable<number> = readable(DEFAULT, (set) => {
  setStore = set; set(percent);
  return () => { setStore = null; };
});
export function getZoomPercent(): number { return percent; }

function clamp(n: number): number   // bulatkan, NaN/tak hingga -> DEFAULT, batasi MIN..MAX

function applyZoom(next: number): void
```

Isi `applyZoom(next)`:

1. `percent = clamp(next)`; `setStore?.(percent)`.
2. `document.documentElement.style.setProperty('--ui-scale', String(percent / 100))`.
3. Font terminal, toleran:
   `import('../terminal').then((m) => (m as any).setTerminalFontScale?.(percent)).catch(() => {});`
   Cast `as any` wajib supaya TypeScript tidak gagal sebelum ekspor ada. Bila `import('../terminal')` mengeluh soal resolusi modul di Vite, gunakan `import(/* @vite-ignore */ '../terminal')`; modul itu ada (`modules/terminal/index.ts`), jadi seharusnya cukup tanpa itu. Jangan memicu error tak tertangkap.

Persist debounce (satu timer, `setTimeout`, bukan `setInterval`):

```ts
let persistTimer: ReturnType<typeof setTimeout> | undefined;
function schedulePersist(): void {
  if (persistTimer !== undefined) clearTimeout(persistTimer);
  persistTimer = setTimeout(() => {
    persistTimer = undefined;
    void setAppConfig({ zoom: percent }).catch(() => {});
  }, PERSIST_DELAY_MS);
}
```

Aksi menu:

```ts
function step(action: 'in' | 'out' | 'reset'): void {
  const next = action === 'in' ? percent + STEP : action === 'out' ? percent - STEP : DEFAULT;
  if (next === percent && clamp(next) === percent) { /* tetap boleh lanjut tanpa efek */ }
  applyZoom(next);
  schedulePersist();
}
onMenuEvent('menu:zoom', (p) => step(p.action));
```

Boleh melewati `schedulePersist` bila nilai tidak berubah (mis. sudah di 300 dan tekan Cmd+=), agar tidak menulis config sia-sia.

Pemulihan saat modul dimuat:

```ts
void getAppConfig().then((cfg) => {
  // Jangan menimpa bila pengguna sudah menekan zoom sebelum config terbaca.
  if (persistTimer === undefined && percent === DEFAULT) applyZoom(cfg.zoom);
}).catch(() => {});
```

Aturan: bila pengguna sudah mengubah zoom sebelum konfigurasi terbaca (ditandai `persistTimer !== undefined` atau `percent !== DEFAULT`), nilai pengguna menang. Pemulihan TIDAK memanggil `schedulePersist` (tidak menulis balik nilai yang baru dibaca).

Catatan penting: modul `zoom` dimuat lewat glob sebelum `mount`, dan `setTerminalFontScale` baru berefek bila `modules/terminal` sudah punya ekspor (Wiring). Sebelum Wiring, panggilan dinamis itu aman (no-op). Terminal yang dibuat setelah pemulihan memakai `currentTerminalFontSize()` (Wiring), jadi urutan pemulihan versus pembuatan terminal tidak masalah.

Larangan: tanpa `console.log`, tanpa `setInterval`, tanpa `registerPanel`, tanpa UI, tanpa menyentuh `localStorage`.

### 3. Verifikasi build

- `cd frontend && npm run build` harus lulus (zoom tidak boleh bergantung pada ekspor terminal yang belum ada; dijamin oleh impor dinamis + `as any`).
- Pastikan `git status`/diff hanya menunjukkan `frontend/src/modules/zoom/`.
- Pastikan tidak ada impor dari `../appmenu` yang bersifat melingkar (appmenu tidak mengimpor zoom).

## Perubahan

Dibuat:

- `frontend/src/modules/zoom/index.ts`
- `frontend/src/modules/zoom/zoom.css`

Tidak ada file lain yang dibuat atau diubah.

## Wiring

Yang diekspos untuk langkah Wiring (JANGAN disambungkan di fitur ini):

- Modul dimuat otomatis oleh glob `main.ts` karena punya `index.ts`; tidak perlu edit `main.ts`.
- Pemanggilan `setTerminalFontScale(percent)` dari `applyZoom` lewat impor dinamis `import('../terminal')`. Agar berfungsi, Wiring menambah `modules/terminal/font-scale.ts` dan ekspor ulangnya di `modules/terminal/index.ts` (overview 2.4, checklist 2 dan 3).
- Aturan CSS `#app .term-host` di `zoom.css` bergantung pada nama kelas `.term-host` di `TerminalTab.svelte`. Wiring memastikan kelas itu cocok; bila berbeda, Wiring boleh menyesuaikan `zoom.css` untuk penyelarasan itu saja. Wiring juga memverifikasi tidak ada skala ganda dan `fit.fit()` menghasilkan `session.resize` yang benar (uji manual Cmd+= / Cmd+- / Cmd+0 saat fokus di terminal; koordinat mouse xterm/seleksi teks di bawah `zoom` bersarang perlu dicek).
- `zoomPercent` (store) dan `getZoomPercent()` tersedia untuk tiket lain (mis. palet perintah); tidak ada konsumen pada tiket ini.

## Selesai bila

- `frontend/src/modules/zoom/index.ts` dan `zoom.css` ada dan hanya itu yang berubah di repo.
- `cd frontend && npm run build` sukses tanpa dependency baru (guard utama; tanpa unit test).
- Kode: langkah +10 / -10 / reset 100 dengan clamp 50..300 tercermin di `step`/`clamp`; `--ui-scale` di-set pada `document.documentElement`; `#app` ber-`zoom`, `.term-host` dikompensasi; `setTerminalFontScale` dipanggil hanya lewat impor dinamis toleran; `setAppConfig({zoom})` dipanggil lewat satu `setTimeout` debounce 300 ms; `getAppConfig()` dipakai saat dimuat; `getZoomPercent` dan `zoomPercent` diekspor.
- Tidak ada `setInterval`, polling, `console.log`, atau UI; tidak ada file di luar `frontend/src/modules/zoom/` yang disunting.
