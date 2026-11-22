# Plan: Tampilan agent di thread

## Metadata

- Tiket: `global-agents-ticket-03-agents-orchestrator` (`docs/tickets/global-agents-ticket-03-agents-orchestrator.md`)
- Fitur: `thread-agent-ui` (Tampilan agent di thread)
- ownedPaths: `frontend/src/modules/thread/`
- Depends on foundation: tidak (foundation hanya sisi Go; fitur ini hanya bergantung pada nama field JSON di overview 2.2 dan 2.7)
- Overview: `docs/plans/global-agents-ticket-03-agents-orchestrator/overview.md` (jangan diubah)

## Tujuan

Thread di frontend menampilkan agent global: baris peserta menunjukkan `role · agentName · kategori` plus model; header thread menunjukkan agent/provider/model master dan penanda teks "tanpa agent Orchestrator (Claude polos)" tepat saat `thread.noOrchestrator` true. Nama kategori dipetakan dari IPC `agents.list` lewat klien thread sendiri (cache, muat ulang pada event `agents.changed`); kegagalan memuat jatuh ke id kategori mentah tanpa merusak tampilan. Thread lama tanpa field baru tampil persis seperti sebelumnya, tanpa galat konsol.

## Path yang dimiliki

HANYA `frontend/src/modules/thread/`. Jangan menyentuh:
- `frontend/src/modules/settings/` (dilarang mengimpor dari `modules/settings/agents`; definisikan tipe sendiri),
- berkas di luar `frontend/src/modules/thread/` mana pun, semua Go (`internal/`, `cmd/`), dan `docs/`,
- fitur paralel Go (`thread-spawn-agent`, `thread-master-agent`) dan file wiring.

Tidak ada perubahan `index.ts` yang diperlukan; tidak ada registrasi baru.

## Kontrak

Dipakai (lihat overview 2.1, 2.2, 2.7; tidak diulang di sini):
- Field JSON dari backend: `Participant.agentId/agentName/category/providerId/model` (string, `omitempty`, `category` = id kategori) dan `Thread.noOrchestrator` (bool, `omitempty`). Field tak ada = kosong.
- IPC `agents.list` -> `{categories: {id,name,role,builtin}[], agents: {id,name,categoryId,providerId,model,isDefault}[]}`; event `agents.changed` (payload diabaikan, cukup jadi pemicu muat ulang).
- Helper yang sudah ada: `call`, `onEvent` dari `../../lib/ipc`; `asList` dari `thread-client.ts`.

Disediakan (internal modul thread; tidak ada IPC thread baru):
- `types.ts`: `Participant` + `agentId?, agentName?, category?, providerId?, model?: string`; `Thread` + `noOrchestrator?: boolean`; konstanta `AGENTS_METHODS = { list: 'agents.list' }` dan `AGENTS_EVENTS = { changed: 'agents.changed' }`; tipe `AgentCategoryInfo { id: string; name: string }`.
- `thread-agents.ts` (baru): store `agentCategories: Readable<Record<string, string>>` (id -> nama), `loadAgentCategories(): Promise<void>`, `subscribeAgentEvents(): () => void` (idempoten), `categoryLabel(map: Record<string,string>, categoryId: string): string` (murni), `participantSub(p, projectName, categoryNames): string` (murni), `masterOf(thread): Participant | undefined` (murni).

## Langkah

### 1. `frontend/src/modules/thread/types.ts`
- Ubah `Participant` (tambah komentar `// agentId.. ditambah epic global-agents-ticket-03`):
  ```ts
  export interface Participant {
    sessionId: string; projectId: string; role: ThreadRole; name: string; dead?: boolean; previousSessionIds?: string[];
    // Ditambah epic global-agents-ticket-03 (semua opsional; thread lama tanpa field ini).
    agentId?: string; agentName?: string; category?: string; providerId?: string; model?: string;
  }
  ```
- `Thread`: tambah `noOrchestrator?: boolean;` (dengan komentar epic yang sama).
- Tambah di bawah `THREAD_EVENTS`:
  ```ts
  export const AGENTS_METHODS = { list: 'agents.list' } as const;
  export const AGENTS_EVENTS = { changed: 'agents.changed' } as const;
  export interface AgentCategoryInfo { id: string; name: string }
  ```

