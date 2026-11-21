export const meta = {
  name: 'create-ticket-from-spec',
  description: 'Pecah spec menjadi tiket epic di docs/tickets/ yang bisa dikerjakan paralel',
  whenToUse: 'Dipanggil oleh /create-ticket-from-spec dengan args {specPath, specSlug}',
  phases: [
    { title: 'Decompose', detail: 'baca spec, pecah jadi epic + fitur paralel' },
    { title: 'Write tickets', detail: 'satu agent per epic menulis file tiket' },
    { title: 'Verify', detail: 'cek file ada, cek tumpang tindih path, tulis index' },
  ],
}

const NO_STOPPING_NOTE =
  'Jangan berhenti untuk bertanya soal proses. Hanya set needsHumanInput=true bila ada ambiguitas requirement yang sungguh tidak bisa diputuskan dari spec.'
const WRITE_FILE_NOTE =
  'Tulis file dengan Write tool (bukan heredoc/echo). Setelah menulis, baca ulang file itu sebelum melapor ok:true.'

const specPath = args && args.specPath
if (!specPath) return { status: 'ERROR', reason: 'specPath kosong' }
const base = specPath.split('/').pop().replace(/\.md$/, '')
const specSlug = (args && args.specSlug) || base.replace(/^\d{4}-\d{2}-\d{2}-/, '').replace(/-design$/, '')
const pad = n => String(n).padStart(2, '0')

// ---------- Phase 1: Decompose ----------
phase('Decompose')
const DECOMP_SCHEMA = {
  type: 'object',
  properties: {
    needsHumanInput: { type: 'boolean' },
    reason: { type: 'string' },
    sharedContracts: { type: 'string', description: 'Kontrak antar-epic (protokol IPC, skema data, antarmuka) yang harus disepakati SEBELUM epic dikerjakan paralel' },
    epics: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          slug: { type: 'string', description: 'kebab-case pendek' },
          title: { type: 'string' },
          summary: { type: 'string' },
          wave: { type: 'number', description: '1 = bisa mulai langsung; 2 = menunggu wave 1, dst.' },
          dependsOn: { type: 'array', items: { type: 'string' }, description: 'slug epic lain' },
          ownedPaths: { type: 'array', items: { type: 'string' }, description: 'folder/file yang HANYA disentuh epic ini' },
          provides: { type: 'array', items: { type: 'string' } },
          consumes: { type: 'array', items: { type: 'string' } },
          features: {
            type: 'array',
            items: {
              type: 'object',
              properties: {
                slug: { type: 'string' },
                title: { type: 'string' },
                description: { type: 'string', description: 'Cukup lengkap agar agent yang tidak membaca sisa spec bisa merencanakannya' },
                ownedPaths: { type: 'array', items: { type: 'string' } },
              },
              required: ['slug', 'title', 'description', 'ownedPaths'],
            },
          },
        },
        required: ['slug', 'title', 'summary', 'wave', 'dependsOn', 'ownedPaths', 'features'],
      },
    },
  },
  required: ['epics', 'sharedContracts'],
}

