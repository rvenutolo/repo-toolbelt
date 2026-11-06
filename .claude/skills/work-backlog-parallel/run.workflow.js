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

// args: {date, main, cap, batch: [{n, kind, after: [m], spec, piece, haltAnswers}]}; see SKILL.md "The run".
// Dates come from args because Date and Math.random throw in a workflow script (they would break resume).
const A = args

// A malformed args file must fail before any agent starts: a missing cap would leave every slot waiting forever, a
// string issue number would slip past the cycle check, and a duplicate would build and merge one issue twice.
function checkArgs() {
  const bad = (why) => { throw new Error(`run-args.json is invalid: ${why}`) }
  if (!A || typeof A !== 'object' || Array.isArray(A)) bad('args must be an object')
  if (typeof A.date !== 'string' || !A.date) bad('date must be a non-empty string')
  if (typeof A.main !== 'string' || !A.main) bad('main must be a non-empty string')
  if (!Number.isInteger(A.cap) || A.cap < 1) bad('cap must be an integer of at least 1')
  if (!Array.isArray(A.batch) || !A.batch.length) bad('batch must be a non-empty list')
  const seen = new Set()
  for (const item of A.batch) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) bad('every batch entry must be an object')
    if (!Number.isInteger(item.n) || item.n < 1) bad(`issue number ${JSON.stringify(item.n)} is not a positive integer`)
    if (seen.has(item.n)) bad(`issue #${item.n} appears twice`)
    seen.add(item.n)
    if (item.kind !== 'bug' && item.kind !== 'feature') bad(`#${item.n} has kind ${JSON.stringify(item.kind)}`)
    if (typeof item.spec !== 'string') bad(`#${item.n} needs a spec string (empty for a bug)`)
    // A feature's build copies <main>/<spec> in; an empty spec would copy the whole main checkout's path.
    if (item.kind === 'feature' && !item.spec) bad(`#${item.n} is a feature with no spec`)
    if (!Array.isArray(item.after)) bad(`#${item.n} needs an "after" list (empty when it has no prerequisite)`)
    if (typeof item.piece !== 'string') bad(`#${item.n} needs a piece string (empty unless it is an umbrella piece)`)
    if (typeof item.haltAnswers !== 'string') bad(`#${item.n} needs a haltAnswers string (empty when none)`)
  }
  for (const item of A.batch) {
    for (const m of item.after) {
      if (!Number.isInteger(m)) bad(`#${item.n} lists ${JSON.stringify(m)} in "after", not an integer`)
      // planning.md keeps a dependent out of the batch when its prerequisite is outside it and unmerged.
      if (!seen.has(m)) bad(`#${item.n} depends on #${m}, which is not in the batch`)
    }
  }
}
checkArgs()
const SKILL = `${A.main}/.claude/skills/work-backlog-parallel`

const str = { type: 'string' }
const strs = { type: 'array', items: str }
const FINDING = {
  type: 'object',
  // whyReal is the agent's case that the finding is real; the controller, not the agent, applies the filing bar.
  properties: { symptom: str, file: str, trigger: str, whyReal: str, foundWhile: str },
  required: ['symptom', 'file', 'trigger', 'whyReal', 'foundWhile'],
}
const findings = { type: 'array', items: FINDING }
const WRITER = {
  type: 'object',
  properties: {
    state: { type: 'string', enum: ['ready', 'parked', 'merged', 'skipped'] },
    branch: str, pr: str, plan: str, worktree: str,
    rulings: strs, findings, parkNote: str, batchHalt: str,
    // True when the agent pushed new commits; a fix that only ruled on findings leaves nothing new to review.
    changed: { type: 'boolean' },
  },
  required: ['state', 'branch', 'pr', 'plan', 'worktree', 'rulings', 'findings', 'parkNote', 'batchHalt', 'changed'],
}
const CHANGE = { type: 'object', properties: { file: str, summary: str }, required: ['file', 'summary'] }
const REVIEW = {
  type: 'object',
  properties: {
    changeFindings: { type: 'array', items: CHANGE }, otherFindings: findings, batchHalt: str,
    recordedWriterFindings: { type: 'boolean' },
  },
  required: ['changeFindings', 'otherFindings', 'batchHalt', 'recordedWriterFindings'],
}
const MERGE = {
  type: 'object',
  properties: {
    state: { type: 'string', enum: ['merged', 'parked', 'skipped', 'needs-review'] },
    parkNote: str, batchHalt: str, mergeWorktree: str, conflictedFiles: strs, leftInPlace: strs,
    rulings: strs, findings,
    // For needs-review: what the change under review is for, the failure it answers, and the commit it was made on,
    // so the reviewer judges that change and not the whole branch.
    reason: { type: 'string', enum: ['', 'conflict', 'gate', 'ci'] }, failure: str, fixBase: str,
  },
  required: ['state', 'parkNote', 'batchHalt', 'mergeWorktree', 'conflictedFiles', 'leftInPlace', 'rulings',
    'findings', 'reason', 'failure', 'fixBase'],
}

