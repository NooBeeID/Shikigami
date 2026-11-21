# Shikigami — Design (sebelumnya: AI Terminal, working title)

Status: disetujui pembuat (2026-11-21). Folder ini bukan git repo, jadi dokumen belum di-commit.
Tanda **[call]** = keputusan saya (bukan hasil kesepakatan), silakan koreksi.

## 1. Intent dan alasan

Aplikasi desktop untuk menjalankan banyak sesi AI agent (Claude Code, Codex, dll.) dalam
terminal, dikelompokkan per **workspace** yang bisa berisi beberapa project. Ini pengganti
Orca untuk pemakaian sehari-hari pembuatnya.

Alasan dibuat (kata pembuat tentang Orca):
1. **Berat.** Dua sesi Claude saja sudah terasa berat (Electron). Mesin: MacBook 16GB, swap hampir penuh.
2. **Sesi terikat project.** Pekerjaan nyata terdiri dari project core + beberapa SDK. Pindah
   project mengganti daftar sesi sehingga membingungkan.
3. **Orca pada dasarnya editor.** Tidak ada dukungan knowledge default, optimasi biaya,
   setup hooks, atau penataan project untuk agent.

Temuan dari htop pembuat: beban berat tidak hanya dari Orca. Ikut menyumbang puluhan proses
`node statusline.mjs` (plugin statusline, di-spawn tiap refresh), `git status` dan `gh pr list`
yang terus polling, dan terminal daemon Orca dengan CPU time besar. Karena itu "ringan"
berarti juga **tanpa polling dan tanpa spawn proses per refresh**.

## 2. Goals

- Spawn banyak terminal dengan overhead sangat kecil per sesi.
- Jalankan Claude / Codex / agent lain di tiap terminal.
- **Workspace** berisi banyak project; daftar sesi milik workspace, bukan project.
- Sesi per project di dalam workspace; sesi bisa saling **mention** lintas project, lintas
  sesi, lintas provider.
- Knowledge otomatis yang dipahami semua agent, terintegrasi dengan **Grimoire**
  (hosted/self-hosted) atau knowledge internal.
- Setup project otomatis dan rapi: rules, code map, context index, cache context,
  system prompt, hooks internal.
- **Baca dan tulis file:** daftar project (tree) per workspace, buka file, dan simpan
  perubahan. `.txt` dan `.md` dibuka dengan **markdown view otomatis**; semua jenis lain
  (`.go`, `.js`, dll.) dibuka sebagai teks biasa untuk saat ini. File biner ditolak.
- Jalan di **macOS dulu**. Windows menyusul setelah Mac stabil.

## 3. Non-goals (versi awal)

- Editor hanya penyunting teks polos: tanpa syntax highlighting, autocomplete, atau LSP **[call]**.
- Tidak ada pencarian isi file atau fitur IDE lain (go-to-definition, dsb.).
- **Windows ditunda** (keputusan pembuat); desain tetap menjaga batas IPC dan PTY agar port nanti mudah.
- Tidak membuat agent sendiri; hanya menjalankan dan mengoordinasikan agent yang ada.
- Tidak ada sinkronisasi cloud atau multi-user.
- Linux belum ditarget (Wails mendukungnya, tapi tidak diuji).

### Cara kerja baca file **[call]**

- Tree project dimuat malas (per folder saat dibuka), tidak memindai seluruh project, dan
  mengabaikan `node_modules`, `.git`, `target`, dll. secara default.
- Tidak ada pemantauan filesystem terus-menerus; tree dan file yang terbuka disegarkan saat
  dibuka ulang, atau lewat file-watch hanya untuk file yang sedang tampil.
- File dibaca dan ditulis oleh daemon lewat IPC. **Tidak ada batas ukuran file** (keputusan
  pembuat); file biner ditolak dengan pesan. Agar file besar tidak membengkakkan RAM, tampilan
  teks dirender malas (hanya baris yang terlihat) **[call]**.
- Penyimpanan eksplisit (Cmd+S), dengan penanda "belum disimpan" di tab. Karena agent juga
  menulis ke file yang sama, bila file berubah di disk saat tab terbuka, UI memberi peringatan
  dan menawarkan muat ulang daripada menimpa diam-diam **[call]**.
