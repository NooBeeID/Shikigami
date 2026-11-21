export const meta = {
  name: 'implement-feature',
  description: 'Implementasi satu tiket di git worktree: foundation -> fitur paralel -> wiring -> build guard -> commit di branch fitur',
  whenToUse: 'Dipanggil oleh /implement-feature dengan args {ticket, maxAgentParalel}',
  phases: [
    { title: 'Check plan', detail: 'pastikan plan ada, jika tidak keluar' },
    { title: 'Setup', detail: 'git worktree + branch feat/<tiket>, salin plan' },
    { title: 'Foundation', detail: 'kerja bersama sebelum paralel (bila ada)' },
    { title: 'Features', detail: 'satu agent per plan fitur, paralel terbatas' },
    { title: 'Wiring', detail: 'sambungkan semua fitur' },
    { title: 'Build guard', detail: 'build harus lulus, perbaiki maks 2x' },
    { title: 'Report', detail: 'tulis progress.md + commit di branch fitur' },
  ],
}

const NO_STOPPING_NOTE =
  'Jangan berhenti untuk bertanya soal proses. Hanya set needsHumanInput=true bila ada ambiguitas requirement yang sungguh tidak bisa diputuskan dari plan. Build error bukan alasan needsHumanInput: perbaiki sendiri.'
const BUILD_GUARD =
  'Guard hanya build, TANPA unit test. Pakai perintah build di bagian "Build guard" overview.md bila ada; bila tidak: `make build` bila ada Makefile, atau `go build ./...` bila ada go.mod, dan `npm run build` di folder frontend bila ada package.json. Laporkan output mentah build, jangan diringkas.'
const COAUTHOR = 'Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>'

const ticket = args && args.ticket ? String(args.ticket).replace(/^.*\//, '').replace(/\.md$/, '') : ''
if (!ticket) return { status: 'ERROR', reason: 'ticket kosong' }
const planDir = `docs/plans/${ticket}`
const WT = `worktree/feat-${ticket}`
const BRANCH = `feat/${ticket}`
const P = `${WT}/${planDir}` // lokasi plan di dalam worktree
const rawMax = args && args.maxAgentParalel !== undefined ? args.maxAgentParalel : 3

const WORKTREE_NOTE = `LOKASI KERJA (WAJIB): semua pekerjaan dilakukan di dalam git worktree \`${WT}\` (branch ${BRANCH}), bukan di root repo. Semua path kode di plan/tiket relatif terhadap ROOT WORKTREE, jadi tulis dan ubah file sebagai \`${WT}/<path>\` dan jalankan perintah build dengan \`cd ${WT}\`. Jangan mengubah file di luar \`${WT}\`. Salinan plan ada di \`${P}/\`.`

// ---------- Check plan ----------
phase('Check plan')
const CHECK_SCHEMA = {
  type: 'object',
  properties: {
    hasOverview: { type: 'boolean' },
    hasFoundation: { type: 'boolean', description: `${planDir}/00-foundation.md ada` },
    features: { type: 'array', items: { type: 'string' }, description: 'slug fitur = nama file .md di folder plan selain overview.md, 00-foundation.md, progress.md' },
  },
  required: ['hasOverview', 'hasFoundation', 'features'],
}
const check = await agent(
  `Lihat folder ${planDir}/ (di root repo). Laporkan: apakah overview.md ada, apakah 00-foundation.md ada, dan daftar slug fitur (nama file .md lain tanpa ekstensi, kecuali progress.md). Hanya baca; jangan mengubah apa pun.`,
  { label: 'check-plan', schema: CHECK_SCHEMA, model: 'haiku', effort: 'low' }
)
if (!check || !check.hasOverview || !check.features.length) {
  return { status: 'NO_PLAN', reason: `Plan untuk ${ticket} belum ada di ${planDir}/ — jalankan /plan-from-tiket ${ticket} dulu` }
}

const features = check.features
const limit = rawMax === 'full' ? features.length : Math.max(1, parseInt(rawMax, 10) || 3)
log(`${features.length} fitur, paralel maks ${limit}${check.hasFoundation ? ', ada foundation' : ''}`)

// ---------- Setup worktree ----------
phase('Setup')
const SETUP_SCHEMA = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    reason: { type: 'string' },
    baseBranch: { type: 'string' },
    reused: { type: 'boolean', description: 'true bila worktree sudah ada dan dipakai ulang' },
  },
  required: ['ok'],
}
const setup = await agent(
  `Siapkan git worktree untuk tiket ${ticket}. Jalankan perintah nyata dan periksa hasilnya.
1. Pastikan repo git ada dan punya minimal satu commit (\`git rev-parse --is-inside-work-tree\` dan \`git rev-parse HEAD\`). Bila tidak, kembalikan ok=false, reason="NOT_GIT_REPO: jalankan git init dan commit awal dulu". Jangan menjalankan git init sendiri.
2. Catat branch saat ini sebagai baseBranch (\`git branch --show-current\`).
3. Pastikan "worktree/" ada di \`.git/info/exclude\` (tambahkan bila belum; jangan mengubah .gitignore).
4. Bila ${WT} sudah ada, pakai ulang (reused=true). Bila belum: \`git worktree add ${WT} -b ${BRANCH}\`; bila branch ${BRANCH} sudah ada, pakai \`git worktree add ${WT} ${BRANCH}\`.
5. Plan dan tiket belum tentu ter-commit, jadi salin ke worktree bila belum ada di sana: \`${planDir}\` -> \`${WT}/${planDir}\`, \`docs/tickets/${ticket}.md\` -> \`${WT}/docs/tickets/${ticket}.md\`, dan folder \`docs/superpowers/specs\` -> \`${WT}/docs/superpowers/specs\`. Gunakan \`cp -R\` tanpa menimpa file yang sudah ada.
Jangan mengubah file lain.`,
  { label: 'setup-worktree', schema: SETUP_SCHEMA, model: 'haiku', effort: 'low' }
)
if (!setup || !setup.ok) {
  return { status: 'SETUP_FAILED', reason: (setup && setup.reason) || 'setup worktree gagal' }
}
log(`worktree ${WT} (${BRANCH}) dari ${setup.baseBranch || '?'}${setup.reused ? ' [dipakai ulang]' : ''}`)

