export const meta = {
  name: 'implement-feature',
  description: 'Implementasi satu tiket di git worktree: foundation -> fitur paralel -> wiring -> build guard -> commit -> squash merge ke branch utama -> hapus worktree dan branch',
  whenToUse: 'Dipanggil oleh /implement-feature dengan args {ticket, maxAgentParalel}',
  phases: [
    { title: 'Check plan', detail: 'pastikan plan ada, jika tidak keluar' },
    { title: 'Setup', detail: 'git worktree + branch feat/<tiket>, salin plan' },
    { title: 'Foundation', detail: 'kerja bersama sebelum paralel (bila ada)' },
    { title: 'Features', detail: 'satu agent per plan fitur, paralel terbatas' },
    { title: 'Fitur lanjutan', detail: 'fitur yang bergantung pada fitur lain (mis. wiring), berurutan setelah antrean paralel' },
    { title: 'Wiring', detail: 'sambungkan semua fitur' },
    { title: 'Build guard', detail: 'build harus lulus, perbaiki maks 2x' },
    { title: 'Report', detail: 'tulis progress.md + commit di branch fitur' },
    { title: 'Merge', detail: 'squash merge ke branch utama, hapus worktree + branch (hanya bila OK)' },
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
    features: { type: 'array', items: { type: 'string' }, description: 'slug fitur = nama file .md di folder plan selain overview.md, 00-foundation.md, progress.md (SEMUA, termasuk yang manual)' },
    manualLast: { type: 'array', items: { type: 'string' }, description: 'subset features yang menurut overview.md dikerjakan MANUAL/TERAKHIR setelah wiring dan butuh daemon/app berjalan (mis. pengukuran/spike). Kosong bila tidak ada.' },
    afterParallel: { type: 'array', items: { type: 'string' }, description: 'subset features (BUKAN manual) yang bergantung pada hasil fitur lain dan HARUS dikerjakan setelah fitur-fitur itu selesai, mis. fitur wiring/penyambung yang menurut overview "dikerjakan setelah semua fitur lain tergabung" atau berurutan. Kosong bila semua fitur mandiri.' },
  },
  required: ['hasOverview', 'hasFoundation', 'features', 'manualLast', 'afterParallel'],
}
const check = await agent(
  `Lihat folder ${planDir}/ (di root repo). Laporkan: apakah overview.md ada, apakah 00-foundation.md ada, dan daftar slug fitur (nama file .md lain tanpa ekstensi, kecuali progress.md).
Baca juga overview.md untuk mengisi manualLast: slug fitur yang di overview dinyatakan dikerjakan terakhir/manual setelah Wiring, tidak paralel, dan butuh daemon atau aplikasi yang sedang berjalan untuk diukur/dicoba (contoh: spike pengukuran RAM/CPU). Fitur kode biasa BUKAN manualLast.
Isi juga afterParallel: slug fitur (yang bukan manual) yang di overview/plan-nya jelas bergantung pada fitur lain dan harus dikerjakan SETELAH fitur-fitur itu selesai, misalnya fitur wiring/penyambung ("dikerjakan setelah semua fitur lain tergabung", "berurutan", "menyambungkan hasil fitur X, Y, Z"), atau fitur yang membaca/mengimpor kode yang dibuat fitur lain. Fitur yang mandiri (hanya menyentuh path miliknya) BUKAN afterParallel.
Hanya baca; jangan mengubah apa pun.`,
  { label: 'check-plan', schema: CHECK_SCHEMA, model: 'haiku', effort: 'low' }
)
if (!check || !check.hasOverview || !check.features.length) {
  return { status: 'NO_PLAN', reason: `Plan untuk ${ticket} belum ada di ${planDir}/ — jalankan /plan-from-tiket ${ticket} dulu` }
}

