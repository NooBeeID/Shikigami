# Plan: Sesi berakhir dan tombol Mulai ulang

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-06-workspace-persist.md` (`shikigami-v2-ui-ticket-06-workspace-persist`)
- Fitur: `restart-dead-session` (Sesi berakhir dan tombol Mulai ulang)
- ownedPaths: `frontend/src/modules/persist/DeadSession.svelte`
- Depends on foundation: tidak (overview bagian 1 dan 4: tidak ada `00-foundation.md`)
- Rujukan: `docs/plans/shikigami-v2-ui-ticket-06-workspace-persist/overview.md` (bagian 2.1 fakta repo, 2.4 kontrak frontend, 3 catatan khusus fitur, 7 aturan paralel). Jangan mengubah overview.

## Tujuan

Membuat satu komponen Svelte 5 mandiri `DeadSession.svelte`: overlay "Sesi berakhir" untuk daun terminal yang sesinya sudah tidak ada atau berstatus `exited`. Overlay menampilkan nama project dan provider, serta tombol "Mulai ulang" yang membuat sesi baru (project dan provider sama, tanpa melanjutkan percakapan agent, tanpa flag tambahan), memasukkan sesi ke store `sessions`, mengganti `sessionId` daun tanpa mengubah posisi/ukuran/judul pane, lalu memanggil `onRestarted(newId)`. Komponen TIDAK mendeteksi sesi mati dan TIDAK didaftarkan sendiri; keduanya urusan host (`TerminalGuard.svelte`, langkah Wiring).

## Path yang dimiliki

HANYA boleh dibuat/diubah:
- `frontend/src/modules/persist/DeadSession.svelte` (baru; folder `modules/persist/` belum ada, buat lewat penulisan file)

Dilarang menyentuh: `modules/persist/index.ts`, `register.ts`, `TerminalGuard.svelte`, `lib/stores/persist.ts` (milik `autosave-restore`/Wiring), `modules/layout/*`, `modules/terminal/*`, `lib/stores/layout.ts`, `lib/stores/sessions.ts`, `lib/stores/workspaces.ts`, `main.ts`, `App.svelte`, file Go, atau file lain apa pun. Jangan membuat file lain (tanpa test, tanpa helper terpisah). Jangan menambah dependency.

## Kontrak

Disediakan (overview 2.4):

```ts
// DeadSession.svelte, Svelte 5 $props
//   leaf: Leaf                              // daun terminal
//   onRestarted?: (newSessionId: string) => void
```

Dipakai (semua sudah ada di repo; hanya fungsi/store publik):
- `import type { Leaf } from '../layout/model'` (`Leaf = {type:'pane', id, kind, sessionId?, projectId?, provider?, filePath?, title, workspaceId?}`).
- `import { call } from '../../lib/ipc'` untuk `call<Session>('session.create', params)`.
- `import { sessions } from '../../lib/stores/sessions'` (`Writable<Session[]>`; hanya `sessions.update`).
- `import { workspaces, activeWorkspaceId } from '../../lib/stores/workspaces'` (`workspaces: Writable<Workspace[]>`, `activeWorkspaceId: Writable<string|null>`, `Workspace.projects: Project[]` dengan `Project {id, path, name}`).
- `import type { Session } from '../../lib/stores/types'`.
- `import { layout } from '../../lib/stores/layout'` dan tipe `LayoutDoc`, `Node` dari `../layout/model`; hanya `layout.serialize()` dan `layout.hydrate(doc)`.
- Pesan IPC: `session.create` params persis `{ workspaceId, projectId, cwd, providerId }` (tanpa `env`, `cols`, `rows`, atau argumen/flag lain). Hasil: `Session` (field `id`).
- Tema: variabel CSS `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok` (fallback boleh, tanpa warna hardcode).
- Tidak ada pendengar event baru (jangan `onEvent`), tidak ada polling, tidak ada timer.

Catatan: pastikan nama ekspor dan path import di atas sesuai dengan berkas sebenarnya (baca `frontend/src/lib/stores/layout.ts`, `lib/ipc.ts` sebelum menulis; jalur relatif dari `frontend/src/modules/persist/`).

## Langkah

1. Baca singkat (jangan ubah): `frontend/src/lib/ipc.ts` (signature `call`), `frontend/src/lib/stores/layout.ts` (ekspor `layout`, `serialize`, `hydrate`), `frontend/src/modules/layout/model.ts` (tipe `Leaf`, `Node`, `LayoutDoc`), dan satu komponen Svelte yang ada (mis. `modules/layout/panes/TerminalPane.svelte`) untuk gaya runes dan CSS. Konfirmasi path import.

2. Buat `frontend/src/modules/persist/DeadSession.svelte` dengan `<script lang="ts">`:
   - Props: `let { leaf, onRestarted }: { leaf: Leaf; onRestarted?: (newSessionId: string) => void } = $props();`
   - State: `let busy = $state(false); let error = $state<string | null>(null);`
   - Turunan:
     - `const project = $derived($workspaces.flatMap((w) => w.projects ?? []).find((p) => p.id === leaf.projectId) ?? null);` Cari hanya di workspace aktif agar sesuai kontrak: `$derived($workspaces.find((w) => w.id === $activeWorkspaceId)?.projects?.find((p) => p.id === leaf.projectId) ?? null)`. (Gunakan sintaks `$store` Svelte 5 di dalam `$derived`.)
     - `const providerLabel = $derived(leaf.provider ?? 'tidak diketahui');`
     - `const missingProject = $derived(!leaf.projectId || !project);`
     - `const canRestart = $derived(!busy && !missingProject && !!leaf.provider && !!$activeWorkspaceId);`
   - Fungsi pembantu murni `replaceSessionId(node: Node | null, leafId: string, newId: string): Node | null` (di dalam `<script>`): rekursif; bila `node.type === 'pane' && node.id === leafId` kembalikan `{ ...node, sessionId: newId }` (field lain, termasuk `title`, `id`, `kind`, `provider`, `projectId`, tidak berubah); bila `split` kembalikan `{ ...node, a: replaceSessionId(node.a, ...)!, b: replaceSessionId(node.b, ...)! }`; `null` tetap `null`. Bisa juga mutasi in-place karena `serialize()` mengembalikan salinan dalam; pilih salah satu dan konsisten.
   - `async function restart()`:
     1. Guard: `if (!canRestart) return;` ambil `const wsId = $activeWorkspaceId`, `const p = project`, `const providerId = leaf.provider` (null-check lagi demi tipe).
     2. `busy = true; error = null;`
     3. `try`:
        - `const s = await call<Session>('session.create', { workspaceId: wsId, projectId: p.id, cwd: p.path, providerId });` (tepat empat kunci itu).
        - Tambahkan ke store: `sessions.update((l) => (l.some((x) => x.id === s.id) ? l : [...l, s]));` (cegah duplikat; dilakukan SEBELUM hydrate agar `TerminalTab` menemukan sesi dan deteksi mati host tidak salah menilai).
        - Ganti `sessionId` daun: `const doc = layout.serialize(); doc.root = replaceSessionId(doc.root, leaf.id, s.id); layout.hydrate(doc);` (posisi, rasio, fokus, maximize, judul tetap karena `hydrateDoc` mempertahankannya; jangan memanggil `closePane`/`openPane`). Bila daun dengan `leaf.id` tidak ditemukan di dokumen (sudah ditutup pengguna), jangan hydrate dan tetap lanjut tanpa error (sesi tetap masuk store).
        - `onRestarted?.(s.id);`
     4. `catch (e)`: `error = e instanceof Error ? e.message : String(e);` (pesan singkat; fallback "Gagal memulai sesi").
     5. `finally`: `busy = false;` (komponen biasanya sudah di-unmount host setelah `sessionId` berganti; aman karena hanya menyetel state lokal).
   - Markup (overlay penuh pane, semantik dasar):
     ```svelte
     <div class="dead" role="alert">
       <div class="card">
         <div class="title">Sesi berakhir</div>
         <div class="info">
           <span>Project: {project ? project.name : 'tidak diketahui'}</span>
           <span>Provider: {providerLabel}</span>
         </div>
         {#if missingProject}
           <div class="note">Project tidak ditemukan</div>
         {/if}
         {#if error}
           <div class="err">{error}</div>
         {/if}
         <button type="button" disabled={!canRestart} onclick={restart}>
           {busy ? 'Memulai...' : error ? 'Coba lagi' : 'Mulai ulang'}
         </button>
       </div>
     </div>
     ```
     Teks wajib persis: "Sesi berakhir", "Mulai ulang", "Memulai...", "Project tidak ditemukan"; label tombol saat error "Coba lagi" (tombol tetap aktif). Bila `leaf.kind !== 'terminal'` render tidak apa-apa (`{#if leaf.kind === 'terminal'}` membungkus seluruh markup).
   - Gaya `<style>`: `.dead { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; background: var(--bg, #1e1e1e); color: var(--fg, #ddd); z-index: 5; }`; `.card` kolom, gap, padding, border `1px solid var(--border, #444)`, background `var(--bg-elev, ...)`, radius kecil, teks tengah; `.title` tebal; `.info` `color: var(--fg-muted)` kolom kecil; `.note` `color: var(--danger)`; `.err` `color: var(--danger)`; tombol dengan `var(--accent)` untuk border/latar, `:hover` `var(--bg-hover)`, `:disabled { opacity: .5; cursor: not-allowed; }`. Tanpa warna hardcode selain fallback di `var()`.

3. Verifikasi build: `cd frontend && npm run build` (tanpa unit test). File belum diimpor siapa pun (host dibuat oleh Wiring), jadi Vite/tsc tidak akan mengompilasinya lewat bundle; tambahan wajib: jalankan `cd frontend && npx svelte-check` bila skrip/paket tersedia (cek `package.json`, mis. `npm run check`) agar `DeadSession.svelte` ikut dicek tipe. Bila tidak ada skrip check, buat berkas uji SEMENTARA di luar repo (scratchpad) tidak diperlukan; cukup pastikan impor dan tipe benar lewat pembacaan. Jangan meninggalkan file sementara di repo.

## Perubahan

- Baru: `frontend/src/modules/persist/DeadSession.svelte`.
- Tidak ada file lain yang dibuat atau diubah.

## Wiring

Yang diekspos untuk disambungkan nanti (JANGAN disambungkan di fitur ini):
- Komponen default-export `frontend/src/modules/persist/DeadSession.svelte` dengan props `{ leaf: Leaf; onRestarted?: (newSessionId: string) => void }`.
- `TerminalGuard.svelte` (Wiring, overview bagian 5 langkah 3) mengimpor `./DeadSession.svelte` langsung, merendernya di atas pane (posisi absolut) saat daun dianggap mati (aturan overview 2.4), dan tidak merender `TerminalPane` saat mati. Setelah "Mulai ulang", `sessionId` daun berganti sehingga guard merender `TerminalPane` baru; `onRestarted` opsional bagi host.
- `register.ts` mendaftarkan ulang jenis `terminal` dengan `component: TerminalGuard` lewat `registerPaneKind`. Semua itu dikerjakan agen Wiring.
- Komponen mengharapkan host menyediakan kontainer ber-`position: relative` karena overlay memakai `position: absolute; inset: 0`.

## Selesai bila

- `frontend/src/modules/persist/DeadSession.svelte` ada, memakai runes (`$props`, `$state`, `$derived`) dan TypeScript, dan menampilkan "Sesi berakhir", nama project, provider, dan tombol "Mulai ulang".
- `session.create` dipanggil hanya dengan `{workspaceId, projectId, cwd, providerId}`; tidak ada `env`/flag/argumen tambahan, tidak ada `--continue`/`--resume`.
- Setelah sukses: sesi ada di store `sessions` tanpa duplikat, `sessionId` daun diganti lewat `layout.serialize` -> ubah -> `layout.hydrate` (daun lain, id daun, posisi, rasio, judul tidak berubah), lalu `onRestarted(newId)` dipanggil.
- Selama proses tombol nonaktif dan berlabel "Memulai..."; gagal menampilkan pesan error dan tombol "Coba lagi" aktif; project tidak ada di workspace aktif menampilkan "Project tidak ditemukan" dan tombol nonaktif.
- Tidak ada pendengar event baru, polling, timer, atau dependency baru; tidak ada file lain yang disentuh.
- Guard: `cd frontend && npm run build` sukses (dan check tipe lewat `npm run check`/`svelte-check` bila tersedia); aplikasi tetap bisa dibangun.

### Asumsi yang dicatat

- `layout.hydrate` mempertahankan id daun, fokus, dan maximize (sesuai overview 2.1); bila ternyata tidak, laporkan, jangan ubah modul layout.
- Provider ditampilkan sebagai id mentah dari `leaf.provider` (`claude|codex|shell`); tidak memakai store `providers` agar komponen tetap mandiri.
- Pencarian project hanya di workspace aktif karena layout hanya memuat daun workspace aktif.