const IMPL_RESULT = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    filesChanged: { type: 'array', items: { type: 'string' } },
    needsHumanInput: { type: 'boolean' },
    reason: { type: 'string' },
  },
  required: ['ok', 'filesChanged'],
}

async function runWithLimit(items, worker, lim) {
  const results = new Array(items.length)
  let next = 0
  async function lane() {
    while (next < items.length) {
      const i = next++
      try { results[i] = await worker(items[i], i) } catch (e) { results[i] = null }
    }
  }
  await Promise.all(Array.from({ length: Math.min(lim, items.length) }, lane))
  return results
}

// ---------- Foundation ----------
let foundation = null
if (check.hasFoundation) {
  phase('Foundation')
  foundation = await agent(
    `Implementasikan foundation untuk tiket ${ticket}.
${WORKTREE_NOTE}
Baca penuh: ${P}/00-foundation.md dan ${P}/overview.md. Kerjakan persis sesuai plan, hanya menyentuh path yang disebut plan.
Agent fitur akan berjalan paralel SETELAH ini dan bergantung pada hasilmu, jadi kontrak (tipe, antarmuka, nama paket) harus sama persis dengan overview.
${BUILD_GUARD} Pastikan build lulus untuk bagian yang kamu buat. Jangan melakukan git commit.
${NO_STOPPING_NOTE}`,
    { label: 'foundation', schema: IMPL_RESULT, model: 'sonnet' }
  )
  if (!foundation || !foundation.ok) {
    return { status: 'FOUNDATION_FAILED', reason: (foundation && foundation.reason) || 'foundation gagal', foundation }
  }
}

// ---------- Features ----------
phase('Features')
function featurePrompt(slug, retry) {
  return `Implementasikan SATU fitur dari tiket ${ticket}: ${slug}.
${WORKTREE_NOTE}
Baca penuh plan-nya: ${P}/${slug}.md. Baca juga ${P}/overview.md untuk kontrak bersama (jangan diubah).${check.hasFoundation ? ` Foundation sudah selesai dikerjakan (${P}/00-foundation.md); pakai apa yang sudah ada, jangan menulis ulang.` : ''}

Aturan paralel (WAJIB): agent lain sedang mengerjakan fitur lain di worktree yang sama saat ini.
- Hanya sentuh file/folder di bagian "Path yang dimiliki" plan-mu.
- JANGAN menyentuh file wiring (registrasi, entry point, rute) atau path fitur lain. Langkah wiring terpisah menyambungkan semuanya di akhir.
- Jangan menjalankan build seluruh project (fitur lain bisa belum selesai dan membuat build gagal karena bukan salahmu). Build/vet hanya paket/folder milikmu.
- Guard hanya build; tidak ada unit test. Jangan melakukan git commit (commit dilakukan sekali di akhir).
${retry ? 'Ini percobaan ulang: periksa apa yang sudah ada dari percobaan sebelumnya, lalu lengkapi; jangan mulai dari nol.' : ''}
${NO_STOPPING_NOTE}`
}

const featureResults = await runWithLimit(
  features,
  async slug => {
    let res = await agent(featurePrompt(slug, false), { label: `impl:${slug}`, phase: 'Features', schema: IMPL_RESULT, model: 'sonnet' })
    if ((!res || !res.ok) && !(res && res.needsHumanInput)) {
      log(`${slug}: percobaan pertama gagal, ulang sekali`)
      res = await agent(featurePrompt(slug, true), { label: `impl-retry:${slug}`, phase: 'Features', schema: IMPL_RESULT, model: 'sonnet' })
    }
    return { slug, ...(res || { ok: false, filesChanged: [], reason: 'agent mati/dilewati' }) }
  },
  limit
)
const done = featureResults.filter(Boolean)
const failed = done.filter(x => !x.ok)
if (failed.length) log(`Fitur gagal: ${failed.map(x => `${x.slug} (${x.reason || '?'})`).join(', ')}`)