const deferred = check.features.filter(s => check.manualLast.includes(s))
const afterParallel = check.features.filter(s => !deferred.includes(s) && (check.afterParallel || []).includes(s))
const features = check.features.filter(s => !deferred.includes(s) && !afterParallel.includes(s))
if (!features.length && !afterParallel.length) {
  return { status: 'NO_PLAN', reason: `Semua fitur di ${planDir}/ bersifat manual (${deferred.join(', ')}); tidak ada yang bisa diimplementasikan otomatis` }
}
const limit = rawMax === 'full' ? Math.max(1, features.length) : Math.max(1, parseInt(rawMax, 10) || 3)
log(`${features.length} fitur paralel (maks ${limit})${afterParallel.length ? `, lalu berurutan: ${afterParallel.join(', ')}` : ''}${check.hasFoundation ? ', ada foundation' : ''}${deferred.length ? `; ditunda (manual): ${deferred.join(', ')}` : ''}`)

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
function featurePrompt(slug, retry, sequential) {
  const rules = sequential
    ? `Fitur ini berjalan SETELAH semua fitur paralel lain selesai dan sudah ada di worktree (tidak ada agent lain yang bekerja sekarang).
- Baca kode fitur-fitur yang kamu sambungkan; jangan menulis ulang atau mengubahnya di luar yang diizinkan plan-mu.
- Kamu boleh menyentuh path yang disebut di plan-mu, termasuk file wiring/entry point milik plan-mu (mis. registrasi, App.svelte) bila plan memintanya.
- Jalankan build/check untuk bagian yang kamu sentuh; build penuh boleh dijalankan.
- Guard hanya build; tidak ada unit test. Jangan melakukan git commit (commit dilakukan sekali di akhir).`
    : `Aturan paralel (WAJIB): agent lain sedang mengerjakan fitur lain di worktree yang sama saat ini.
- Hanya sentuh file/folder di bagian "Path yang dimiliki" plan-mu.
- JANGAN menyentuh file wiring (registrasi, entry point, rute) atau path fitur lain. Langkah wiring terpisah menyambungkan semuanya di akhir.
- Jangan menjalankan build seluruh project (fitur lain bisa belum selesai dan membuat build gagal karena bukan salahmu). Build/vet hanya paket/folder milikmu.
- Guard hanya build; tidak ada unit test. Jangan melakukan git commit (commit dilakukan sekali di akhir).`
  return `Implementasikan SATU fitur dari tiket ${ticket}: ${slug}.
${WORKTREE_NOTE}
Baca penuh plan-nya: ${P}/${slug}.md. Baca juga ${P}/overview.md untuk kontrak bersama (jangan diubah).${check.hasFoundation ? ` Foundation sudah selesai dikerjakan (${P}/00-foundation.md); pakai apa yang sudah ada, jangan menulis ulang.` : ''}

${rules}
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
const parallelDone = featureResults.filter(Boolean)

// ---------- Fitur lanjutan (bergantung pada fitur lain, berurutan) ----------
const sequentialDone = []
if (afterParallel.length) {
  phase('Fitur lanjutan')
  const parallelFailed = parallelDone.filter(x => !x.ok).map(x => x.slug)
  for (const slug of afterParallel) {
    if (parallelFailed.length) {
      // Prasyarat gagal: jangan membuat stub; catat dan lewati.
      log(`${slug}: dilewati karena fitur prasyarat gagal (${parallelFailed.join(', ')})`)
      sequentialDone.push({ slug, ok: false, filesChanged: [], reason: `dilewati: fitur paralel gagal (${parallelFailed.join(', ')})` })
      continue
    }
    let res = await agent(featurePrompt(slug, false, true), { label: `impl-seq:${slug}`, phase: 'Fitur lanjutan', schema: IMPL_RESULT, model: 'sonnet' })
    if ((!res || !res.ok) && !(res && res.needsHumanInput)) {
      log(`${slug}: percobaan pertama gagal, ulang sekali`)
      res = await agent(featurePrompt(slug, true, true), { label: `impl-seq-retry:${slug}`, phase: 'Fitur lanjutan', schema: IMPL_RESULT, model: 'sonnet' })
    }
    sequentialDone.push({ slug, ...(res || { ok: false, filesChanged: [], reason: 'agent mati/dilewati' }) })
  }
}
const done = parallelDone.concat(sequentialDone)
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
- langkah manual yang ditunda (tidak dikerjakan workflow, jalankan sendiri setelah build lulus dengan daemon/app berjalan): ${deferred.length ? deferred.join(', ') + ' (lihat plan masing-masing)' : 'tidak ada'}
- build guard terakhir: passed=${build ? build.passed : false}
Output build (terakhir):
${build ? build.output : '(tidak ada)'}
2. Ubah baris "status:" di frontmatter ${WT}/docs/tickets/${ticket}.md menjadi ${status === 'OK' && !deferred.length ? 'implemented' : 'in-progress'} (hanya baris itu).
3. ${canCommit
    ? `Commit SEMUA perubahan di branch ${BRANCH}, dari dalam worktree: \`git -C ${WT} add -A\` lalu \`git -C ${WT} commit\` dengan pesan "feat(${ticket}): implementasi tiket${failed.length ? ' (sebagian)' : ''}" dan baris terakhir pesan tepat: ${COAUTHOR}. Jangan push, jangan merge, jangan menyentuh branch lain. Laporkan hash commit.`
    : 'Build TIDAK lulus, jadi JANGAN melakukan commit; biarkan perubahan di worktree. committed=false.'}`,
  { label: 'report-and-commit', schema: REPORT_SCHEMA, model: 'haiku', effort: 'low' }
)

// ---------- Merge ----------
phase('Merge')
const MERGE_SCHEMA = {
  type: 'object',
  properties: {
    outcome: { type: 'string', enum: ['MERGED', 'SKIPPED', 'BLOCKED', 'CONFLICT', 'FAILED'] },
    reason: { type: 'string' },
    mergeCommit: { type: 'string' },
    conflicts: { type: 'array', items: { type: 'string' } },
    cleaned: { type: 'boolean', description: 'worktree dan branch fitur sudah dihapus' },
    postMergeBuildPassed: { type: 'boolean' },
    postMergeBuildOutput: { type: 'string', description: 'ekor output build di branch utama setelah merge' },
  },
  required: ['outcome'],
}
let merge = { outcome: 'SKIPPED', reason: '' }
if (status === 'OK' && report && report.committed) {
  merge = await agent(
    `Squash merge branch fitur ${BRANCH} ke branch utama (${setup.baseBranch}), lalu bersihkan. Jalankan perintah nyata, periksa tiap hasil, dan ikuti aturan keselamatan ini TANPA pengecualian:
