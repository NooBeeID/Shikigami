# Plan: Logika terakhir dibuka dan pencocokan path

## Metadata
- Tiket: `welcome-screen-ticket-02-welcome-screen` (`docs/tickets/welcome-screen-ticket-02-welcome-screen.md`)
- Fitur: `welcome-logic`
- ownedPaths: `frontend/src/modules/welcome/logic.ts`
- Depends on foundation: tidak (tidak ada `00-foundation.md`)
- Kontrak bersama: `docs/plans/welcome-screen-ticket-02-welcome-screen/overview.md` bagian 2.2 (jangan diubah)

## Tujuan
Menyediakan logika murni dan aksi untuk layar welcome: memilih workspace "Terakhir dibuka" (`recentWorkspaces`), mencocokkan path folder ke workspace tersimpan (`findWorkspaceByPath`), serta aksi `openFromDisk`, `createNew`, dan `openRecent`. Semua error ditampilkan lewat toast, tidak dilempar ke UI.

## Path yang dimiliki
HANYA `frontend/src/modules/welcome/logic.ts`. Jangan sentuh `Welcome.svelte`, `index.ts`, `App.svelte`, `lib/stores/*`, `modules/openfolder/*`, `modules/topbar/*`, `modules/appmenu/*`, kode Go, atau file fitur lain. Folder `frontend/src/modules/welcome/` mungkin belum ada; buat bersama file ini. Jangan menambah dependency atau unit test.

## Kontrak
Ekspor persis (lihat overview 2.2 untuk perilaku):
```ts
export function recentWorkspaces(workspaces: Workspace[], openIds: string[], limit = 5): Workspace[];
export function findWorkspaceByPath(workspaces: Workspace[], path: string): Workspace | undefined;
export async function openFromDisk(): Promise<void>;
export async function createNew(): Promise<Workspace | null>;
export async function openRecent(id: string): Promise<void>;
```

Impor yang dipakai:
- `workspaces`, `openWorkspace`, `createWorkspace`, tipe `Workspace` dari `../../lib/stores`
- `newWorkspaceFromFolder`, `errMessage` dari `../openfolder/logic`
- `pickDirectory` dari `../appmenu` (bertipe `Promise<string | null>`; sudah mengembalikan `null` bila batal/kosong)
- `showToast` dari `../openfolder/toast-store` (`showToast(text, kind = 'info', ms)`)
- `defaultWorkspaceName` dari `../topbar/util` (`(list: Workspace[]) => string`)
- `get` dari `svelte/store`

Catatan: `openWorkspace` dan `Workspace.lastOpenedAt` berasal dari tiket 1 (`welcome-screen-ticket-01-session-workspaces`). Saat plan ini ditulis, `openWorkspace` belum ada di `lib/stores/workspaces.ts` pada working tree. Periksa `frontend/src/lib/stores/workspaces.ts` dan `types`; bila tiket 1 sudah merge, nama ekspor harus ada; ikuti nama persis di kode. JANGAN menambahkan `openWorkspace` atau field `lastOpenedAt` sendiri (di luar ownedPaths). Bila tiket 1 belum ada sehingga build gagal hanya karena impor itu, laporkan sebagai blocker alih-alih mengubah file lain.

## Langkah
1. Verifikasi impor: baca `frontend/src/lib/stores/index.ts` (barrel), `workspaces.ts`, serta tipe `Workspace` (apakah `lastOpenedAt?: number` dan `projects: {path: string}[]` ada). Pastikan `openWorkspace`, `createWorkspace`, `workspaces`, `Workspace` terekspor dari `../../lib/stores`.
2. Buat `frontend/src/modules/welcome/logic.ts` dengan header impor (bagian Kontrak), komentar singkat bahasa Indonesia.
3. Helper privat (tidak diekspor):
   ```ts
   function normalizePath(p: string): string {
     let s = p;
     while (s.length > 1 && (s.endsWith('/') || s.endsWith('\\'))) s = s.slice(0, -1);
     return s;
   }
   ```
