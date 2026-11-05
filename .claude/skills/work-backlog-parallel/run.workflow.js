export const meta = {
  name: 'work-backlog-parallel',
  description: 'Build and review a /work-backlog-parallel batch concurrently, then merge it one PR at a time',
  phases: [
    { title: 'Build', detail: 'one isolated agent per issue, dependencies first' },
    { title: 'Review', detail: 'read-only review, two rounds at most' },
    { title: 'Fix', detail: 'isolated agent per review round with findings' },
    { title: 'Merge', detail: 'serial: rebase, gate, CI, merge' },
  ],
}

// args: {date, main, cap, batch: [{n, kind, after: [m], spec}]}; see SKILL.md "The run".
// Dates come from args because Date and Math.random throw in a workflow script (they would break resume).
const A = args
const SKILL = `${A.main}/.claude/skills/work-backlog-parallel`

const str = { type: 'string' }
const strs = { type: 'array', items: str }
const FINDING = {
  type: 'object',
  properties: { symptom: str, file: str, trigger: str, passesBar: str, foundWhile: str },
  required: ['symptom', 'file', 'trigger', 'passesBar', 'foundWhile'],
}
const findings = { type: 'array', items: FINDING }
const WRITER = {
  type: 'object',
  properties: {
    state: { type: 'string', enum: ['ready', 'parked', 'merged', 'skipped'] },
    branch: str, pr: str, plan: str, worktree: str,
    rulings: strs, findings, parkNote: str, batchHalt: str,
  },
  required: ['state', 'branch', 'pr', 'plan', 'worktree', 'rulings', 'findings', 'parkNote', 'batchHalt'],
}
const CHANGE = { type: 'object', properties: { file: str, summary: str }, required: ['file', 'summary'] }
const REVIEW = {
  type: 'object',
  properties: { changeFindings: { type: 'array', items: CHANGE }, otherFindings: findings },
  required: ['changeFindings', 'otherFindings'],
}
const MERGE = {
  type: 'object',
  properties: {
    state: { type: 'string', enum: ['merged', 'parked', 'skipped', 'needs-review'] },
    parkNote: str, batchHalt: str, mergeWorktree: str, conflictedFiles: strs, leftInPlace: strs,
  },
  required: ['state', 'parkNote', 'batchHalt', 'mergeWorktree', 'conflictedFiles', 'leftInPlace'],
}

const prompt = (file, section, inputs) =>
  `You are a /work-backlog-parallel workflow agent. Read ${SKILL}/${file} and follow "Every agent" and the ` +
  `section "${section}". Inputs (JSON): ${JSON.stringify(inputs)}`

// The cap bounds build, fix and merge agents together: each runs the full pre-push gate when it pushes.
let free = A.cap
const waiting = []
async function slot(fn) {
  if (free > 0) free--
  else await new Promise((resolve) => waiting.push(resolve))
  try {
    return await fn()
  } finally {
    const next = waiting.shift()
    if (next) next()
    else free++
  }
}

// One merge at a time; a resolution review holds the queue so main cannot move under it.
let mergeTail = Promise.resolve()
function serial(fn) {
  const run = mergeTail.then(fn)
  mergeTail = run.catch(() => null)
  return run
}

let batchHalt = ''
const records = {}
const settle = {}
const finished = {}
for (const item of A.batch) {
  finished[item.n] = new Promise((resolve) => { settle[item.n] = resolve })
}

function end(rec, state, note) {
  rec.state = state
  if (note) rec.note = note
  settle[rec.n](state)
  return rec
}

