export const meta = {
  name: 'plan-from-tiket',
  description: 'Buat implementation plan dari satu tiket epic: overview.md + satu plan per fitur',
  whenToUse: 'Dipanggil oleh /plan-from-tiket dengan args {ticket, force}',
  phases: [
    { title: 'Resolve', detail: 'baca tiket, cek guard' },
    { title: 'Overview', detail: 'desain bersama + daftar fitur final' },
    { title: 'Feature plans', detail: 'satu agent per fitur, paralel' },
    { title: 'Verify', detail: 'cek file plan ada, retry sekali' },
  ],
}

const NO_STOPPING_NOTE =
  'Jangan berhenti untuk bertanya soal proses. Hanya set needsHumanInput=true bila ada ambiguitas requirement yang sungguh tidak bisa diputuskan dari tiket/spec.'
const WRITE_FILE_NOTE =
  'Tulis file dengan Write tool (bukan heredoc/echo). Setelah menulis, baca ulang file itu sebelum melapor ok:true.'

const ticket = args && args.ticket ? String(args.ticket).replace(/^.*\//, '').replace(/\.md$/, '') : ''
if (!ticket) return { status: 'ERROR', reason: 'ticket kosong' }
const planDir = `docs/plans/${ticket}`

// ---------- Resolve ----------
phase('Resolve')
const RESOLVE_SCHEMA = {
  type: 'object',
  properties: {
    ticketExists: { type: 'boolean' },
    alreadyPlanned: { type: 'boolean', description: `true bila ${planDir}/overview.md sudah ada` },
    specPath: { type: 'string' },
  },
  required: ['ticketExists', 'alreadyPlanned'],
}
const r = await agent(
  `Cek: apakah docs/tickets/${ticket}.md ada, dan apakah ${planDir}/overview.md sudah ada.
Bila tiket ada, ambil nilai "spec:" dari frontmatternya sebagai specPath. Hanya laporkan; jangan mengubah apa pun.`,
  { label: 'resolve-ticket', schema: RESOLVE_SCHEMA, model: 'haiku', effort: 'low' }
)
if (!r || !r.ticketExists) return { status: 'NOT_FOUND', reason: `docs/tickets/${ticket}.md tidak ada` }
if (r.alreadyPlanned && !(args && args.force)) {
  return { status: 'ALREADY_PLANNED', reason: `${planDir}/overview.md sudah ada; jalankan lagi dengan --force untuk menimpa` }
}

// ---------- Overview ----------
phase('Overview')
const OVERVIEW_SCHEMA = {
  type: 'object',
  properties: {
    needsHumanInput: { type: 'boolean' },
    reason: { type: 'string' },
    hasFoundation: { type: 'boolean', description: 'true bila ada kerja bersama yang harus selesai SEBELUM fitur paralel (ditulis di 00-foundation.md)' },
    features: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          slug: { type: 'string' },
          title: { type: 'string' },
          scope: { type: 'string', description: 'Ringkasan scope fitur ini, cukup lengkap untuk agent yang tidak membaca fitur lain' },
          ownedPaths: { type: 'array', items: { type: 'string' } },
        },
        required: ['slug', 'title', 'scope', 'ownedPaths'],
      },
    },
  },
  required: ['hasFoundation', 'features'],
}

const ov = await agent(
  `Baca tiket docs/tickets/${ticket}.md secara penuh, spec yang dirujuknya${r.specPath ? ` (${r.specPath})` : ''}, dan kode project yang sudah ada bila ada.
Tugasmu: tulis ${planDir}/overview.md — desain BERSAMA untuk epic ini. Plan per fitur akan ditulis agent lain secara paralel dan hanya membaca overview + tiket, jadi overview harus memuat semua yang dipakai bersama.

overview.md wajib memuat:
1. Ringkasan epic dan urutan eksekusi: Foundation (bila ada) -> fitur paralel -> Wiring.
2. Kontrak bersama: tipe, antarmuka, pesan IPC, nama paket/folder, konvensi penamaan.
3. Daftar fitur (slug, scope, ownedPaths) — turunkan dari bagian Fitur di tiket; jangan menambah/menggabung fitur kecuali tiket jelas keliru.
4. Foundation: hal yang harus ada sebelum fitur paralel (skeleton paket, tipe bersama). Bila ada, tulis juga ${planDir}/00-foundation.md berisi plan langkah-langkahnya (hasFoundation=true).
5. Wiring checklist: persis apa yang harus disambungkan setelah semua fitur selesai (registrasi, entry point, rute, binding) — agen wiring akan membaca ini.
6. Build guard: perintah build yang menjadi satu-satunya guard (mis. go build ./... , build frontend). Tanpa unit test.
7. Aturan paralel: tiap agent hanya menyentuh ownedPaths fiturnya; jangan menyentuh path fitur lain atau file wiring.

Tulis dalam bahasa Indonesia; identifier kode dalam bahasa Inggris.
${NO_STOPPING_NOTE}
${WRITE_FILE_NOTE}`,
  { label: 'write-overview', schema: OVERVIEW_SCHEMA }
)
if (!ov || ov.needsHumanInput || !ov.features || !ov.features.length) {
  return { status: 'NEEDS_HUMAN_INPUT', reason: (ov && ov.reason) || 'overview tidak menghasilkan fitur' }
}
log(`${ov.features.length} fitur: ${ov.features.map(f => f.slug).join(', ')}${ov.hasFoundation ? ' (+foundation)' : ''}`)