4. `recentWorkspaces(workspaces, openIds, limit = 5)`: murni.
   ```ts
   return workspaces
     .filter((w) => (w.lastOpenedAt ?? 0) > 0 && !openIds.includes(w.id))
     .sort((a, b) => (b.lastOpenedAt ?? 0) - (a.lastOpenedAt ?? 0))
     .slice(0, limit);
   ```
   `filter` sudah menghasilkan array baru sehingga `sort` tidak memutasi input.
5. `findWorkspaceByPath(workspaces, path)`: murni. `const target = normalizePath(path);` lalu `return workspaces.find((w) => w.projects.some((p) => normalizePath(p.path) === target));` (pakai `(w.projects ?? [])` bila tipe mengizinkan undefined).
6. Flag modul `let busy = false;`.
7. `openFromDisk()`:
   ```ts
   export async function openFromDisk(): Promise<void> {
     if (busy) return;
     busy = true;
     try {
       const path = await pickDirectory();
       if (!path) return;
       const existing = findWorkspaceByPath(get(workspaces), path);
       if (existing) await openWorkspace(existing.id);
       else await newWorkspaceFromFolder(path);
     } catch (e) {
       showToast(errMessage(e), 'error');
     } finally {
       busy = false;
     }
   }
   ```
   Batal dialog (`null`) return tanpa menyentuh store. `newWorkspaceFromFolder` sudah memanggil `openWorkspace`; jangan panggil lagi.
8. `createNew()`: 
   ```ts
   export async function createNew(): Promise<Workspace | null> {
     try {
       return await createWorkspace(defaultWorkspaceName(get(workspaces)));
     } catch (e) {
       showToast(errMessage(e), 'error');
       return null;
     }
   }
   ```
   JANGAN memanggil `openWorkspace` (UI yang membukanya setelah input nama inline). Perhatikan: bila `createWorkspace` dari tiket 1 otomatis menambah ke `openWorkspaceIds`/memilih workspace, itu di luar kendali fitur ini; cukup catat di laporan.
9. `openRecent(id)`: `try { await openWorkspace(id); } catch (e) { showToast(errMessage(e), 'error'); }`.
10. Jalankan guard: `cd frontend && npm run build` dan `npm run check` (svelte-check). Perbaiki hanya error di `logic.ts`; error dari file lain (mis. `Welcome.svelte` belum ada, atau `openWorkspace` belum ada dari tiket 1) bukan tanggung jawab fitur ini, catat saja.

## Perubahan
- Dibuat: `frontend/src/modules/welcome/logic.ts`
- Tidak ada file lain yang dibuat atau diubah.

## Wiring
Fitur ini hanya mengekspos modul `frontend/src/modules/welcome/logic.ts` dengan lima ekspor di bagian Kontrak. Tidak ada registrasi, tidak ada `index.ts` (milik `welcome-ui`, yang mengimpor `./logic` untuk `recentWorkspaces`, `openFromDisk`, `createNew`, `openRecent`). `App.svelte` tidak memakai `logic.ts` langsung; penyambungan `Welcome` ke `App.svelte` dilakukan oleh `welcome-wiring`. Jangan menyambungkan apa pun di sini.

## Selesai bila
- [ ] `logic.ts` ada dengan kelima ekspor dan signature persis seperti Kontrak.
- [ ] `recentWorkspaces` mengecualikan `lastOpenedAt` undefined/0 dan id yang ada di `openIds`, terurut menurun, dipotong ke `limit` (default 5), tanpa memutasi input.
- [ ] `findWorkspaceByPath` cocok walau salah satu sisi punya slash/backslash akhir; root `/` tetap utuh; `undefined` bila tidak ada.
- [ ] `openFromDisk` memakai guard `busy` yang direset di `finally`; batal dialog tidak menyentuh store; path ada -> `openWorkspace`, path baru -> `newWorkspaceFromFolder`; error -> `showToast(errMessage(e), 'error')`.
- [ ] `createNew` memakai `defaultWorkspaceName(get(workspaces))`, tidak membuka workspace, mengembalikan `null` + toast bila gagal.
- [ ] `openRecent` membuka workspace dengan try/catch toast.
- [ ] Hanya `frontend/src/modules/welcome/logic.ts` yang berubah (cek `git status`).
- [ ] Guard: build frontend (`cd frontend && npm run build`, dan `npm run check`) tidak menghasilkan error yang berasal dari `logic.ts`. Tanpa unit test.
