# Grimoire (knowledge internal per project) — desain

Status: draf, menunggu persetujuan user (2026-11-22). Ukuran: proyek (desain penuh).

## Maksud
Setiap sesi agent sekarang mulai tanpa konteks proyek dan harus membaca ulang kode. User ingin Shikigami **meng-generate knowledge dari kode** (dinamai *grimoire*) lalu **menyuntikkannya ke system prompt** semua agent, sehingga Claude dan Codex berbicara dari pengetahuan yang sama dan memakai lebih sedikit token untuk eksplorasi.

Referensi: Lemongrass + bibliothek (`github.com/faizalv/lemongrass`, `github.com/faizalv/bibliothek`). Bibliothek hanyalah paket skill: agent sendiri yang menulis `biblio/` dengan disiplinnya, tanpa generator dari kode dan tanpa deteksi drift. Ide yang diambil: index satu baris per entri, buku = folder dan bab = file, isi hanya dibuka bila perlu, teks orientation identik di semua tab (prompt cache bersama).

## Tujuan
- Grimoire per project di-generate otomatis dari kode.
- Index grimoire disuntik ke system prompt Claude dan Codex dengan **teks byte-identik**.
- Halaman grimoire dibuka agent sesuai kebutuhan lewat pembacaan file biasa.
- Panel **Grimoire** di sidebar kiri: tombol Generate bila belum ada, daftar halaman bila sudah ada, klik membuka pane baru berisi halaman (lihat "UI").
- Halaman yang sumbernya berubah terdeteksi lewat hash dan diperbarui inkremental.
- `laws.md` buatan user ikut disuntik bersama index.

## Bukan tujuan
- Prosedur alur kerja agent (task close, handover, whiteboard) seperti Lemongrass.
- Server MCP grimoire (ditunda; opsi lanjutan bila index sudah terlalu besar).
- Integrasi layanan Grimoire eksternal (Go + Postgres; spec utama §8, `WorkspaceConfig.GrimoireURL/Key`). Itu jalur terpisah dan tidak diubah.
- Menghasilkan *laws* otomatis (laws bukan turunan kode).
- Berbagi grimoire antar anggota tim.
- Windows.

## Anti-tujuan
- Menyentuh repo proyek (tidak ada berkas yang ditulis di dalam repo).
- Mengubah prompt di tengah sesi (merusak prompt cache).
- Ringkasan yang tak bisa diverifikasi: setiap halaman wajib menyebut path sumbernya.
- Biaya generasi yang tidak terkendali: LLM hanya dipanggil untuk modul yang hash-nya berubah.

## Batasan
- Disimpan di data dir Shikigami: `<DataDir>/grimoire/<projectId>/` (keputusan user).
- Tanpa dependency baru bila bisa dihindari; memakai Go dan paket yang sudah ada (`internal/fsx`, `internal/paths`).
- Penyuntikan lewat `provider.RegisterLaunchContribution` / `LaunchContext.Args`, tanpa mengubah paket session.
- Prinsip ringan spec v2: tidak ada polling. Generasi hanya dipicu saat sesi baru dimulai atau lewat perintah eksplisit.

## Hasil verifikasi penyuntikan (2026-11-22, diuji langsung)
| Agent | Mekanisme | Hasil |
|---|---|---|
| Claude Code | `--append-system-prompt-file <berkas>` | Terbukti: token rahasia di berkas dijawab agent. Ada juga `--append-system-prompt <teks>`. |
| Codex (0.167.0) | `-c developer_instructions="<teks>"` (nilai berupa string TOML) | Terbukti: token rahasia dijawab agent. Berkas lewat `model_instructions_file` tidak dipakai: itu **mengganti** instruksi dasar, bukan menambah. |

Catatan: `developer_instructions` menempatkan teks sebagai pesan developer, bukan system prompt Claude. Teksnya tetap identik. Codex juga membaca `AGENTS.md` (batas `project_doc_max_bytes` 32768), tetapi itu menulis ke repo, jadi tidak dipakai.

