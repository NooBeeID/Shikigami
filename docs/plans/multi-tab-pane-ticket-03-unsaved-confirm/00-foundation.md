# 00-foundation: tabRegistry.ts

Baca dulu `overview.md` (kontrak bagian 2.2). Dikerjakan SATU agent sebelum fitur paralel. Tujuan: `files/tabRegistry.ts` ada dan type-check, sehingga `confirm-close-dialog` (mengimpor `getFileTab`) dan `file-tab-registry` bisa berjalan paralel tanpa stub.

Path yang boleh disentuh: HANYA `frontend/src/modules/files/tabRegistry.ts` (file baru). Prasyarat: tiket 01 dan 02 sudah ter-merge.

## Langkah

1. Baca `frontend/src/modules/files/index.ts` dan satu store ringan di `frontend/src/lib/stores/` untuk melihat gaya penulisan (`svelte/store`).
2. Buat `frontend/src/modules/files/tabRegistry.ts`:
   - `import { writable, get, type Readable } from 'svelte/store';`
   - `export interface FileTabHandle { isDirty(): boolean; save(): Promise<boolean>; }`
   - `const handles = new Map<string, FileTabHandle>();` dan `const dirtySet = writable<ReadonlySet<string>>(new Set());`
   - `export const dirtyTabs: Readable<ReadonlySet<string>> = { subscribe: dirtySet.subscribe };`
   - `registerFileTab(tabId, handle)`: `handles.set`; kembalikan fungsi cabut yang hanya mencabut bila `handles.get(tabId) === handle` (panggil `unregisterFileTab`).
   - `unregisterFileTab(tabId)`: `handles.delete`, lalu `setFileTabDirty(tabId, false)`.
   - `getFileTab(tabId)`: `handles.get`.
   - `setFileTabDirty(tabId, dirty)`: no-op bila status sama; bila berbeda `dirtySet.update` dengan `new Set(old)` lalu add/delete (imutabel agar reaktif).
   - `isTabDirty(tabId)`: `get(dirtySet).has(tabId)`.
   - Komentar singkat berbahasa Indonesia yang menjelaskan fungsi registri sebagai jembatan FileTab (dirty) dan penutup tab.
3. JANGAN mengedit `files/index.ts`, `FileTab.svelte`, `FilePane.svelte`, atau membuat `confirmClose/` (milik fitur).
4. Build: `cd frontend && npm run build`. Harus lulus.
5. Commit hanya file ini bila alur proyek mengharuskan; laporkan path file.