// ---------- Wiring ----------
phase('Wiring')
const wiring = await agent(
  `Langkah WIRING untuk tiket ${ticket}.
${WORKTREE_NOTE}
Baca ${P}/overview.md (bagian "Wiring checklist" dan "Build guard") dan bagian "Wiring" dari tiap plan fitur di ${P}/.
Fitur yang selesai: ${done.filter(x => x.ok).map(x => x.slug).join(', ') || '(tidak ada)'}.
Fitur yang GAGAL (jangan disambungkan; catat saja): ${failed.map(x => x.slug).join(', ') || '(tidak ada)'}.
Sambungkan semuanya sesuai checklist (registrasi, entry point, rute, binding). Setelah itu jalankan build penuh dan perbaiki sendiri bila gagal. Jangan melakukan git commit.
${BUILD_GUARD}
${NO_STOPPING_NOTE}`,
  { label: 'wiring', schema: IMPL_RESULT, model: 'sonnet' }
)

// ---------- Build guard ----------
phase('Build guard')
const BUILD_SCHEMA = {
  type: 'object',
  properties: {
    passed: { type: 'boolean' },
    output: { type: 'string', description: 'Output mentah build terakhir (dipotong ke 4000 karakter terakhir bila panjang)' },
    fixed: { type: 'string', description: 'Apa yang diperbaiki pada putaran ini (kosong bila tidak ada)' },
  },
  required: ['passed', 'output'],
}
let build = null
for (let attempt = 1; attempt <= 3; attempt++) {
  build = await agent(
    `Jalankan build guard penuh untuk tiket ${ticket} dan laporkan hasil NYATA (jangan mengira-ngira).
${WORKTREE_NOTE}
${BUILD_GUARD}
${attempt === 1 ? 'Bila gagal, JANGAN memperbaiki; hanya laporkan.' : 'Build sebelumnya gagal. Perbaiki penyebab di kode (hanya yang perlu), lalu jalankan build lagi dan laporkan hasilnya.'}
Output build sebelumnya:
${build ? build.output : '(belum ada)'}`,
    { label: `build-guard:${attempt}`, schema: BUILD_SCHEMA, model: 'sonnet' }
  )
  if (build && build.passed) break
  if (attempt === 1) log('Build gagal, mulai perbaikan (maks 2 putaran)')
}

// ---------- Report ----------
phase('Report')
const status = failed.length ? 'PARTIAL' : build && build.passed ? 'OK' : 'BUILD_FAILED'
const canCommit = !!(build && build.passed)
const REPORT_SCHEMA = {
  type: 'object',
  properties: { committed: { type: 'boolean' }, commit: { type: 'string' }, note: { type: 'string' } },
  required: ['committed'],
}
const report = await agent(
  `Selesaikan tiket ${ticket}. ${WORKTREE_NOTE}
1. Tulis ${P}/progress.md (Write tool), bahasa Indonesia, berisi ringkasan eksekusi:
- status: ${status}
- foundation: ${foundation ? (foundation.ok ? 'selesai' : 'gagal') : 'tidak ada'}
- hasil per fitur (JSON): ${JSON.stringify(done)}
- wiring: ${JSON.stringify(wiring)}
- build guard terakhir: passed=${build ? build.passed : false}
Output build (terakhir):
${build ? build.output : '(tidak ada)'}
2. Ubah baris "status:" di frontmatter ${WT}/docs/tickets/${ticket}.md menjadi ${status === 'OK' ? 'implemented' : 'in-progress'} (hanya baris itu).
3. ${canCommit
    ? `Commit SEMUA perubahan di branch ${BRANCH}, dari dalam worktree: \`git -C ${WT} add -A\` lalu \`git -C ${WT} commit\` dengan pesan "feat(${ticket}): implementasi tiket${failed.length ? ' (sebagian)' : ''}" dan baris terakhir pesan tepat: ${COAUTHOR}. Jangan push, jangan merge, jangan menyentuh branch lain. Laporkan hash commit.`
    : 'Build TIDAK lulus, jadi JANGAN melakukan commit; biarkan perubahan di worktree. committed=false.'}`,
  { label: 'report-and-commit', schema: REPORT_SCHEMA, model: 'haiku', effort: 'low' }
)

return {
  status,
  ticket,
  worktree: WT,
  branch: BRANCH,
  baseBranch: setup.baseBranch,
  features: done.map(x => ({ slug: x.slug, ok: x.ok, reason: x.reason })),
  wiring: wiring ? wiring.ok : false,
  buildPassed: build ? build.passed : false,
  committed: report ? report.committed : false,
  commit: report ? report.commit : undefined,
  progress: `${P}/progress.md`,
}