- Markdown dirender di UI dengan library ringan; teks lain ditampilkan polos.
- File dibuka sebagai tab di samping/bersama terminal dalam workspace yang sama; tombol
  "mention ke sesi" untuk menyalin referensi file ke sesi agent adalah ide lanjutan, bukan
  bagian versi awal.

## 4. Anti-goals (gagal walau fungsinya jalan)

- Terasa seberat Orca / Electron.
- Daftar sesi berubah ketika berpindah project.
- Ada polling (git, PR, statusline) yang diam-diam menghabiskan CPU.
- Disk besar: toolchain atau dependency yang menggembung (pembuat tidak kuat disk untuk Rust).

## 5. Constraints

- Bahasa inti **Go** (disk toolchain kecil; Rust ditolak karena `target/` sangat besar).
- UI **Wails** (WKWebView di macOS). **Bukan Electron.** Tidak bergantung tmux.
- Frontend ringan: Svelte atau vanilla **[call]** (Grimoire memakai Svelte + Vite, jadi sudah dikenal).
- Versi awal hanya macOS, jadi library PTY cukup `creack/pty`. Bila Windows nanti dikerjakan,
  ganti ke library yang mendukung ConPTY (mis. `aymanbagabas/go-pty`); karena itu akses PTY
  dibungkus satu antarmuka tipis **[call]**.

## 6. Approaches yang dipertimbangkan

| Pendekatan | Hasil |
|---|---|
| Electron | Ditolak: alasan utama pembuat meninggalkan Orca. |
| TUI di terminal yang ada | Ditolak: kurang enak untuk workspace multi-project (sidebar, status banyak sesi, notifikasi). |
| Go + Gio (render sendiri) | Ditolak: emulator terminal harus dibuat sendiri. |
| Swift native (Mac) + UI Windows terpisah | Ditunda: biaya dua codebase UI; bisa ditambah nanti karena UI dan daemon dipisah. |
| **Go daemon + Wails UI (dipilih)** | Satu UI untuk Mac dan Windows, tanpa Chromium bundel, daemon terpisah sehingga UI bisa diganti. |

**Dipilih oleh pembuat:** Go daemon + Wails.

## 7. Arsitektur

```
 Wails UI (Svelte + xterm.js) ──IPC lokal──┐
 (unix socket di macOS)                    ▼
                              ┌─────────────────────────┐
                              │ Daemon Go               │
                              │ - workspace & sesi      │
                              │ - PTY per sesi          │
                              │ - routing mention (MCP) │
                              │ - knowledge & hooks     │
                              └──────┬──────────────────┘
                                     ▼
                  claude / codex / shell (proses di PTY)
```

- Daemon jalan terpisah dari UI; sesi tetap hidup walau window ditutup. UI menyalakan
  daemon bila belum ada **[call]**.
- Satu webview menampung semua terminal (tanpa proses baru per terminal).

### Konsep

- **Workspace**: nama + daftar project + konfigurasi.
- **Project**: path di disk, milik workspace.
- **Sesi**: id, workspace, project (working directory), provider, PTY, status.
- **Provider**: adapter tipis (perintah peluncur, cara mengirim input, cara memasang MCP/hooks):
  claude, codex, generic shell.

### Mention antar sesi

Dua jalur, keduanya ditargetkan:
1. **Agent → agent:** daemon menyediakan server MCP lokal yang dipasang ke tiap sesi
   dengan tool seperti `list_sessions`, `send_message(to, text)`,
   `read_output(to, last_n)`.
2. **User → sesi:** mengetik `@nama-sesi` di UI untuk meneruskan teks/output.

Risiko utama: menyuntik teks ke agent yang sedang sibuk. Usul **[call]**: pesan masuk
antrean dan dikirim saat sesi target idle; idle dideteksi dari hooks provider bila ada
(mis. event Stop/Notification di Claude Code, event-driven tanpa polling), dan dari
output yang diam sebagai cadangan. Perlu dibuktikan lewat prototipe.

### Status sesi dan biaya

- Status (berjalan / menunggu / selesai) berbasis event dari hooks, bukan polling.
- Informasi git hanya diambil saat diminta atau lewat file-watch untuk project yang
  sedang terlihat; tidak ada interval tetap.
