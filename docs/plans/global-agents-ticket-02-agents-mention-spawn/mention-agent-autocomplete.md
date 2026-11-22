# Plan: Autocomplete @ untuk agent di UI

## Metadata

- Tiket: `global-agents-ticket-02-agents-mention-spawn` (`docs/tickets/global-agents-ticket-02-agents-mention-spawn.md`)
- Fitur: `mention-agent-autocomplete`
- ownedPaths: `frontend/src/modules/mention/`
- Depends on foundation: tidak (tidak ada `00-foundation.md`). Bergantung pada tiket 1 hanya lewat kontrak IPC `agents.list` dan event `agents.changed` (runtime), bukan lewat impor kode.
- Rujukan: `docs/plans/global-agents-ticket-02-agents-mention-spawn/overview.md` bagian 2.4 (IPC `mention.send`), 2.5 (frontend), 6 (build guard), 7 (aturan paralel).

## Tujuan

Membuat composer mention (`MentionComposer.svelte`) mengenali agent global dan kategori ber-default: autocomplete `@` menampilkan sesi, agent, dan kategori (label teks "agent baru" bila belum ada sesinya); validasi submit tidak lagi menolak `@planner`; pengiriman membawa `projectId`; feedback menyebut sesi baru dibuka; galat Indonesia dari backend tampil apa adanya; placeholder baru.

## Path yang dimiliki

HANYA boleh menyentuh:
- `frontend/src/modules/mention/types.ts`
- `frontend/src/modules/mention/mention-client.ts`
- `frontend/src/modules/mention/MentionComposer.svelte`

Dilarang: mengimpor dari `frontend/src/modules/settings/agents/` atau `frontend/src/modules/thread/`; menyentuh `internal/`, `cmd/`, `lib/`, modul lain, `index.ts` (tidak berubah), `QueueBadge.svelte`, `queue-store.ts`. Jangan menambah unit test. Kebutuhan di luar path dicatat di laporan, jangan diedit.

## Kontrak

Dipakai (runtime, dari tiket 1; jangan didefinisikan ulang di Go):
- IPC `agents.list` -> `{categories: Category[], agents: Agent[]}`; Category `{id,name,role,builtin}`, Agent `{id,name,categoryId,providerId,model,isDefault}`.
- Event `agents.changed` (payload `{}`), didengar lewat `onEvent('agents.changed', ...)` dari `lib/ipc`.
- Store `activeProjectId`, `activeWorkspaceId` dari `../../lib/stores` (sudah diekspor; `activeProjectId: Writable<string | null>`).

Dipakai dari fitur paralel `mention-agent-resolve` (hanya kontrak JSON, overview 2.4):
- `mention.send` params tambahan opsional `projectId: string`.
- Hasil: `Message` + field opsional `spawned?: boolean` dan `spawnedSession?: string` (flat, nama JSON persis ini). Anggap keduanya opsional.
- Pesan galat Indonesia dari backend yang harus lolos apa adanya: mengandung "project konteks", "agent", atau "Gagal membuka sesi".

Disediakan (dikonsumsi `MentionComposer.svelte`, internal modul):
```ts
// types.ts
export interface AgentInfo { id: string; name: string; categoryId: string; providerId: string; model: string; isDefault: boolean }
export interface CategoryInfo { id: string; name: string; role: string; builtin: boolean }
export type MentionSuggestion =
  | ({ kind: 'session' } & NameInfo)
  | { kind: 'agent'; name: string; categoryName: string; providerId: string; model: string; isDefault: boolean; hasSession: boolean }
  | { kind: 'category'; name: string; defaultAgent: string; hasSession: boolean };
export interface MentionSendResult extends Message { spawned?: boolean; spawnedSession?: string }

// mention-client.ts
export function listAgents(): Promise<{ categories: CategoryInfo[]; agents: AgentInfo[] }>;
export function buildSuggestions(names: NameInfo[], agents: AgentInfo[], categories: CategoryInfo[]): MentionSuggestion[];
export function matchAgentRef(name: string, agents: AgentInfo[], categories: CategoryInfo[]): boolean;
export function onAgentsChanged(fn: () => void): () => void;
export function sendMention(workspaceId: string, to: string, text: string, projectId?: string): Promise<MentionSendResult>;
```

## Langkah

1. `types.ts`: tambahkan `AgentInfo`, `CategoryInfo`, `MentionSuggestion`, `MentionSendResult` persis seperti kontrak di atas (komentar Indonesia). Jangan ubah tipe yang ada.