## Pendekatan
**1. Siapa yang menulis isi grimoire.**
- Statik saja (pindai tree, package, simbol): murah, tapi tidak menjelaskan "mengapa".
- LLM saja: paling berguna, tapi mahal dan tanpa jangkar.
- **Gabungan (dipilih user):** kerangka statik (struktur modul, path, hash), lalu ringkasan LLM hanya untuk modul yang hash-nya berubah.

**2. Cara agent menerimanya.**
- **A. Berkas + index di prompt (dipilih user).** Halaman ditulis sebagai berkas; hanya index dan `laws.md` yang disuntik.
- B. Hanya MCP: agent bisa lupa memakainya dan knowledge tidak terjamin ada di konteks. Ditunda.
- C. A sekarang, B menyusul bila index membengkak. Ini jalur evolusi A.

## Desain

### Struktur penyimpanan
```
<DataDir>/grimoire/<projectId>/
  index.md            # disuntik; satu baris per halaman
  laws.md             # ditulis user; disuntik apa adanya
  state.json          # hash sumber per halaman, commit HEAD saat build, waktu build
  books/<nama_snake>/<bab_snake>.md
```

### Format `index.md`
Satu baris per buku, dengan nama bab dan tag satu token camelCase (lihat "Keputusan setelah builder check"):
```
nama_buku (bab1, bab2) [tag,camelTag]
nama_buku (bab1) [tag] warning: stale, jalankan grimoire build
```
Header singkat di atas berisi aturan pakai untuk agent (baca index, buka halaman yang relevan sebelum membaca source, path sumber tertera di tiap bab). Header ini konstan, agar prompt stabil.

### Format halaman
Markdown. Baris pertama bab mencantumkan `Sumber: <path relatif repo>, ...`. Isi snapshot fakta (tanpa tanggal, narasi, atau changelog); ditulis ulang utuh saat sumber berubah, tidak di-patch.

### Pipeline generasi (daemon Go)
1. **Pindai:** telusuri project (hormati `.gitignore`), kelompokkan menjadi modul (unit: package/direktori). Hitung hash isi per modul.
2. **Diff:** bandingkan dengan `state.json`. Modul baru atau hash berubah masuk antrean; modul yang hilang dihapus halamannya.
3. **Ringkas:** untuk tiap modul dalam antrean, jalankan agent headless (`claude -p`) dengan prompt tetap, berisi kerangka statik + berkas sumber modul. Keluaran ditulis sebagai bab.
4. **Index:** bangun ulang `index.md` dari daftar halaman; modul gagal diringkas diberi `warning: stale`.
5. **Simpan** `state.json`.

### Pemicu
- Saat sesi baru dibuat (via `LaunchContribution`): bila HEAD git berubah atau modul stale, jalankan build **sebelum** meluncurkan agent, lalu bekukan snapshot. Peluncuran tidak boleh menunggu LLM: bila build butuh ringkasan baru, sesi memakai snapshot terakhir dan build berjalan di latar belakang untuk sesi berikutnya.
- Manual: `aitermctl grimoire build [--project <id>]` dan `aitermctl grimoire status`.

### Penyuntikan
- Per sesi, daemon menyusun satu teks: header aturan + `index.md` + `laws.md`, menulisnya sebagai snapshot di `<DataDir>/run/grimoire-<hex>.md` (mode 0600, pola sama dengan `writeMCPConfig`).
- Claude: tambah `--append-system-prompt-file <snapshot>`.
- Codex: tambah `-c developer_instructions=<teks sebagai string TOML ter-escape>`.
- Teks identik untuk kedua agent dan tidak berubah selama sesi hidup.
- Provider `shell` tidak menerima suntikan.

### Pengukuran
`aitermctl grimoire status` menampilkan ukuran index (byte, estimasi token), jumlah halaman, jumlah stale, dan biaya build terakhir, supaya klaim "hemat token" bisa diverifikasi, bukan diasumsikan.

## UI (ditambahkan atas permintaan user)
Diminta user: menu/shortcut sendiri di sidebar kiri bertajuk **Grimoire**, panel sedikit lebih gelap dari background sidebar; tombol **Generate Grimoire** bila belum ada; bila sudah ada, daftar halaman; klik membuka **pane baru** berisi halaman itu.