### 2. `frontend/src/modules/thread/thread-client.ts`
- Normalisasi: perluas `normThread` agar defensif untuk thread lama dan payload tak lengkap. Peserta dinormalisasi lewat fungsi `normParticipant(p: Participant): Participant` yang mempertahankan semua field (`{ ...p }`) dan memaksa field string baru yang bertipe salah menjadi `undefined` (mis. `agentName: typeof p.agentName === 'string' ? p.agentName : undefined`, sama untuk `agentId`, `category`, `providerId`, `model`). `normThread` menjadi:
  ```ts
  function normThread(t: Thread): Thread {
    const parts = Array.isArray(t.participants) ? t.participants.filter((p) => p && typeof p === 'object').map(normParticipant) : [];
    return { ...t, participants: parts, noOrchestrator: t.noOrchestrator === true };
  }
  ```
  Dengan begitu `thread.get`/`thread.list`/event `thread.updated` (semuanya sudah lewat `normThread`) membawa field baru.
- Tambah impor `AGENTS_METHODS` dari `./types` dan fungsi:
  ```ts
  export async function listAgentCategories(): Promise<AgentCategoryInfo[]> {
    const res = await call<unknown>(AGENTS_METHODS.list, {});
    return asList<AgentCategoryInfo>(res, 'categories').filter((c) => c && typeof c.id === 'string' && typeof c.name === 'string');
  }
  export function onAgentsChanged(fn: () => void): () => void {
    return onEvent(AGENTS_EVENTS.changed, () => fn());
  }
  ```
  (impor `AGENTS_EVENTS` dan `AgentCategoryInfo` juga.) Hanya `categories` yang dibaca; abaikan `agents`.

### 3. `frontend/src/modules/thread/thread-agents.ts` (baru)
- Komentar Indonesia di kepala berkas: cache nama kategori untuk UI thread; sengaja tidak mengimpor `modules/settings/agents`.
- Impor `writable`, `get`, `Readable` dari `svelte/store`; `inWails` dari `../../lib/ipc`; `listAgentCategories`, `onAgentsChanged` dari `./thread-client`; tipe `Participant`, `Thread` dari `./types`.
- State: `const names = writable<Record<string, string>>({}); export const agentCategories: Readable<Record<string, string>> = { subscribe: names.subscribe };`
- `let loadGen = 0;` dan
  ```ts
  export async function loadAgentCategories(): Promise<void> {
    if (!inWails) return;
    const gen = ++loadGen;
    try {
      const list = await listAgentCategories();
      if (gen !== loadGen) return;
      const map: Record<string, string> = {};
      for (const c of list) map[c.id] = c.name;
      names.set(map);
    } catch {
      // Gagal memuat: pertahankan cache terakhir (atau kosong); UI jatuh ke id mentah.
    }
  }
  ```
  Jangan menulis ke `threadError` (kegagalan tidak boleh mengganggu halaman thread) dan jangan melempar.
- `export function categoryLabel(map: Record<string, string>, categoryId: string): string`: kosong -> `''`; bila `map[categoryId]` ada dan nama (case-insensitive) berbeda dari id -> `` `${categoryId} (${map[categoryId]})` `` (mis. `planner (Planner)`); bila ada dan sama -> nama itu saja; bila tak ada -> `categoryId` mentah.
- `export function masterOf(thread: Thread): Participant | undefined`: `thread.participants.find((p) => p.role === 'master')`.
- `export function participantSub(p: Participant, projectName: string, categoryNames: Record<string, string>): string`:
  - tanpa `p.agentName`: persis perilaku lama `[p.role, projectName].filter(Boolean).join(' · ')`;
  - dengan `agentName`: `[p.role, projectName, p.agentName, categoryLabel(categoryNames, p.category ?? ''), p.model].filter(Boolean).join(' · ')`. (Contoh: `peer · api-gateway · planner-1 · planner (Planner) · claude-opus`.)
- `let cleanup: (() => void) | null = null;`
  ```ts
  export function subscribeAgentEvents(): () => void {
    if (cleanup) return cleanup;
    void loadAgentCategories();
    const off = onAgentsChanged(() => void loadAgentCategories());
    cleanup = () => { off(); cleanup = null; };
    return cleanup;
  }
  ```
  `onEvent` dari `lib/ipc` harus aman di luar Wails (sama seperti `onThreadUpdated` yang sudah dipakai tanpa penjagaan); bila `onEvent` melempar di luar Wails, bungkus `onAgentsChanged` dengan try/catch dan kembalikan fungsi kosong.

### 4. `frontend/src/modules/thread/thread-store.ts`
- Impor `subscribeAgentEvents` dari `./thread-agents`.
- Di `subscribeThreadEvents()` panggil `const offAgents = subscribeAgentEvents();` bersama langganan lain, dan panggil `offAgents()` di dalam `stop`. Perilaku idempoten fungsi tidak berubah. Jangan mengubah fungsi lain.

### 5. `frontend/src/modules/thread/participants/ParticipantRow.svelte`
- Impor `agentCategories` dan `participantSub` dari `../thread-agents`.
- Ganti `sub` dengan:
  ```ts
  let categoryNames = $state<Record<string, string>>({});
  $effect(() => agentCategories.subscribe((v) => (categoryNames = v)));
  const sub = $derived(participantSub(participant, projectName, categoryNames));
  ```
  (pola `$effect` + subscribe sama seperti `status` di berkas itu.)