2. `mention-client.ts`, tambahkan impor tipe baru dan fungsi berikut (komentar Indonesia):
   - `slug(s: string): string`: `s.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')` (helper lokal, tidak diekspor atau diekspor tipis).
   - `listAgents()`: `const res = await call<{categories?: unknown; agents?: unknown}>('agents.list', {})`; kembalikan `{ categories: asList<CategoryInfo>(res, 'categories'), agents: asList<AgentInfo>(res, 'agents') }`. `asList` menerima objek dan key, jadi gunakan `asList<CategoryInfo>(res, 'categories')` dengan `res` utuh.
   - `buildSuggestions(names, agents, categories)` (murni):
     a. Hasil awal: `names.map(n => ({kind:'session', ...n}))`.
     b. `taken = new Set(names.map(n => n.name))`; `hasSession(base) = names.some(n => n.name.startsWith(base + '-'))`.
     c. Untuk tiap agent: `name = slug(a.name)`; lewati bila kosong atau `taken.has(name)`; tambahkan `{kind:'agent', name, categoryName: categories.find(c => c.id === a.categoryId)?.name ?? a.categoryId, providerId, model, isDefault, hasSession: hasSession(name)}`; tambahkan `name` ke `taken`.
     d. Untuk tiap kategori yang punya agent default (`agents.find(a => a.categoryId === c.id && a.isDefault)`): `name = slug(c.name)`; lewati bila kosong atau `taken.has(name)`; tambahkan `{kind:'category', name, defaultAgent: slug(defaultAgent.name), hasSession: hasSession(name) || hasSession(slug(defaultAgent.name))}`; tambahkan ke `taken`.
     e. Urutan: sesi, agent, kategori. Tidak ada duplikasi nama.
   - `matchAgentRef(name, agents, categories)` (murni): `const n = slug(name)`; benar bila ada agent dengan `slug(a.name) === n`, atau kategori `c` dengan `slug(c.name) === n || c.id.toLowerCase() === n` yang punya agent default.
   - `onAgentsChanged(fn)`: `return onEvent('agents.changed', () => fn());`
   - `sendMention(workspaceId, to, text, projectId?)`: params `{ workspaceId, to, text }` ditambah `projectId` hanya bila string non-kosong; kembalikan `call<MentionSendResult>('mention.send', params)`.
   - `errorText`: sisipkan cabang baru SEBELUM cabang `ERR_NOT_FOUND` dan sebelum `ERR_INVALID|tidak valid|workspace`: bila `/project konteks|Gagal membuka sesi|agent/i.test(msg)` kembalikan `msg.slice(0, 160)`. Catatan: cabang `user_only` dan `ERR_CONFLICT|penuh` tetap dicek lebih dulu agar label khususnya tidak berubah; letakkan cabang baru tepat setelah cabang `user_only` dan sebelum `ERR_NOT_FOUND`. Pesan "Tidak ada project konteks ..." dan "Gagal membuka sesi agent @x: ..." harus tidak tertimpa label generik "Sesi tidak ditemukan" atau "Pesan tidak valid". Untuk pesan yang hanya berisi kode seperti `ERR_INVALID` tanpa kata kunci, perilaku lama tetap.
   - Impor `onEvent` sudah ada; tidak perlu impor baru dari luar modul selain tipe.

3. `MentionComposer.svelte`:
   - Impor tambahan: `activeProjectId` dari `../../lib/stores` (gabung dengan impor yang ada); `buildSuggestions, listAgents, matchAgentRef, onAgentsChanged` dari `./mention-client`; tipe `AgentInfo, CategoryInfo, MentionSuggestion` dari `./types`.
   - State baru: `let agents = $state<AgentInfo[]>([]); let categories = $state<CategoryInfo[]>([]);`.
   - `refreshAgents()`: bila `!inWails` kosongkan keduanya dan return; `try { const r = await listAgents(); agents = r.agents; categories = r.categories; } catch { agents = []; categories = []; }` (gagal = perilaku lama, tidak menyentuh `names`/`namesError`).
   - `$effect` baru memanggil `void refreshAgents()` sekali saat mount (atau di `onMount`); di `onMount` yang ada, gabungkan unsubscribe: `const offExit = onSessionExit(...)`, `const offAgents = onAgentsChanged(() => void refreshAgents())`, dan `void refreshAgents()`; return fungsi yang memanggil keduanya.
   - `suggestions = $derived(buildSuggestions(names, agents, categories))`; `filtered = $derived(suggestions.filter(s => s.name.startsWith(query)))` bertipe `MentionSuggestion[]`.
   - `pick(n: MentionSuggestion)`: logika sama, memakai `n.name`.
   - Daftar menu: `{#each filtered as s, i (s.kind + ':' + s.name)}` (key unik per kind+nama; untuk sesi `name` sudah unik di workspace). Atribut `role="option"`, `aria-selected`, kelas `active`, handler `onmousedown` tidak berubah.
     - `s.kind === 'session'`: tampilan lama (`@nama`, `projectName · providerId`, status).
     - `s.kind === 'agent'`: `@nama`, teks muted `{s.categoryName} · {s.providerId} · {s.model || 'default'}`, dan bila `!s.hasSession` badge teks `agent baru` (`<span class="new">agent baru</span>`).
     - `s.kind === 'category'`: `@nama`, teks muted `kategori · default: @{s.defaultAgent}`, dan badge `agent baru` bila `!s.hasSession`.
     - Badge `.new` punya teks terbaca (bukan hanya warna): border 1px dan font-size 12px, margin-left auto.
   - Navigasi panah/Enter/Tab/Escape tidak diubah.
   - `submit()`:
     - Ganti validasi nama: `const known = names.some(n => n.name === parsed.name) || matchAgentRef(parsed.name, agents, categories);`. Blok lama `if (!namesError && names.length > 0 && !names.some(...))` menjadi `if (!namesError && names.length > 0 && !known)`; blok `names.length === 0` menjadi `if (!namesError && names.length === 0 && !known && ws)`. Pesan galat lama dipertahankan.
     - Pengiriman: `const project = $activeProjectId ?? undefined; const msg = await sendMention(ws, parsed.name, body, project);`.
     - Feedback: bila `msg.spawned` dan `msg.state !== 'failed'`: `const sess = msg.spawnedSession || parsed.name; feedback = { kind:'ok', msg: \`Sesi baru @${sess} dibuka dan pesan ${stateLabel(msg.state, sess, msg.error)}\` }` — agar enak dibaca gunakan: untuk `delivered` -> "Sesi baru @X dibuka dan pesan terkirim", untuk `queued` -> "Sesi baru @X dibuka dan pesan diantrekan", untuk `awaiting_confirm` -> "Sesi baru @X dibuka; pesan menunggu konfirmasi". Implementasikan sebagai fungsi kecil `spawnLabel(state, sess)` di komponen yang memakai `stateLabel` untuk state selain tiga itu. Bila tidak `spawned`, pakai `stateLabel` seperti sebelumnya. `failed` tetap kind `err`.
     - Teks pesan hanya dikosongkan bila `msg.state !== 'failed'`; pada galat (`catch`) teks tidak dikosongkan (perilaku sekarang dipertahankan).
   - Placeholder textarea: `@nama-sesi atau @agent pesan...`. Pesan galat `parseMention` ("Awali pesan dengan @nama-sesi") boleh menjadi "Awali pesan dengan @nama-sesi atau @agent".
   - Style: tambahkan `.new { margin-left: auto; font-size: 12px; padding: 0 6px; border: 1px solid var(--accent, #5b94ff); border-radius: 8px; color: var(--accent, #5b94ff); }`. Karena `.st` juga memakai `margin-left: auto`, aman (hanya satu per baris).
   - Di luar Wails (`inWails` false): `refreshNames` dan `submit` tetap menghasilkan "Hanya tersedia di aplikasi desktop"; jangan memanggil `listAgents` bila `!inWails`.

