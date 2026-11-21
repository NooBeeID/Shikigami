# Foundation: stub store grup dan registri slot

Rujukan: `overview.md` bagian 2.3 (antarmuka store) dan 2.4 (registri slot). Dikerjakan satu agen SEBELUM `group-daemon`, `groups-store`, dan `group-tabs-ui` berjalan paralel. Tujuan: kontrak frontend bisa dikompilasi oleh `group-tabs-ui` tanpa menunggu `groups-store`. Tanpa unit test; guard `cd frontend && npm run build`.

Berkas yang boleh disentuh Foundation (hanya ini):
- `frontend/src/lib/stores/groups.ts` (baru; kelak ditimpa penuh oleh `groups-store`)
- `frontend/src/modules/shell/registry.ts` (tambah saja)
- `frontend/src/modules/shell/index.ts` (tambah re-export)
- `frontend/src/modules/sidebar-left/registry.ts` (baru)
- `frontend/src/modules/sidebar-left/index.ts` (tambah re-export)

## Langkah

1. Baca `modules/shell/registry.ts`, `modules/shell/index.ts`, `modules/sidebar-left/index.ts`, `lib/stores/types.ts` (tipe `Session`) untuk nama asli.
2. `modules/shell/registry.ts`: tambah `LayoutHeaderSpec {id, component: Component<any>, order?}`, store `layoutHeaderStore`, `registerLayoutHeader(spec)` (pola persis `registerTopbarItem`) dan `layoutHeaders(): Readable<LayoutHeaderSpec[]>` terurut `order` lalu `id` (pakai `byOrder` yang ada). Jangan mengubah ekspor yang sudah ada. `modules/shell/index.ts`: tambah `registerLayoutHeader`, `layoutHeaders`, tipe `LayoutHeaderSpec` ke baris re-export registri.
3. `modules/sidebar-left/registry.ts` (baru) sesuai overview 2.4: `SessionListWrapperProps` (`onerror`, `list: Snippet<[filter: (s: Session) => boolean]>`), `SessionListWrapperSpec`, `registerSessionListWrapper` (terakhir menang), `sessionListWrapper: Readable<SessionListWrapperSpec | null>`, `SessionMenuItemProps {sessionId, onDone?}`, `SessionMenuItemSpec {id, component, order?}`, `registerSessionMenuItem`, `sessionMenuItems()` terurut. `Session` diimpor dari `../../lib/stores/types`. `modules/sidebar-left/index.ts`: tambah re-export registri dan tipenya (tetap mengekspor `AgentSidebar`).
4. `lib/stores/groups.ts` (stub): ekspor PERSIS semua simbol overview 2.3: tipe `SessionGroup {id,name,order,main}`, `MAIN_GROUP_NAME`, store `groups`, `mainGroupId`, `activeGroupId`, `activeGroup`, `sessionGroupMap`, `groupSessionCounts`, `groupsError`, fungsi `initGroups` (mengembalikan no-op cleanup), `createGroup`, `renameGroup`, `deleteGroup`, `reorderGroup`, `setActiveGroup`, `moveSession`, `groupOfSession`. Implementasi murni in-memory: state awal dua grup ("Utama" dengan `main: true`, "Grup 2"), `activeGroupId` = Utama, `sessionGroupMap` dan `groupSessionCounts` dihitung dari store `sessions` (tanpa entri = Utama), aksi mengubah state memori dengan aturan dasar (trim, nama kosong ditolak lewat `groupsError`, Utama tidak bisa dihapus, order rapat). Stub TIDAK menyentuh store layout, persist, IPC, localStorage, maupun `workspaces`. Komentar header: `// STUB(foundation): diganti seluruhnya oleh groups-store`.
5. Jalankan `cd frontend && npm run build`; harus lulus tanpa galat tipe. Pastikan tidak ada perubahan tampilan (tidak ada yang memanggil registri baru).

## Selesai bila

- Semua simbol kontrak 2.3/2.4 terekspor dengan tanda tangan sesuai overview dan build frontend lulus.
- Tidak ada berkas lain berubah (tidak ada Go, tidak ada `types.ts`/`autosave.ts`); tidak ada dependency baru.