- JANGAN pernah memakai --force, reset --hard, clean, atau checkout yang membuang perubahan. JANGAN push.
- JANGAN menyelesaikan konflik sendiri. Konflik = batalkan dan laporkan.
- Jangan mengubah file apa pun kecuali lewat merge itu sendiri.

Langkah:
1. Tentukan ROOT = checkout utama: \`dirname "$(git rev-parse --path-format=absolute --git-common-dir)"\`. Semua perintah git berikutnya memakai \`git -C "$ROOT" ...\`, bukan cwd (cwd-mu bisa berada di dalam worktree).
2. Pastikan di ROOT branch aktif = ${setup.baseBranch} (\`git -C "$ROOT" branch --show-current\`). Bila tidak: outcome=SKIPPED, reason jelas. Pastikan tidak ada perubahan pada file TERLACAK di ROOT (\`git -C "$ROOT" status --porcelain --untracked-files=no\` kosong). Bila ada: outcome=SKIPPED, reason berisi daftar file.
3. Cek tabrakan dengan file TAK-TERLACAK di ROOT: ambil \`git -C "$ROOT" diff --name-only ${setup.baseBranch}...${BRANCH}\`. Untuk tiap path yang ada di ROOT sebagai file tak-terlacak (git ls-files --error-unmatch gagal): bila isinya identik dengan versi di branch (\`git -C "$ROOT" show ${BRANCH}:<path> | cmp - "$ROOT/<path>"\`), hapus salinan tak-terlacak itu di ROOT (aman, akan dikembalikan oleh merge). Bila isinya berbeda: outcome=BLOCKED, sebutkan path-nya, jangan menghapus apa pun.
4. \`git -C "$ROOT" merge --squash ${BRANCH}\`. Bila gagal atau ada path berstatus konflik (\`git -C "$ROOT" diff --name-only --diff-filter=U\`): jalankan \`git -C "$ROOT" reset --merge\` untuk membatalkan, outcome=CONFLICT, conflicts=daftar path. Worktree dan branch dibiarkan utuh.
5. Bila sukses: \`git -C "$ROOT" commit\` dengan subjek "feat(${ticket}): implementasi tiket (squash)", body satu-dua kalimat ringkasan fitur, dan baris terakhir pesan tepat: ${COAUTHOR}. Catat hash sebagai mergeCommit.
6. Verifikasi build di ROOT setelah merge (cd "$ROOT"). ${BUILD_GUARD} Catat postMergeBuildPassed dan ekor outputnya (maks 3000 karakter). JANGAN membatalkan merge bila build gagal; cukup laporkan.
7. Bersihkan: \`git -C "$ROOT" worktree remove ${WT}\` (tanpa --force; bila gagal, jangan paksa, laporkan reason dan cleaned=false), lalu \`git -C "$ROOT" branch -D ${BRANCH}\` (-D wajib karena squash merge membuat branch tampak belum ter-merge). Set cleaned=true bila keduanya berhasil.
8. outcome=MERGED bila commit merge terbentuk (walau cleanup atau build bermasalah; jelaskan di reason).`,
    { label: 'squash-merge', schema: MERGE_SCHEMA, model: 'sonnet' }
  ) || { outcome: 'FAILED', reason: 'agent merge mati/dilewati' }
} else {
  merge = {
    outcome: 'SKIPPED',
    reason: status !== 'OK'
      ? `status ${status}: merge otomatis hanya untuk status OK; worktree dan branch dibiarkan untuk diperiksa`
      : 'commit di branch fitur tidak terbentuk',
  }
}
log(`merge: ${merge.outcome}${merge.reason ? ' — ' + merge.reason : ''}`)

return {
  status,
  ticket,
  worktree: WT,
  branch: BRANCH,
  baseBranch: setup.baseBranch,
  features: done.map(x => ({ slug: x.slug, ok: x.ok, reason: x.reason })),
  deferredManual: deferred,
  wiring: wiring ? wiring.ok : false,
  buildPassed: build ? build.passed : false,
  committed: report ? report.committed : false,
  commit: report ? report.commit : undefined,
  merge,
  progress: merge.outcome === 'MERGED' ? `${planDir}/progress.md` : `${P}/progress.md`,
}