const prompt = (file, section, inputs) =>
  `You are a /work-backlog-parallel workflow agent with no human watching. Read ${SKILL}/common.md, then follow ` +
  `the section "${section}" of ${SKILL}/${file}. Inputs (JSON): ${JSON.stringify(inputs)}`

// The cap bounds build, fix and merge agents together: each runs the full pre-push gate when it pushes. Merge and
// fix agents go ahead of builds still waiting, so ready work is not held behind issues that have not started.
let free = A.cap
const urgent = []
const waiting = []
async function slot(fn, { priority = false } = {}) {
  if (free > 0) free--
  else await new Promise((resolve) => (priority ? urgent : waiting).push(resolve))
  try {
    return await fn()
  } finally {
    const next = urgent.shift() || waiting.shift()
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
// behind the cap never starts after it. The first cause is the one the summary leads with.
let batchHalt = ''
const halt = (cause) => { if (cause && !batchHalt) batchHalt = cause }
const HALTED = { halted: true }
const unlessHalted = (start) => {
  if (batchHalt) return HALTED
  return start().then((result) => {
    if (result) halt(result.batchHalt)
    return result
  })
}
const haltNote = () => `Batch halted: ${batchHalt}`
// The agent that hit a batch-level failure reports it in batchHalt and may leave parkNote empty. A park with no note
// at all still needs one: the summary and leftovers.md ask the user the note's question.
const writerNote = (result) => result.parkNote || (result.batchHalt ? `Batch halted: ${result.batchHalt}` : '') ||
  (result.state === 'parked' ? 'The agent parked the issue without saying why; read its PR and branch.' : '')

// An agent returns null on terminal API errors and when the user skips it from /workflows; the script cannot tell
// the two apart. One that stalls on every attempt or exceeds the StructuredOutput retry cap throws. Either way one
// retry is safe because every agent starts by reconciling against
// git and GitHub, and parking at once would also park every dependent. A second throw reaches runIssue's catch. Once
// a batch halt is set there is no retry, and the caller reports the halt, not a death (dead()).
async function call(text, opts) {
  try {
    const result = await agent(text, opts)
    if (result) return result
  } catch {
    // Retried below.
  }
  if (batchHalt) return null
  return agent(text, { ...opts, label: `${opts.label} retry` })
}
const dead = (who) =>
  (batchHalt ? haltNote() : `The ${who} returned no result twice (an API failure, or skipped from /workflows).`)

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
  const stack = [...afterOf[start]]
  while (stack.length) {
    const m = stack.pop()
    if (m === start) return true
    if (seen.has(m)) continue
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
  settle[rec.n](rec.state)
  return rec
}

async function mergeIssue(rec) {
  if (batchHalt) return { state: 'parked', parkNote: haltNote(), batchHalt: '' }
  const inputs = { n: rec.n, branch: rec.branch, pr: rec.pr, worktrees: rec.worktrees, date: A.date, main: A.main }
  let resolutionReviewed = false
  let resolutionFindings = []
  let mergeWorktree = ''
  // A call that returns needs-review has not pushed, so its rulings are not in the PR body yet; the next call writes
  // them before its push.
  const pendingRulings = []
  // Parking here, between merge calls, leaves a rebased branch no agent pushed; the note must say where it is.
  const park = (note, cause = '') => ({
    state: 'parked', batchHalt: cause,
    parkNote: mergeWorktree
      ? `${note} The worktree ${mergeWorktree} holds an unpushed, rebased ${rec.branch} that diverges from ` +
        `origin/${rec.branch}.`
      : note,
  })
  // A second failure must still park through park(), which keeps the divergence sentence leftovers.md relies on;
  // runIssue's catch would write a generic note.
  const ask = (text, opts) => call(text, opts).catch((error) => ({ thrown: String(error) }))
  // A conflict resolution, or a fix for the gate or CI, goes to review and back; main can move during CI and force
  // another, so three at most.
  for (const attempt of [1, 2, 3]) {
    const merge = await slot(() => unlessHalted(() => ask(
      prompt('merge-queue.md', 'Merge queue', { ...inputs, resolutionReviewed, resolutionFindings, pendingRulings }),
      { label: `merge #${rec.n}${attempt > 1 ? ` a${attempt}` : ''}`, phase: 'Merge', schema: MERGE })),
    { priority: true })
    if (merge === HALTED) return park(haltNote())
    if (!merge) return park(dead('merge agent'))
    if (merge.thrown) return park(`The merge agent failed twice: ${merge.thrown}`)
    // The merge agent may have made merge-<n>; the record must name every worktree it can leave behind.
    if (merge.mergeWorktree && !rec.worktrees.includes(merge.mergeWorktree)) rec.worktrees.push(merge.mergeWorktree)
    rec.rulings.push(...(merge.rulings || []))
    rec.unrecorded.push(...(merge.findings || []))
    if (merge.state !== 'needs-review') return merge
    // The resolution review needs both to judge the change; without them it would review nothing.
    if (!merge.mergeWorktree || !merge.reason) {
      return park('The merge agent asked for review without a merge worktree or a reason.')
    }
    pendingRulings.push(...(merge.rulings || []).filter((r) => !r.startsWith('Summary:')))
    mergeWorktree = merge.mergeWorktree
    if (attempt === 3) {
      return park('Three changes needing review in a row (conflict resolutions, gate or CI fixes); the third was ' +
        'not reviewed.')
    }
    if (batchHalt) return park(haltNote())
    // The review appends its out-of-scope findings to <n>-findings.md itself, so they are not unrecorded.
    const review = await ask(
      prompt('agents.md', 'Review', {
        n: rec.n, resolution: true, reason: merge.reason, failure: merge.failure, fixBase: merge.fixBase,
        mergeWorktree, conflictedFiles: merge.conflictedFiles, branch: rec.branch, pr: rec.pr, main: A.main,
      }),
      { label: `review #${rec.n} resolution a${attempt}`, phase: 'Review', schema: REVIEW })
    if (!review) return park(dead('resolution review agent'))
    if (review.thrown) return park(`The resolution review agent failed twice: ${review.thrown}`)
    if (review.batchHalt) {
      halt(review.batchHalt)
      rec.unrecorded.push(...review.otherFindings)
      return park(haltNote(), review.batchHalt)
    }
    resolutionReviewed = true
    resolutionFindings = review.changeFindings
  }
}

async function runIssue(item) {
  const n = item.n
  const rec = {
    n, state: 'building', note: '', branch: '', pr: '', plan: '', worktrees: [], rulings: [], unrecorded: [],
    leftInPlace: [],
  }
  records[n] = rec

  if (inCycle(n)) return end(rec, 'parked', `Its "after" list forms a dependency cycle: ${item.after.join(', ')}.`)
  for (const m of item.after) {
    const state = await finished[m]
    if (state !== 'merged') return end(rec, 'parked', `Depends on #${m}, which ended ${state}.`)
  }

  const build = await slot(() => unlessHalted(() => call(
    prompt('agents.md', 'Build', {
      n, kind: item.kind, spec: item.spec, piece: item.piece, after: item.after, haltAnswers: item.haltAnswers,
      date: A.date, main: A.main,
    }),
    { label: `build #${n}`, phase: 'Build', schema: WRITER, isolation: 'worktree' })))
  if (build === HALTED) return end(rec, 'parked', haltNote())
  if (!build) return end(rec, 'parked', dead('build agent'))
  rec.branch = build.branch
  rec.pr = build.pr
  rec.plan = build.plan
  if (build.worktree) rec.worktrees.push(build.worktree)
  rec.rulings.push(...build.rulings)
  rec.unrecorded = build.findings
  halt(build.batchHalt)
  // A build that skipped at its first step changed nothing, so its worktree is not left in place.
  if (build.state !== 'ready') return end(rec, build.state, writerNote(build))
  // Every review, fix and merge prompt names the branch and PR; without them the rest would act on nothing.
  if (!build.branch || !build.pr) return end(rec, 'parked', 'The build agent returned ready without a branch or PR.')

  let last = build
  let head = build.worktree
  for (const round of [1, 2]) {
    if (batchHalt) return end(rec, 'parked', haltNote())
    const review = await call(
      prompt('agents.md', 'Review', {
        n, round, kind: item.kind, spec: item.spec, branch: rec.branch, pr: rec.pr, headWorktree: head,
        plan: rec.plan, worktrees: rec.worktrees, build: last, main: A.main,
      }),
      { label: `review #${n} r${round}`, phase: 'Review', schema: REVIEW })
    if (!review) return end(rec, 'parked', dead(`round ${round} review agent`))
    // A review stopped by a batch halt may not have recorded the writer's findings; keep them for the controller
    // unless it says it did.
    if (review.batchHalt) {
      halt(review.batchHalt)
      if (review.recordedWriterFindings) rec.unrecorded = []
      rec.unrecorded.push(...review.otherFindings)
      return end(rec, 'parked', haltNote())
    }
    if (review.recordedWriterFindings) rec.unrecorded = []
    if (!review.changeFindings.length) break
    const fix = await slot(() => unlessHalted(() => call(
      prompt('agents.md', 'Fix', {
        n, round, kind: item.kind, branch: rec.branch, pr: rec.pr, spec: item.spec, plan: rec.plan,
        changeFindings: review.changeFindings, date: A.date, main: A.main,
      }),
      { label: `fix #${n} r${round}`, phase: 'Fix', schema: WRITER, isolation: 'worktree' })), { priority: true })
    if (fix === HALTED) return end(rec, 'parked', haltNote())
    if (!fix) return end(rec, 'parked', dead(`round ${round} fix agent`))
    if (fix.worktree) rec.worktrees.push(fix.worktree)
    rec.rulings.push(...fix.rulings)
    rec.unrecorded = fix.findings
    halt(fix.batchHalt)
    // A fix skipped after the build pushed leaves the build's worktree behind.
    if (fix.state === 'skipped') rec.leftInPlace = [...rec.worktrees]
    if (fix.state !== 'ready') return end(rec, fix.state, writerNote(fix))
    last = fix
    head = fix.worktree
    // A fix that pushed nothing (every finding ruled on, none fixed) leaves the reviewed code as it was.
    if (!fix.changed) break
  }

  const merged = await serial(() => mergeIssue(rec))
  halt(merged.batchHalt)
  if (merged.leftInPlace && merged.leftInPlace.length) rec.leftInPlace = merged.leftInPlace
  else if (merged.state === 'skipped') rec.leftInPlace = [...rec.worktrees]
  return end(rec, merged.state, writerNote(merged))
}

phase('Build')
log(`Batch of ${A.batch.length}, cap ${A.cap}`)
await parallel(A.batch.map((item) => () => runIssue(item).catch((error) => {
  const rec = records[item.n] || {
    n: item.n, note: '', branch: '', pr: '', plan: '', worktrees: [], rulings: [], unrecorded: [], leftInPlace: [],
  }
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