const decomp = await agent(
  `Baca spec di ${specPath} secara penuh (dan lihat isi project yang sudah ada bila ada).
Pecah pekerjaan di spec menjadi TIKET = EPIC/MODUL (bukan satu fitur per tiket). Tiap epic memuat beberapa FITUR.

PERIKSA DULU apakah repo sudah berisi aplikasi yang berjalan (mis. go.mod, cmd/, internal/, frontend/ dan folder docs/tickets/ berisi tiket sebelumnya). Bila SUDAH ada, tiket baru dibuat DI ATAS kode yang ada: aturan "Tiket 1 = base/scaffold", "Hello World", "spike pengukuran fase 0", dan "tab terminal minimal di tiket sesi pertama" di bawah TIDAK berlaku; tiket 1 langsung kemampuan pertama dari spec. Baca struktur kode nyata (paket, modul, registry, komponen) agar ownedPaths merujuk path yang sungguh ada atau yang akan dibuat secara konsisten dengan konvensi yang ada, dan agar tiket memperluas titik ekstensi yang sudah ada, bukan membuat ulang. Tiket lama di docs/tickets/ adalah catatan historis; jangan diubah dan jangan dijadikan dependensi kecuali benar-benar diperlukan.

Aturan pemecahan (IRISAN VERTIKAL YANG SELALU BISA JALAN):
- SETIAP tiket, begitu selesai, harus menghasilkan aplikasi yang BISA DI-BUILD dan BISA DIJALANKAN (end-to-end, daemon + UI bila relevan), di atas hasil tiket-tiket sebelumnya. Tidak boleh ada tiket yang hasilnya baru berguna setelah tiket lain (tidak ada tiket "wiring di akhir").
- Tiket 1 = base: scaffold project + build berhasil + hello world/placeholder yang bisa dijalankan. Tiket 1 juga menyediakan TITIK EKSTENSI (mis. registri method daemon dan registri panel UI berbasis satu file per modul) agar tiket berikutnya menambah fiturnya sendiri tanpa merombak file milik tiket lain.
- Tiket berikutnya = satu kemampuan yang menambah nilai nyata, tiap tiket menyambungkan dirinya sendiri sampai terlihat di aplikasi (mis. tiket 2 hanya soal session/PTY: bisa spawn satu terminal dari aplikasi; tiket 3 workspace/project; dst). Urutkan menurut nilai bagi pengguna dan sesuai spec, dengan MVP (fase 0 + 1a) lebih dulu dan baca/tulis file (1b) sesudahnya. Spike pengukuran RAM/CPU (fase 0) diwujudkan sebagai fitur TERAKHIR di tiket sesi pertama (2-3 PTY berjalan + laporan hasil ukur di docs/perf/), JANGAN dibuat tiket terpisah, supaya gerbang ini ketahuan sedini mungkin.
- Kemampuan tiap tiket harus TERLIHAT DI APLIKASI (UI), bukan hanya lewat CLI debug. Khusus tiket sesi pertama: sertakan satu tab terminal minimal di UI (xterm.js, satu sesi shell, input/output jalan) lewat titik ekstensi tiket 1. Tiket UI shell berikutnya menyempurnakan tata letak (sidebar, banyak tab), bukan membuat terminal dari nol.
- wave = urutan eksekusi. Secara default tiket berurutan (wave 1, 2, 3, ...), karena tiap tiket membangun di atas yang sebelumnya. Dua tiket boleh satu wave HANYA bila tidak menyentuh file yang sama dan sama-sama hanya bergantung pada tiket sebelum wave itu.
- PARALELISME ada DI DALAM tiket: fitur-fitur dalam satu tiket (mis. bagian daemon vs bagian UI, atau komponen terpisah) harus bisa dikerjakan paralel oleh agent berbeda. Tiap tiket dan fitur punya ownedPaths yang tidak tumpang tindih dengan fitur lain DALAM tiket yang sama; tiket yang lebih baru boleh memperluas file milik tiket sebelumnya hanya lewat titik ekstensi, dan harus menyebutnya eksplisit di "Wiring".
- Hal bersama dalam satu tiket (kontrak, tipe) dikerjakan lebih dulu di tiket itu (foundation tiket tsb), baru fitur paralel, lalu wiring tiket tsb. Tuliskan kontrak antar-tiket secara konkret di sharedContracts.
- Jangan memecah secara buatan dan jangan menggabung hal yang berbeda. Nama epic = kemampuan/modul (mis. base, session-pty, workspace, providers, persistence, file-browser), bukan kalimat panjang.
- Cakupan: hanya yang dinyatakan spec untuk versi awal; fase yang ditandai belakangan/ditunda tidak dibuat tiketnya kecuali spec memintanya. Tanpa unit test; guard hanya build.

${NO_STOPPING_NOTE}`,
  { label: 'decompose-spec', schema: DECOMP_SCHEMA }
)

if (!decomp || decomp.needsHumanInput || !decomp.epics || !decomp.epics.length) {
  return { status: 'NEEDS_HUMAN_INPUT', reason: (decomp && decomp.reason) || 'dekomposisi kosong' }
}

const epics = decomp.epics.map((e, i) => ({
  ...e,
  ticketName: `${specSlug}-ticket-${pad(i + 1)}-${e.slug}`,
}))
const nameOf = {}
epics.forEach(e => { nameOf[e.slug] = e.ticketName })
epics.forEach(e => { e.dependsOnTickets = (e.dependsOn || []).map(s => nameOf[s] || s) })
log(`${epics.length} epic: ${epics.map(e => e.ticketName).join(', ')}`)

// ---------- Phase 2: Write tickets ----------
phase('Write tickets')
const OVERVIEW = epics
  .map(e => `- ${e.ticketName} (wave ${e.wave}): ${e.title} — owns ${e.ownedPaths.join(', ')}`)
  .join('\n')