async function runIssue(item) {
  const n = item.n
  const rec = { n, state: 'building', note: '', branch: '', pr: '', worktrees: [], rulings: [], unrecorded: [] }
  records[n] = rec

  for (const m of item.after) {
    if (!finished[m]) continue
    const state = await finished[m]
    if (state !== 'merged') return end(rec, 'parked', `Depends on #${m}, which ended ${state}.`)
  }
  if (batchHalt) return end(rec, 'parked', `Batch halted: ${batchHalt}`)

  const build = await slot(() => agent(
    prompt('agents.md', 'Build', { n, kind: item.kind, spec: item.spec, date: A.date, main: A.main }),
    { label: `build #${n}`, phase: 'Build', schema: WRITER, isolation: 'worktree' }))
  if (!build) return end(rec, 'parked', 'The build agent died without a result.')
  rec.branch = build.branch
  rec.pr = build.pr
  if (build.worktree) rec.worktrees.push(build.worktree)
  rec.rulings.push(...build.rulings)
  rec.unrecorded = build.findings
  if (build.batchHalt) batchHalt = build.batchHalt
  if (build.state !== 'ready') return end(rec, build.state, build.parkNote)

  let last = build
  let head = build.worktree
  for (const round of [1, 2]) {
    const review = await agent(
      prompt('agents.md', 'Review', {
        n, round, branch: rec.branch, pr: rec.pr, headWorktree: head, plan: build.plan,
        worktrees: rec.worktrees, build: last, main: A.main,
      }),
      { label: `review #${n} r${round}`, phase: 'Review', schema: REVIEW })
    if (!review) return end(rec, 'parked', `The round ${round} review agent died without a result.`)
    rec.unrecorded = []
    if (!review.changeFindings.length) break
    const fix = await slot(() => agent(
      prompt('agents.md', 'Fix', {
        n, round, branch: rec.branch, pr: rec.pr, changeFindings: review.changeFindings, date: A.date, main: A.main,
      }),
      { label: `fix #${n} r${round}`, phase: 'Fix', schema: WRITER, isolation: 'worktree' }))
    if (!fix) return end(rec, 'parked', `The round ${round} fix agent died without a result.`)
    if (fix.worktree) rec.worktrees.push(fix.worktree)
    rec.rulings.push(...fix.rulings)
    rec.unrecorded = fix.findings
    if (fix.batchHalt) batchHalt = fix.batchHalt
    if (fix.state !== 'ready') return end(rec, fix.state, fix.parkNote)
    last = fix
    head = fix.worktree
  }

  const merged = await serial(async () => {
    if (batchHalt) return { state: 'parked', parkNote: `Batch halted: ${batchHalt}`, batchHalt: '' }
    const inputs = { n, branch: rec.branch, pr: rec.pr, worktrees: rec.worktrees, date: A.date, main: A.main }
    const first = await slot(() => agent(
      prompt('merge-queue.md', 'Merge queue', { ...inputs, resolutionReviewed: false, resolutionFindings: [] }),
      { label: `merge #${n}`, phase: 'Merge', schema: MERGE }))
    if (!first || first.state !== 'needs-review') return first
    const review = await agent(
      prompt('agents.md', 'Review', {
        n, resolution: true, mergeWorktree: first.mergeWorktree, conflictedFiles: first.conflictedFiles,
        branch: rec.branch, pr: rec.pr, main: A.main,
      }),
      { label: `review #${n} resolution`, phase: 'Review', schema: REVIEW })
    if (!review) return { state: 'parked', parkNote: 'The resolution review agent died without a result.', batchHalt: '' }
    rec.unrecorded.push(...review.otherFindings)
    return slot(() => agent(
      prompt('merge-queue.md', 'Merge queue', {
        ...inputs, resolutionReviewed: true, resolutionFindings: review.changeFindings,
      }),
      { label: `merge #${n} resolved`, phase: 'Merge', schema: MERGE }))
  })
  if (!merged) return end(rec, 'parked', 'The merge agent died without a result.')
  if (merged.batchHalt) batchHalt = merged.batchHalt
  if (merged.leftInPlace && merged.leftInPlace.length) rec.leftInPlace = merged.leftInPlace
  return end(rec, merged.state, merged.parkNote)
}

phase('Build')
log(`Batch of ${A.batch.length}, cap ${A.cap}`)
await parallel(A.batch.map((item) => () => runIssue(item).catch((error) => {
  const rec = records[item.n] || { n: item.n, worktrees: [], rulings: [], unrecorded: [] }
  records[item.n] = rec
  return end(rec, 'parked', `The script failed on this issue: ${error}`)
})))

const all = A.batch.map((item) => records[item.n])
return {
  merged: all.filter((r) => r.state === 'merged').map((r) => r.n),
  parked: all.filter((r) => r.state === 'parked').map((r) => ({ n: r.n, note: r.note })),
  skipped: all.filter((r) => r.state === 'skipped').map((r) => r.n),
  batchHalt: batchHalt || null,
  records: all,
}