- Target biaya **[call]**, harus diverifikasi: daemon idle < 30MB RSS, tambahan per sesi
  < ~10MB di luar proses agent, UI < 200MB dengan 6 sesi, CPU idle ≈ 0%.

## 8. Knowledge dan project setup (fase belakang)

- **Knowledge:** Grimoire adalah backend utama. Grimoire memakai Go + Postgres, punya gateway
  MCP read-only (`POST /mcp`, tool `list_files`, `read_file`, `search_knowledge`,
  `list_api_specs`, `list_api_endpoints`, `get_api_endpoint`) dengan API key. Aplikasi
  memasang konfigurasi MCP itu ke semua provider otomatis. Knowledge internal sederhana
  (direktori markdown lokal yang diekspos lewat MCP yang sama) sebagai cadangan bila
  tanpa Grimoire **[call]**.
- **Penataan project:** perintah init per project menghasilkan rules, code map, indeks
  konteks, cache konteks, system prompt, dan hooks internal dari **satu sumber**, lalu
  menerjemahkannya ke format tiap provider (mis. `CLAUDE.md`, `AGENTS.md`) agar semua
  agent mengerti hal yang sama **[call]**.

## 9. Fase pengerjaan **[call]**

0. **Spike pengukuran:** daemon + Wails + xterm.js dengan 2–3 PTY di Mac; ukur RAM/CPU
   dibanding Orca. Bila terlalu berat, pertimbangkan UI Swift native untuk Mac.
1. **Inti (macOS saja)**, dibagi dua agar MVP bisa dipakai lebih cepat:
   - **1a. MVP (target pertama: bisa spawn sesi dari aplikasi baru).** Urutan: daemon + PTY +
     sesi; workspace/project + UI sidebar dan tab terminal; provider claude/codex/shell;
     persistensi sesi (tetap hidup saat window ditutup, scrollback terbatas).
     Fase 0 + 1a = **MVP**; setelah ini aplikasi sudah dipakai untuk kerja sehari-hari.
   - **1b. Baca/tulis file**, menyusul setelah 1a: tree project + viewer/editor (txt/md
     dengan markdown view, lainnya teks). Tidak menghalangi MVP.
   Windows ditunda.
2. **Mention antar sesi** (MCP lokal, antrean, deteksi idle).
3. **Status dari hooks** dan notifikasi.
4. **Knowledge:** integrasi Grimoire + fallback internal.
5. **Penataan project:** init, rules, code map, context index/cache, hooks internal.

## 9b. Keputusan tambahan setelah review builder (semuanya **[call]**)

- **Spike (fase 0) adalah gerbang.** Fase 1–5 baru dikomit bila hasil ukur mendekati target
  di §7. Bila jauh meleset, UI Mac diganti Swift native + SwiftTerm, daemon Go tidak berubah.
  UI tetap di balik batas IPC sejak hari pertama, tanpa investasi ekstra di luar itu.
- **Urutan fase 2 dan 3 ditukar:** status dari hooks dibangun lebih dulu karena deteksi idle
  untuk mention bergantung padanya.
- **Fallback mention:** bila injeksi ke agent sibuk tidak andal, versi minimum adalah
  mention dari user saja, dikirim setelah konfirmasi atau saat sesi idle.
- **Provider memakai model kapabilitas:** tiap adapter menyatakan apa yang didukung (hooks,
  MCP, idle event). Provider tanpa hooks (Codex bila belum ada, shell biasa) memakai deteksi
  output-diam. Kemampuan Codex diverifikasi di fase 1.
- **Persistensi:** sesi bertahan saat UI ditutup. Tidak wajib bertahan saat daemon restart
  atau reboot di versi awal. UI yang tersambung kembali menerima buffer scrollback terbatas.
- **Grimoire** diperlakukan sebagai layanan eksternal (hanya lewat tool MCP-nya). URL dan
  API key disimpan per workspace dengan default global.
- **Fase 5** belum didefinisikan rinci; akan dibrainstorm terpisah setelah fase 1–4 jalan.

## 10. Diserahkan ke builder

Framework frontend final, protokol IPC (mis. gRPC/JSON-RPC), format konfigurasi dan
penyimpanan state, pemilihan library PTY, cara memasang MCP/hooks per provider, nama produk,
detail UI (tata letak sidebar workspace, tab sesi).