4. Periksa tidak ada impor dari `modules/settings/agents` atau `modules/thread`: `grep -rn "settings/agents\|modules/thread\|\.\./thread" frontend/src/modules/mention/` harus kosong.

## Perubahan

- Diubah: `frontend/src/modules/mention/types.ts`
- Diubah: `frontend/src/modules/mention/mention-client.ts`
- Diubah: `frontend/src/modules/mention/MentionComposer.svelte`
- Tidak berubah: `index.ts`, `QueueBadge.svelte`, `queue-store.ts`.

## Wiring

Tidak ada yang perlu disambungkan oleh fitur ini; `index.ts` sudah mendaftarkan `MentionComposer` sebagai panel (`registerPanel`) dan tetap tidak diubah (overview bagian 5 butir 5). Yang diekspos untuk langkah wiring/verifikasi:
- Pemanggilan IPC `agents.list` dan `mention.send` dengan `projectId` (nama field JSON `projectId`, `spawned`, `spawnedSession` harus sama persis dengan sisi Go; periksa konsistensi di langkah wiring, overview 5 butir 6).
- Jangan mengedit file Go atau wiring apa pun.

## Selesai bila

- [ ] Tiga file di atas diubah sesuai langkah; tidak ada file lain di luar `frontend/src/modules/mention/` yang berubah (`git status` hanya menampilkan path itu untuk fitur ini).
- [ ] `cd frontend && npm run build` sukses; `npm run check` sukses bila skrip ada (guard utama; tanpa unit test).
- [ ] Mengetik `@` menampilkan sesi, lalu agent, lalu kategori ber-default tanpa duplikasi nama; `@pla` menyaring berdasarkan prefix.
- [ ] Item agent/kategori tanpa sesi menampilkan teks "agent baru"; model kosong tampil "default".
- [ ] `@planner halo` tidak ditolak validasi lokal bila `planner` agent/kategori ber-default valid; `@asal-asalan` tetap ditolak "Sesi @asal-asalan tidak ada di workspace ini".
- [ ] `sendMention` mengirim `projectId` (dari `activeProjectId`) hanya bila ada; respons `spawned` memunculkan feedback "Sesi baru @... dibuka ...".
- [ ] Galat backend ("Tidak ada project konteks...", "Gagal membuka sesi agent @...", galat kategori tanpa default) tampil apa adanya (maks 160 karakter) dan teks pesan tidak dikosongkan.
- [ ] Daftar agent diperbarui saat `agents.changed` tanpa reload; kegagalan `listAgents` tidak merusak daftar sesi.
- [ ] Tidak ada impor dari `modules/settings/agents` atau `modules/thread`; di luar Wails pesan "Hanya tersedia di aplikasi desktop" tetap.
