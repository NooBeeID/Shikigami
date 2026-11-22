# Plan: Entri Laws dan info suntikan di panel

## Metadata

- Tiket: `grimoire-ticket-02-grimoire-inject` (`docs/tickets/grimoire-ticket-02-grimoire-inject.md`)
- Fitur: `laws-ui`
- ownedPaths: `frontend/src/modules/grimoire/laws/`
- Depends on foundation: tidak (murni frontend; foundation hanya stub Go). Bergantung pada tiket 01 yang sudah ter-merge: `frontend/src/modules/grimoire/client.ts` (`grimoireStatus`, `GrimoireStatus`) harus sudah ada.
- Rujukan wajib: `docs/plans/grimoire-ticket-02-grimoire-inject/overview.md` (bagian 2.2 butir 5, 2.4, 2.6, 6, 7).

## Tujuan

Pengguna dapat membuka entri "Laws" di panel Grimoire, mengedit `laws.md` project aktif lewat editor inline (textarea monospace, penghitung byte, Simpan, indikator tersimpan/galat, catatan "berlaku untuk sesi baru", tombol kembali), dan melihat baris "Disuntik ke sesi baru: N byte, ~M token" yang menyegarkan diri lewat event `grimoire.changed` tanpa polling. Semua dikemas dalam satu komponen slot (`LawsSlot.svelte`) yang nanti ditanam oleh langkah Wiring di `GrimoirePanel.svelte`.

## Path yang dimiliki

HANYA boleh membuat/mengubah berkas di `frontend/src/modules/grimoire/laws/`:
- `lawsStore.ts`
- `LawsEditor.svelte`
- `InjectionInfo.svelte`
- `LawsSlot.svelte`

Jangan menyentuh: `GrimoirePanel.svelte`, `client.ts`, `store.ts`, `index.ts` (tiket 01), `frontend/src/lib/*`, `frontend/src/styles/theme.css`, `package.json`/lockfile, apa pun di `internal/`, `cmd/`, atau path fitur lain. Jangan menambah token tema global dan dependency baru.

## Kontrak

Dipakai (sudah ada):
- `call<T>(method, params?)` dan `onEvent(topicPrefix, fn(topic, payload))` dari `../../../lib/ipc` (`onEvent` mengembalikan fungsi berhenti; callback menerima `payload: unknown`; filter berdasarkan awalan topik).
- `grimoireStatus(projectId: string): Promise<GrimoireStatus>` dan tipe `GrimoireStatus` (field dipakai: `exists`, `indexBytes`, `estTokens`) dari `../client` (tiket 01). Bila signature nyata di `client.ts` berbeda dari yang dikutip, ikuti kode nyata dan catat di bagian Perubahan.
- Token tema yang ada: `--bg`, `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--danger` (`frontend/src/styles/theme.css`). Svelte 5 runes.
- IPC (didaftarkan fitur `laws-measure-ipc`, overview 2.4): `grimoire.laws.get {projectId}` -> `{content: string}`; `grimoire.laws.set {projectId, content}` -> `{lawsBytes: number}`; event `grimoire.changed` dengan payload `{projectId}`. Galat IPC datang sebagai `Error` (pesan ditampilkan apa adanya).

Disediakan (signature final, jangan diubah):
```ts
// lawsStore.ts
import type { Writable, Readable } from 'svelte/store';
export interface LawsInjectionInfo { exists: boolean; indexBytes: number; estTokens: number }
export const lawsViewOpen: Writable<boolean>;
export const injectionInfo: Readable<LawsInjectionInfo | null>;
export function startLawsStore(projectId: string): () => void;
export function getLaws(projectId: string): Promise<string>;
export function setLaws(projectId: string, content: string): Promise<number>;
export function utf8Bytes(s: string): number;
export const MAX_LAWS_BYTES = 16 * 1024;
```
Komponen: `LawsEditor` props `{ projectId: string; onback: () => void }`; `InjectionInfo` props `{ projectId: string }`; `LawsSlot` props `{ projectId: string | null; exists: boolean }`.

## Langkah

1. **Prasyarat.** Pastikan `frontend/src/modules/grimoire/client.ts` ada dan mengekspor `grimoireStatus`/`GrimoireStatus`; lihat cara modul grimoire lain memakai Svelte 5 runes dan gaya CSS (mis. `GrimoirePanel.svelte`, bila ada) supaya konsisten. Bila `client.ts` tidak ada, berhenti dan laporkan.

