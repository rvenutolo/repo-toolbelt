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
  properties: { changeFindings: { type: 'array', items: CHANGE }, otherFindings: findings, batchHalt: str },
  required: ['changeFindings', 'otherFindings', 'batchHalt'],
}
const MERGE = {
  type: 'object',
  properties: {
    state: { type: 'string', enum: ['merged', 'parked', 'skipped', 'needs-review'] },
    parkNote: str, batchHalt: str, mergeWorktree: str, conflictedFiles: strs, leftInPlace: strs,
    rulings: strs, findings,
  },
  required: ['state', 'parkNote', 'batchHalt', 'mergeWorktree', 'conflictedFiles', 'leftInPlace', 'rulings',
    'findings'],
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

// A batch-level halt is recorded before the slot is released and checked once a slot is free, so an agent queued
// behind the cap never starts after it.
let batchHalt = ''
const HALTED = { halted: true }
const unlessHalted = (start) => {
  if (batchHalt) return HALTED
  return start().then((result) => {
    if (result && result.batchHalt && !batchHalt) batchHalt = result.batchHalt
    return result
  })
}
const haltNote = () => `Batch halted: ${batchHalt}`

const records = {}
const settle = {}
const finished = {}
for (const item of A.batch) {
  finished[item.n] = new Promise((resolve) => { settle[item.n] = resolve })
}

// An issue whose `after` chain leads back to itself would wait forever and hang the whole run.
const afterOf = {}
for (const item of A.batch) afterOf[item.n] = item.after
function inCycle(start) {
  const seen = new Set()
  const stack = [...(afterOf[start] || [])]
  while (stack.length) {
    const m = stack.pop()
    if (m === start) return true
    if (seen.has(m) || !afterOf[m]) continue
    seen.add(m)
    stack.push(...afterOf[m])
  }
  return false
}

const TERMINAL = ['merged', 'parked', 'skipped']
function end(rec, state, note) {
  if (TERMINAL.includes(state)) {
    rec.state = state
    if (note) rec.note = note
  } else {
    rec.state = 'parked'
    rec.note = `The issue stopped in state "${state}". ${note || ''}`.trim()
  }
  if (rec.state === 'skipped' && rec.worktrees.length) rec.leftInPlace = [...rec.worktrees]
  settle[rec.n](rec.state)
  return rec
}

async function mergeIssue(rec) {
  if (batchHalt) return { state: 'parked', parkNote: haltNote(), batchHalt: '' }
  const inputs = { n: rec.n, branch: rec.branch, pr: rec.pr, worktrees: rec.worktrees, date: A.date, main: A.main }
  let resolutionReviewed = false
  let resolutionFindings = []
  let mergeWorktree = ''
  // Parking here, between merge calls, leaves a rebased branch no agent pushed; the note must say where it is.
  const park = (note, halt = '') => ({
    state: 'parked', batchHalt: halt,
    parkNote: mergeWorktree
      ? `${note} The worktree ${mergeWorktree} holds an unpushed, rebased ${rec.branch} that diverges from ` +
        `origin/${rec.branch}.`
      : note,
  })
  // A code resolution goes to review and back; main can move during CI and force another, so three at most.
  for (const attempt of [1, 2, 3]) {
    const merge = await slot(() => unlessHalted(() => agent(
      prompt('merge-queue.md', 'Merge queue', { ...inputs, resolutionReviewed, resolutionFindings }),
      { label: `merge #${rec.n}${attempt > 1 ? ` a${attempt}` : ''}`, phase: 'Merge', schema: MERGE })))
    if (merge === HALTED) return park(haltNote())
    if (!merge) return park('The merge agent died without a result.')
    rec.rulings.push(...(merge.rulings || []))
    rec.unrecorded.push(...(merge.findings || []))
    if (merge.state !== 'needs-review') return merge
    mergeWorktree = merge.mergeWorktree
    // The review appends its out-of-scope findings to <n>-findings.md itself, so they are not unrecorded.
    const review = await agent(
      prompt('agents.md', 'Review', {
        n: rec.n, resolution: true, mergeWorktree, conflictedFiles: merge.conflictedFiles,
        branch: rec.branch, pr: rec.pr, main: A.main,
      }),
      { label: `review #${rec.n} resolution a${attempt}`, phase: 'Review', schema: REVIEW })
    if (!review) return park('The resolution review agent died without a result.')
    if (review.batchHalt) {
      rec.unrecorded.push(...review.otherFindings)
      return park(review.batchHalt, review.batchHalt)
    }
    resolutionReviewed = true
    resolutionFindings = review.changeFindings
  }
  return park('Three code resolutions in a row: main kept moving under this branch.')
}

async function runIssue(item) {
  const n = item.n
  const rec = { n, state: 'building', note: '', branch: '', pr: '', worktrees: [], rulings: [], unrecorded: [] }
  records[n] = rec

  if (inCycle(n)) return end(rec, 'parked', `Its "after" list forms a dependency cycle: ${item.after.join(', ')}.`)
  for (const m of item.after) {
    if (!finished[m]) continue
    const state = await finished[m]
    if (state !== 'merged') return end(rec, 'parked', `Depends on #${m}, which ended ${state}.`)
  }

  const build = await slot(() => unlessHalted(() => agent(
    prompt('agents.md', 'Build', { n, kind: item.kind, spec: item.spec, date: A.date, main: A.main }),
    { label: `build #${n}`, phase: 'Build', schema: WRITER, isolation: 'worktree' })))
  if (build === HALTED) return end(rec, 'parked', haltNote())
  if (!build) return end(rec, 'parked', 'The build agent died without a result.')
  rec.branch = build.branch
  rec.pr = build.pr
  rec.plan = build.plan
  if (build.worktree) rec.worktrees.push(build.worktree)
  rec.rulings.push(...build.rulings)
  rec.unrecorded = build.findings
  if (build.batchHalt) batchHalt = build.batchHalt
  if (build.state !== 'ready') return end(rec, build.state, build.parkNote)

  let last = build
  let head = build.worktree
  for (const round of [1, 2]) {
    if (batchHalt) return end(rec, 'parked', haltNote())
    const review = await agent(
      prompt('agents.md', 'Review', {
        n, round, branch: rec.branch, pr: rec.pr, headWorktree: head, plan: rec.plan,
        worktrees: rec.worktrees, build: last, main: A.main,
      }),
      { label: `review #${n} r${round}`, phase: 'Review', schema: REVIEW })
    if (!review) return end(rec, 'parked', `The round ${round} review agent died without a result.`)
    // A review stopped by a batch halt may not have recorded the writer's findings; keep them for the controller.
    if (review.batchHalt) {
      batchHalt = review.batchHalt
      rec.unrecorded.push(...review.otherFindings)
      return end(rec, 'parked', haltNote())
    }
    rec.unrecorded = []
    if (!review.changeFindings.length) break
    const fix = await slot(() => unlessHalted(() => agent(
      prompt('agents.md', 'Fix', {
        n, round, branch: rec.branch, pr: rec.pr, spec: item.spec, changeFindings: review.changeFindings,
        date: A.date, main: A.main,
      }),
      { label: `fix #${n} r${round}`, phase: 'Fix', schema: WRITER, isolation: 'worktree' })))
    if (fix === HALTED) return end(rec, 'parked', haltNote())
    if (!fix) return end(rec, 'parked', `The round ${round} fix agent died without a result.`)
    if (fix.worktree) rec.worktrees.push(fix.worktree)
    rec.rulings.push(...fix.rulings)
    rec.unrecorded = fix.findings
    if (fix.batchHalt) batchHalt = fix.batchHalt
    if (fix.state !== 'ready') return end(rec, fix.state, fix.parkNote)
    last = fix
    head = fix.worktree
  }

  const merged = await serial(() => mergeIssue(rec))
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
