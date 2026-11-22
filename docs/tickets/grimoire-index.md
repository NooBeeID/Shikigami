# Grimoire: index tiket

Sumber: `docs/superpowers/specs/2026-11-22-grimoire-design.md`

Grimoire adalah knowledge internal per project yang di-generate dari kode, disimpan di `<DataDir>/grimoire/<projectId>/`, lalu index-nya disuntikkan ke system prompt Claude dan Codex.

## Tabel tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|---|---|---|---|---|
| grimoire-ticket-01-grimoire-build | Grimoire: generate kerangka statik + panel sidebar | 1 | - | 6 |
| grimoire-ticket-02-grimoire-inject | Grimoire: suntik index + laws ke agent | 2 | grimoire-ticket-01-grimoire-build | 4 |
| grimoire-ticket-03-grimoire-refresh | Grimoire: deteksi perubahan, stale, dan build latar belakang | 3 | grimoire-ticket-02-grimoire-inject | 3 |
| grimoire-ticket-04-grimoire-llm | Grimoire: ringkasan LLM opt-in | 4 | grimoire-ticket-03-grimoire-refresh | 3 |

Total: 4 tiket, 16 fitur.

Fitur per tiket:
- **01**: contract-types, scan-modules, pages-index, ipc-ctl, panel-ui, readonly-pane
- **02**: compose-snapshot, launch-contribution, laws-measure-ipc, laws-ui
- **03**: diff-incremental, session-trigger, stale-ui
- **04**: settings-estimate, llm-summarizer, llm-ui

## Urutan eksekusi per wave

Rantai dependensi bersifat linear, sehingga tidak ada dua tiket yang bisa berjalan paralel. Setiap wave bergantung pada wave sebelumnya.

1. **Wave 1**: grimoire-ticket-01-grimoire-build
2. **Wave 2**: grimoire-ticket-02-grimoire-inject (setelah wave 1 selesai)
3. **Wave 3**: grimoire-ticket-03-grimoire-refresh (setelah wave 2 selesai)
4. **Wave 4**: grimoire-ticket-04-grimoire-llm (setelah wave 3 selesai)

Di dalam satu tiket, fitur boleh dikerjakan paralel selama tidak saling bergantung pada berkas yang sama.

## Kontrak bersama

### Konvensi
- CLI bernama **`shikigamictl`** (bukan `aitermctl` seperti di spec).
- Semua subcommand didaftarkan lewat `RegisterCommand` di `cmd/shikigamictl/cmd_grimoire.go`.
- Modul daemon didaftarkan lewat `RegisterModule` di `cmd/shikigamid/wire_grimoire*.go`. Nama modul unik per berkas, misalnya `grimoire`, `grimoire-inject`.
- Modul dipasang secara alfabetis. Layanan lain diambil lewat `d.OnReady` / `d.Lookup`, bukan dengan urutan pemasangan.
- Tiket 2-4 menambah modul wire sendiri dan fungsi `Register` tambahan di `internal/grimoire`. Mereka tidak mengedit `wire_grimoire.go` milik tiket 1, kecuali satu baris panggilan bila terpaksa (wajib disebut di bagian Wiring tiket).

### Penyimpanan
- Lokasi: `<DataDir>/grimoire/<projectId>/`
  - `index.md`: disuntik ke agent.
  - `laws.md`: ditulis user, disuntik apa adanya.
  - `state.json`: status build (tiket 1 dan 3).
  - `settings.json`: pengaturan (tiket 4).
  - `books/<nama_snake>/<bab_snake>.md`: halaman.
- Tidak ada berkas yang ditulis di dalam repo proyek.
- Satu `Project.Path` = satu grimoire. Worktree dihitung sebagai project sendiri.
- Nama buku dan bab memakai snake_case.

### Format bab
- Baris pertama: `Sumber: <path relatif repo>, ...`
- Isi: snapshot fakta, tanpa tanggal atau changelog. Ditulis ulang utuh saat sumber berubah, tidak di-patch.

### Format index.md
- Header aturan konstan, lalu satu baris per buku:
  - `nama_buku (bab1, bab2) [tag,camelTag]`
  - Bila stale: `nama_buku (bab1) [tag] warning: stale, jalankan grimoire build`
- Tag statik berasal dari nama folder atau package, satu token camelCase.
- Index memiliki batas ukuran. Bila terlampaui, buku yang paling jarang berubah dilipat menjadi satu baris per kelompok.

### state.json
```
{projectId, builtAt, modules: {<nama>: {hash, sources[], stale?: bool}}}
```
- `hash` = isi working tree per modul, bukan git HEAD. Edit yang belum di-commit ikut terdeteksi.

### IPC
Format JSON. Error mengikuti `internal/ipc/errors.go`.

| Method | Request | Response |
|---|---|---|
| `grimoire.build` | `{projectId, full?}` | - |
| `grimoire.status` | `{projectId}` | `{exists, books, pages, staleCount, building, indexBytes, estTokens, lawsBytes, llmEnabled, lastBuildCost}` |
| `grimoire.list` | `{projectId}` | `[{name, tags, chapters: [{name, path, stale}]}]` |
| `grimoire.read` | `{projectId, path}` | - |
| `grimoire.laws.get` / `grimoire.laws.set` | `{projectId, content}` | - |
| `grimoire.settings.get` / `grimoire.settings.set` | `{projectId, llm?, maxModules?}` | - |
| `grimoire.estimate` | `{projectId}` | `{modules, estTokens}` |

- Event: `grimoire.changed {projectId}`.
- Field baru hanya boleh ditambahkan secara aditif. Field yang sudah ada tidak boleh berubah artinya.

### Suntikan
- Satu teks tunggal berisi: header aturan konstan + `index.md` + `laws.md`. Teks ini byte-identik untuk Claude dan Codex.
- Teks dibekukan per sesi di `<DataDir>/run/grimoire-<hex>.md` (mode 0600).
- Claude: `--append-system-prompt-file <snapshot>`.
- Codex: `-c developer_instructions=<string TOML ter-escape>`. Bukan `model_instructions_file`, karena opsi itu mengganti instruksi dasar, bukan menambah.
- Provider `shell` tidak menerima suntikan.
- Disambungkan lewat `provider.RegisterLaunchContribution` dan `LaunchContext.Args`. Paket session tidak diubah, dan flag izin agent tidak digandakan.

### Frontend
- Satu modul `modules/grimoire` dengan `registerPanel({id: 'grimoire', area: 'sidebar'})`.
- Store per project aktif.
- Refresh hanya lewat event `grimoire.changed`. Tanpa `setInterval` dan tanpa polling.
- Fitur tiket 2-4 menambah subfolder sendiri (`laws/`, `stale/`, `llm/`) dan slot aditif di `GrimoirePanel.svelte`.
- Klik halaman membuka pane `file` read-only dengan leaf `{root: <direktori grimoire project>, readOnly: true}`. Akses tulis ke luar `Project.Path` tidak dilonggarkan.

### Guard dan ruang lingkup
- Tidak ada dependency baru.
- Tidak ada unit test. Guard hanya build: `go build ./...` dan `make build`.
- Di luar cakupan: integrasi grimoire eksternal (`GrimoireURL` / `Key`), MCP grimoire, laws otomatis, dan Windows.