2. **`lawsStore.ts`** (tanpa efek saat impor modul; tanpa `setInterval`).
   - `import { writable, readonly, type Writable, type Readable } from 'svelte/store'`; impor `call`, `onEvent` dari `'../../../lib/ipc'`; impor `grimoireStatus` dari `'../client'`.
   - `export const MAX_LAWS_BYTES = 16 * 1024;`
   - `export function utf8Bytes(s: string): number { return new TextEncoder().encode(s).length; }`
   - `export const lawsViewOpen: Writable<boolean> = writable(false);`
   - State privat: `const info = writable<LawsInjectionInfo | null>(null); export const injectionInfo: Readable<LawsInjectionInfo | null> = readonly(info);` dan `let generation = 0;`.
   - `getLaws`: `const r = await call<{ content?: string }>('grimoire.laws.get', { projectId }); return r?.content ?? '';`
   - `setLaws`: `const r = await call<{ lawsBytes?: number }>('grimoire.laws.set', { projectId, content }); return r?.lawsBytes ?? utf8Bytes(content);`
   - `startLawsStore(projectId)`:
     - `const gen = ++generation; let stopped = false;` `info.set(null);`
     - `async function refresh()`: `try { const s = await grimoireStatus(projectId); if (stopped || gen !== generation) return; info.set(s.exists ? { exists: true, indexBytes: s.indexBytes, estTokens: s.estTokens } : null); } catch { if (!stopped && gen === generation) info.set(null); }` (galat status tidak boleh melempar tak tertangani).
     - `const off = onEvent('grimoire.changed', (_t, payload) => { const id = (payload as { projectId?: string } | null)?.projectId; if (id === projectId) void refresh(); });`
     - `void refresh();`
     - Kembalikan fungsi berhenti idempoten: `() => { if (stopped) return; stopped = true; off(); if (gen === generation) info.set(null); }`.
   - Pemanggilan `startLawsStore` untuk project lain menaikkan `generation` sehingga hasil dan callback lama diabaikan.

3. **`InjectionInfo.svelte`** (props `{ projectId: string }`).
   - `import { injectionInfo } from './lawsStore';` `let { projectId }: { projectId: string } = $props();`
   - Markup: `{#if $injectionInfo && $injectionInfo.exists}<div class="inject-info" data-project={projectId} title="Estimasi kasar (~4 karakter per token)">Disuntik ke sesi baru: {$injectionInfo.indexBytes} byte, ~{$injectionInfo.estTokens} token</div>{/if}`. Tidak merender apa pun bila belum ada grimoire.
   - Gaya: `font-size: 11px; color: var(--fg-muted); padding: 2px 12px;` hanya token tema yang ada.

4. **`LawsEditor.svelte`** (props `{ projectId: string; onback: () => void }`).
   - State runes: `let text = $state(''); let loaded = $state(false); let loadError = $state<string | null>(null); let saving = $state(false); let saved = $state(false); let saveError = $state<string | null>(null);`
   - Turunan: `const bytes = $derived(utf8Bytes(text)); const over = $derived(bytes > MAX_LAWS_BYTES); const canSave = $derived(loaded && !saving && !over);`
   - Pemuatan: `$effect(() => { const id = projectId; let cancelled = false; loaded = false; loadError = null; saved = false; saveError = null; text = ''; getLaws(id).then((c) => { if (!cancelled) { text = c; loaded = true; } }).catch((e) => { if (!cancelled) loadError = e instanceof Error ? e.message : String(e); }); return () => { cancelled = true; }; });` Editor TIDAK berlangganan `grimoire.changed` (agar suntingan pengguna yang belum disimpan tidak tertimpa); `InjectionInfo` yang menyegarkan angka.
   - `async function save()`: bila `!canSave` keluar; `const id = projectId; saving = true; saved = false; saveError = null; try { await setLaws(id, text); if (id === projectId) saved = true; } catch (e) { if (id === projectId) saveError = e instanceof Error ? e.message : String(e); } finally { saving = false; }`.
   - `oninput` textarea: `saved = false; saveError = null;`.
   - Markup (urutan): header dengan tombol "Kembali" (`onclick={onback}`) dan judul "Laws"; catatan `Perubahan berlaku untuk sesi baru; sesi yang sedang berjalan tidak berubah.`; `<textarea bind:value={text} oninput=... disabled={!loaded} spellcheck="false" placeholder="Aturan khusus project ini, disuntik ke sesi agent baru." aria-label="Isi laws.md">`; baris status: penghitung `{bytes} / {MAX_LAWS_BYTES} byte` (kelas `over` berwarna `--danger` bila melebihi), tombol "Simpan" (`disabled={!canSave}`, teks "Menyimpan..." saat `saving`), indikator `Tersimpan` (bila `saved`) atau galat (`saveError`/`loadError`, `role="alert"`, warna `--danger`). Bila `over`, tampilkan teks "Melebihi batas {MAX_LAWS_BYTES} byte".
   - Gaya: textarea `font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; background: var(--bg-elev); color: var(--fg); border: 1px solid var(--border); border-radius: 4px; resize: vertical; min-height: 200px; width: 100%; box-sizing: border-box;` fokus `border-color: var(--accent)`; tombol memakai `--border`/`--bg-hover`, tombol Simpan aksen `--accent`; tombol nonaktif `opacity: .5`. Padding sisi 12px, tanpa overflow horizontal.