// ---------- Feature plans ----------
phase('Feature plans')
const FEATURE_RESULT = {
  type: 'object',
  properties: { ok: { type: 'boolean' }, path: { type: 'string' }, needsHumanInput: { type: 'boolean' }, reason: { type: 'string' } },
  required: ['ok'],
}
const FEATURE_LIST = ov.features.map(f => `- ${f.slug}: ${f.title} — owns ${f.ownedPaths.join(', ')}`).join('\n')

function featurePrompt(f) {
  return `Tulis implementation plan untuk SATU fitur: ${planDir}/${f.slug}.md
Baca dulu (jangan diubah): ${planDir}/overview.md, docs/tickets/${ticket}.md${r.specPath ? `, ${r.specPath}` : ''}${ov.hasFoundation ? `, ${planDir}/00-foundation.md` : ''}.

Fitur ini:
${JSON.stringify(f, null, 2)}

Fitur paralel lain (jangan disentuh; agent lain sedang mengerjakan):
${FEATURE_LIST}

Plan ini akan di-inject ke SATU agent implementasi yang tidak membaca plan lain, jadi harus mandiri. Bahasa Indonesia; identifier kode bahasa Inggris. Struktur wajib:
# Plan: ${f.title}
## Metadata  (tiket, fitur, ownedPaths, depends on foundation ya/tidak)
## Tujuan
## Path yang dimiliki  (HANYA ini yang boleh disentuh; jangan sentuh file wiring atau path fitur lain)
## Kontrak  (yang disediakan dan dipakai, merujuk overview; konkret: nama fungsi/tipe/pesan)
## Langkah  (berurutan, spesifik: file apa, isi apa, signature)
## Perubahan  (daftar file yang dibuat/diubah)
## Wiring  (apa yang fitur ini ekspos untuk disambungkan nanti oleh langkah wiring; JANGAN menyambungkannya sendiri)
## Selesai bila  (kriteria; guard = build path fitur ini sukses; tanpa unit test)

Jangan mengulang isi overview; rujuk saja. Jangan memodifikasi overview.md.
${NO_STOPPING_NOTE}
${WRITE_FILE_NOTE}`
}

await pipeline(
  ov.features,
  f => agent(featurePrompt(f), { label: `plan:${f.slug}`, phase: 'Feature plans', schema: FEATURE_RESULT })
)

// ---------- Verify ----------
phase('Verify')
const expected = ['overview.md']
  .concat(ov.hasFoundation ? ['00-foundation.md'] : [])
  .concat(ov.features.map(f => `${f.slug}.md`))
  .map(n => `${planDir}/${n}`)

async function missingFiles() {
  const res = await agent(
    `Cek file berikut ada di disk dan tidak kosong:
${expected.join('\n')}
Laporkan hanya path yang hilang/kosong (array kosong bila semua ada). Jangan mengubah file.`,
    { label: 'verify-plans', schema: { type: 'object', properties: { missing: { type: 'array', items: { type: 'string' } } }, required: ['missing'] }, model: 'haiku', effort: 'low' }
  )
  return res ? res.missing : expected
}

let missing = await missingFiles()
if (missing.length) {
  log(`File plan hilang, coba tulis ulang: ${missing.join(', ')}`)
  const redo = ov.features.filter(f => missing.includes(`${planDir}/${f.slug}.md`))
  await parallel(redo.map(f => () => agent(featurePrompt(f), { label: `retry:${f.slug}`, phase: 'Verify', schema: FEATURE_RESULT })))
  missing = await missingFiles()
}

return {
  status: missing.length ? 'PARTIAL' : 'OK',
  ticket,
  planDir,
  files: expected,
  missing,
}