Call saya (belum diperiksa user):
- Panel didaftarkan lewat `registerPanel({area:'sidebar'})` (pola panel Files), diletakkan di bawah daftar sesi, tanpa mengubah `AgentSidebar`.
- Buku tampil sebagai grup yang bisa dilipat, bab sebagai item. Halaman `stale` diberi penanda.
- Klik halaman membuka pane `file` read-only yang memakai penampil markdown yang ada; tidak ada jenis pane baru.
- Tombol Generate menjalankan build statik. Ringkasan LLM mengikuti flag `grimoire.llm` (opt-in); panel menampilkan progres dan estimasi dari `--dry-run`.
- Project aktif menentukan grimoire yang ditampilkan.

Sketsa:
```
┌ AGENT ───────────────┐
│ ● claude  api-fix    │
├ GRIMOIRE ────────────┤  <- lebih gelap
│ [ Generate Grimoire ]│  (belum ada)
│ session_pty          │  (sudah ada)
│   ├ manager          │
│   └ scrollback       │
│ provider (3)         │
└──────────────────────┘
```

## Keputusan setelah builder check
Keputusan user:
- **Biaya LLM: opt-in.** Flag `grimoire.llm` per project, default **mati**. Saat mati, hanya kerangka statik yang dibuat dan agent tetap mendapat index. `aitermctl grimoire status --dry-run` menampilkan estimasi modul dan token sebelum flag dinyalakan.
- **Tanpa batas modul per build secara default.** Opt-in dan dry-run sudah menjadi persetujuan yang cukup. Opsi `grimoire.maxModules` tersedia tetapi kosong (tanpa batas); modul di atas batas ditandai `stale` bila diisi. (Usul awal 20 modul dibuang atas pertanyaan user.)

Call saya (belum diperiksa user):
- **Agent bisa membaca data dir.** Provider selalu berjalan dengan `--dangerously-skip-permissions` / `--dangerously-bypass-approvals-and-sandbox`, jadi pembacaan di luar cwd tidak diblokir. Dibuktikan dari kode, bukan uji baca.
- **Isi index:** satu baris per buku: `nama_buku (bab1, bab2) [tag,camelTag]`. Tag dibuat statik dari nama folder/package, bukan LLM. Index punya batas ukuran; bila terlewati, buku yang paling jarang berubah dilipat jadi satu baris per kelompok. Ini menggantikan "tanpa deskripsi, nama bab menjelaskan isi" di bagian Format.
- **Project:** satu `Project.Path` = satu grimoire. Worktree dihitung project sendiri. Pemicu build adalah perubahan hash isi working tree (bukan HEAD), jadi edit yang belum di-commit terdeteksi. Folder tanpa git didukung.
- **Sesi pertama:** memakai index kerangka statik tanpa ringkasan LLM. Ringkasan menyusul di latar belakang untuk sesi berikutnya.
- **Tanpa `claude` terpasang/login** (mis. pengguna Codex saja): jatuh ke mode statik.

## Risiko yang diterima
- Penghematan token belum terukur; index menambah token per sesi.
- Ringkasan LLM bisa salah; mitigasi: path sumber di tiap bab.
- Snapshot bisa tertinggal dari kode selama satu sesi; dipilih demi prompt cache.
- Agent bisa mengabaikan index; tidak ada jaminan.
- Grimoire tidak dibagi ke tim.

## Yang diserahkan ke builder
- Algoritma pengelompokan modul (package Go, folder Svelte/TS, dll) dan batas ukuran per modul.
- Prompt ringkasan persis dan model yang dipakai (default saran: model murah).
- Estimasi token untuk `status`.
- Pengelolaan berkas snapshot lama di `run/`.
- Bentuk perintah CLI dan method IPC.
- Detail visual panel (warna persis, ikon, animasi progres) selama mengikuti token tema yang ada.
- Cara pane read-only membaca berkas dengan root grimoire (operasi baca file memakai parameter `Root` per project; grimoire ada di data dir, di luar project) tanpa melonggarkan akses tulis.