5. **`LawsSlot.svelte`** (props `{ projectId: string | null; exists: boolean }`).
   - Impor `lawsViewOpen`, `startLawsStore` dari `./lawsStore`, `LawsEditor`, `InjectionInfo`.
   - Efek siklus hidup: `$effect(() => { const id = projectId; if (!id || !exists) return; const stop = startLawsStore(id); return () => { stop(); lawsViewOpen.set(false); }; });` Teardown berjalan saat `projectId`/`exists` berubah dan saat unmount, jadi editor selalu tertutup pada pergantian project atau unmount. Jangan membaca `$lawsViewOpen` di dalam efek ini (hindari loop).
   - Markup: `{#if projectId && exists}{#if $lawsViewOpen}<LawsEditor {projectId} onback={() => lawsViewOpen.set(false)} />{:else}<button class="laws-entry" onclick={() => lawsViewOpen.set(true)}>Laws</button><InjectionInfo {projectId} />{/if}{/if}`. Bila `exists` false atau `projectId` null render kosong.
   - Gaya tombol entri: lebar penuh, rata kiri, `background: transparent; color: var(--fg); border: 0; padding: 6px 12px; cursor: pointer;` hover `var(--bg-hover)`.
   - Tambahkan `{#key projectId}` di sekitar `LawsEditor` bila perlu memastikan state editor ter-reset (efek pemuatan editor sudah menangani; kunci hanya cadangan).

6. **Verifikasi.** `cd frontend && npm run check && npm run build` (atau `make build-ui`). Perbaiki galat dan peringatan svelte-check pada berkas milik sendiri. Pastikan: tidak ada `setInterval`/`setTimeout` polling, `git diff --stat -- frontend/package.json frontend/package-lock.json` kosong, `git status` hanya menampilkan berkas baru di `frontend/src/modules/grimoire/laws/` dan tidak ada berkas lain yang berubah oleh Anda. Bila build gagal karena berkas fitur/tiket lain, jangan diperbaiki; laporkan.

## Perubahan

Dibuat (semua baru, di `frontend/src/modules/grimoire/laws/`):
- `lawsStore.ts`
- `LawsEditor.svelte`
- `InjectionInfo.svelte`
- `LawsSlot.svelte`

Tidak ada berkas yang diubah. Catat di laporan bila nama/signature nyata `client.ts` berbeda dari overview.

## Wiring

Yang diekspos untuk disambungkan langkah Wiring (JANGAN disambungkan di plan ini; `GrimoirePanel.svelte` tidak disentuh):
- `LawsSlot.svelte`: `<LawsSlot projectId={...} exists={...} />` diganti ke posisi komentar `<!-- slot: laws (tiket 2) -->` di `GrimoirePanel.svelte` (`projectId` = id project aktif atau null, `exists` = `status.exists`).
- `lawsViewOpen` dari `./laws/lawsStore`: panel membungkus daftar buku/status dengan `{#if !$lawsViewOpen} ... {/if}` agar editor menggantikan daftar saat terbuka.
- `LawsEditor` dan `InjectionInfo` dipakai internal oleh `LawsSlot`; panel tidak perlu mengimpornya langsung.
- Ketergantungan runtime pada IPC `grimoire.laws.get/set` (fitur `laws-measure-ipc`) dan field status `indexBytes`/`estTokens` (diisi blok Wiring di `ipc.go`); tanpa itu info suntikan menampilkan nilai sementara tiket 01 dan editor menampilkan galat IPC, bukan crash.

## Selesai bila

- Keempat berkas ada, signature `lawsStore.ts` dan props komponen persis sesuai Kontrak.
- `cd frontend && npm run check && npm run build` (`make build-ui`) sukses tanpa galat dari berkas milik fitur ini.
- Tidak ada `setInterval`/polling; refresh info hanya via `onEvent('grimoire.changed')` terfilter per `projectId`.
- `LawsSlot` kosong bila `exists` false atau `projectId` null; menutup editor (`lawsViewOpen` false) saat project berganti dan saat unmount.
- `LawsEditor` menonaktifkan Simpan bila belum termuat, sedang menyimpan, atau melebihi 16384 byte; menampilkan indikator tersimpan atau galat dan catatan perubahan berlaku untuk sesi baru.
- Tidak ada dependency baru, token tema baru, atau perubahan di luar `frontend/src/modules/grimoire/laws/`.
- Tanpa unit test; guard hanya build.