- Pada elemen `.sub` tambahkan `title={sub}` agar teks penuh terlihat saat dipotong ellipsis (CSS `overflow: hidden; text-overflow: ellipsis; white-space: nowrap` sudah ada). Informasi disampaikan sebagai teks, bukan warna. Tanpa perubahan gaya lain.

### 6. `frontend/src/modules/thread/ThreadHeader.svelte`
- Impor `masterOf` dari `./thread-agents`.
- Tambah turunan:
  ```ts
  const master = $derived(masterOf(thread));
  const masterInfo = $derived(
    master ? [master.agentName, master.providerId, master.model].filter(Boolean).join(' · ') : '',
  );
  ```
- Di `.top`, setelah badge status/"siap ditutup" dan sebelum `.actions`, render:
  ```svelte
  {#if thread.noOrchestrator}
    <span class="meta warn" title="Tidak ada agent Orchestrator default; master berjalan dengan Claude polos. Atur di Settings > Agent.">tanpa agent Orchestrator (Claude polos)</span>
  {:else if masterInfo}
    <span class="meta" title={`Master: ${masterInfo}`}>{masterInfo}</span>
  {/if}
  ```
  Saat `noOrchestrator` true dan master punya `providerId` (mis. claude), penanda tetap tampil (teks yang sama persis, `title` penjelas); info master tidak ditampilkan bersamanya agar tidak ambigu. Tidak ada penanda bila `noOrchestrator` false/undefined, termasuk thread lama (`masterInfo` kosong -> tidak ada elemen).
- CSS: `.meta { font-size: 13px; color: var(--fg-muted, #888); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; flex: 0 1 auto; }` dan `.meta.warn { border: 1px solid var(--border, #8884); border-radius: 10px; padding: 0 7px; }`. Warna tidak membawa makna sendirian (teks yang membawa). `.title` tetap `min-width: 0`, dan `.actions` tetap `flex: none` sehingga tombol tidak terdorong.

### 7. Pesan sistem dari backend
Tidak ada perubahan: pesan sistem seperti "gagal men-spawn ...: agent ..." sudah tampil di `MessageItem` lewat jalur `kind: 'system'`. Cukup verifikasi dengan membaca bahwa `MessageItem.svelte` tidak membatasi teks sistem; jangan mengubahnya.

### 8. Verifikasi
Dari `frontend/`: `npm run build` dan `npm run check` (harus tanpa galat tipe). Periksa dengan `grep -rn "settings/agents" frontend/src/modules/thread/` bahwa tidak ada impor terlarang, dan `git status` bahwa hanya berkas di `frontend/src/modules/thread/` yang berubah.

## Perubahan

Diubah:
- `frontend/src/modules/thread/types.ts`
- `frontend/src/modules/thread/thread-client.ts`
- `frontend/src/modules/thread/thread-store.ts`
- `frontend/src/modules/thread/participants/ParticipantRow.svelte`
- `frontend/src/modules/thread/ThreadHeader.svelte`

Dibuat:
- `frontend/src/modules/thread/thread-agents.ts`

## Wiring

Tidak ada yang perlu disambungkan: modul thread sudah terdaftar dan memanggil `subscribeThreadEvents()` (yang kini juga memulai cache `agents.list`/`agents.changed`) dari `index.ts` tanpa perubahan. Langkah wiring tiket hanya menyentuh `cmd/shikigamid/wire_thread.go`; sisi frontend hanya perlu verifikasi manual (header menampilkan agent+model, penanda tanpa Orchestrator, baris peserta dengan agent/kategori/model). Fitur ini bergantung pada backend mengirim field JSON di overview 2.2 dan IPC `agents.list`/event `agents.changed` dari tiket 01; bila `agents.list` tak tersedia, UI jatuh ke id kategori mentah.

## Selesai bila

- `cd frontend && npm run build` sukses dan `npm run check` tanpa galat tipe (guard; tanpa unit test).
- Baris peserta bersub `role · project · agentName · kategori · model` bila `agentName` ada; thread lama tampil seperti sebelumnya tanpa galat konsol.
- Header menampilkan agent/provider/model master dan penanda "tanpa agent Orchestrator (Claude polos)" (dengan `title` penjelas) hanya saat `noOrchestrator` true.
- Kegagalan `agents.list` tidak melempar dan tidak mengisi `threadError`; kategori tampil sebagai id mentah.
- Tidak ada impor dari `modules/settings/agents` dan tidak ada perubahan di luar `frontend/src/modules/thread/`.