function ticketPrompt(e) {
  return `Tulis file tiket untuk SATU epic: docs/tickets/${e.ticketName}.md
Spec sumber: ${specPath} (baca penuh). Tulis dalam bahasa Indonesia; identifier kode dalam bahasa Inggris.

Epic ini (hasil dekomposisi, ikuti, boleh diperkaya detailnya):
${JSON.stringify(e, null, 2)}

Epic lain dalam spec ini (batas kepemilikan, jangan disentuh):
${OVERVIEW}

Kontrak bersama antar-epic:
${decomp.sharedContracts}

Format file (wajib):
---
id: ${e.ticketName}
title: ${e.title}
spec: ${specPath}
status: ready
wave: ${e.wave}
depends_on: [${e.dependsOnTickets.join(', ')}]
---
# ${e.title}
## Tujuan  (apa dan kenapa epic ini ada, 2-4 kalimat)
## Hasil yang bisa dijalankan  (apa yang bisa dilihat/dilakukan pengguna di aplikasi setelah tiket ini selesai, di atas tiket sebelumnya; build harus lulus dan aplikasi harus jalan)
## Batasan  (in scope / out of scope)
## Kontrak  (apa yang epic ini sediakan dan pakai: antarmuka, pesan IPC, tipe; konkret)
## Fitur  (untuk TIAP fitur, sub-heading "### <slug>: <judul>" berisi: deskripsi detail, acceptance criteria berupa daftar periksa, ownedPaths, dan catatan bahwa fitur ini bisa dikerjakan paralel dengan fitur lain di epic ini kecuali disebut sebaliknya)
## Path yang dimiliki  (daftar; hanya ini yang boleh disentuh)
## Wiring  (langkah penutup TIKET INI: menyambungkan fitur-fiturnya ke aplikasi lewat titik ekstensi tiket 1 sehingga hasilnya terlihat dan jalan; tiket ini tidak boleh bergantung pada tiket lain untuk bisa dijalankan)
## Guard  (cukup build berhasil: go build ./... dan/atau build frontend; tanpa unit test)

Detail harus cukup agar agent yang hanya membaca tiket + spec bisa membuat plan implementasi tanpa bertanya.
${NO_STOPPING_NOTE}
${WRITE_FILE_NOTE}`
}

const TICKET_RESULT = {
  type: 'object',
  properties: { ok: { type: 'boolean' }, path: { type: 'string' }, needsHumanInput: { type: 'boolean' }, reason: { type: 'string' } },
  required: ['ok'],
}

const written = await pipeline(
  epics,
  e => agent(ticketPrompt(e), { label: `ticket:${e.slug}`, phase: 'Write tickets', schema: TICKET_RESULT })
)

// ---------- Phase 3: Verify ----------
phase('Verify')
const expected = epics.map(e => `docs/tickets/${e.ticketName}.md`)
const VERIFY_SCHEMA = {
  type: 'object',
  properties: {
    missing: { type: 'array', items: { type: 'string' } },
    overlaps: { type: 'array', items: { type: 'string' }, description: 'deskripsi tumpang tindih ownedPaths antar tiket' },
  },
  required: ['missing', 'overlaps'],
}

async function verify() {
  return agent(
    `Cek file tiket berikut benar-benar ada di disk dan tidak kosong:
${expected.join('\n')}
Lalu baca semuanya dan cari tumpang tindih path di bagian "Path yang dimiliki" antar tiket (dua tiket memiliki path yang sama/saling mencakup).
Laporkan: missing (path yang tidak ada/kosong) dan overlaps (deskripsi singkat tiap tumpang tindih; kosong bila tidak ada).
Jangan memodifikasi tiket.`,
    { label: 'verify-tickets', schema: VERIFY_SCHEMA, model: 'haiku', effort: 'low' }
  )
}

let v = await verify()
if (v && v.missing.length) {
  log(`File hilang, coba tulis ulang: ${v.missing.join(', ')}`)
  const redo = epics.filter(e => v.missing.includes(`docs/tickets/${e.ticketName}.md`))
  await parallel(redo.map(e => () => agent(ticketPrompt(e), { label: `retry:${e.slug}`, phase: 'Verify', schema: TICKET_RESULT })))
  v = await verify()
}

const indexPath = `docs/tickets/${specSlug}-index.md`
await agent(
  `Tulis ${indexPath}: ringkasan semua tiket dari spec ${specPath}.
Isi: tabel (tiket, judul, wave, depends_on, jumlah fitur) dan urutan eksekusi per wave (wave yang sama bisa paralel), plus kontrak bersama berikut:
${decomp.sharedContracts}
Data tiket:
${JSON.stringify(epics.map(e => ({ ticket: e.ticketName, title: e.title, wave: e.wave, dependsOn: e.dependsOnTickets, features: e.features.map(f => f.slug) })), null, 2)}
${WRITE_FILE_NOTE}`,
  { label: 'write-index', phase: 'Verify', model: 'haiku', effort: 'low' }
)

if (v && v.overlaps.length) log(`PERINGATAN tumpang tindih path: ${v.overlaps.join(' | ')}`)

return {
  status: v && v.missing.length ? 'PARTIAL' : 'OK',
  specSlug,
  tickets: expected,
  index: indexPath,
  missing: v ? v.missing : expected,
  overlaps: v ? v.overlaps : [],
}
