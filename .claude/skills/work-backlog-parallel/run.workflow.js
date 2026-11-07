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

// args: {date, main, cap, deadline, maxParked, stopBeforeMerge, batch: [{n, kind, after: [m], spec, piece,
// haltAnswers}]}; see SKILL.md "The run". Dates come from args because Date and Math.random throw in a workflow
// script (they would break resume), so the deadline is checked by each build agent, which can read the clock.
const A = args

// A malformed args file must fail before any agent starts: a missing cap would leave every slot waiting forever, a
// string issue number would slip past the cycle check, and a duplicate would build and merge one issue twice.
function checkArgs() {
  const bad = (why) => { throw new Error(`run-args.json is invalid: ${why}`) }
  if (!A || typeof A !== 'object' || Array.isArray(A)) bad('args must be an object')
  if (typeof A.date !== 'string' || !A.date) bad('date must be a non-empty string')
  // Every agent prompt is `${A.main}/.claude/skills/…`; a relative path would resolve against each agent's own cwd.
  if (typeof A.main !== 'string' || !A.main.startsWith('/')) bad('main must be an absolute path')
  if (!Number.isInteger(A.cap) || A.cap < 1) bad('cap must be an integer of at least 1')
  // Build agents compare `date --utc +%FT%TZ` with the deadline as strings, which orders correctly only in this form.
  if (typeof A.deadline !== 'string' || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/.test(A.deadline)) {
    bad('deadline must be a UTC time like 2026-01-31T18:00:00Z')
  }
  if (!Number.isInteger(A.maxParked) || A.maxParked < 1) bad('maxParked must be an integer of at least 1')
  if (typeof A.stopBeforeMerge !== 'boolean') bad('stopBeforeMerge must be true or false')
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
  const afterOf = {}
  for (const item of A.batch) {
    for (const m of item.after) {
      if (!Number.isInteger(m)) bad(`#${item.n} lists ${JSON.stringify(m)} in "after", not an integer`)
      // planning.md keeps a dependent out of the batch when its prerequisite is outside it and unmerged.
      if (!seen.has(m)) bad(`#${item.n} depends on #${m}, which is not in the batch`)
    }
    afterOf[item.n] = item.after
  }
  // An issue whose `after` chain leads back to itself would wait forever. Rejected here, not parked at run time: a
  // planning typo must not spend the parked budget while the rest of the batch runs.
  for (const item of A.batch) {
    const visited = new Set()
    const stack = [...item.after]
    while (stack.length) {
      const m = stack.pop()
      if (m === item.n) bad(`#${item.n} is in a dependency cycle through its "after" list`)
      if (visited.has(m)) continue
      visited.add(m)
      stack.push(...afterOf[m])
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
    // True when a build skipped because the run's deadline had passed; its dependents are skipped with it.
    deadlinePassed: { type: 'boolean' },
  },
  required: ['state', 'branch', 'pr', 'plan', 'worktree', 'rulings', 'findings', 'parkNote', 'batchHalt', 'changed',
    'deadlinePassed'],
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
    // ready-to-merge: the shakedown's stopAtMerge call, current and green on CI, left for the controller to merge.
    state: { type: 'string', enum: ['merged', 'parked', 'skipped', 'needs-review', 'ready-to-merge'] },
    parkNote: str, batchHalt: str, mergeWorktree: str, conflictedFiles: strs, leftInPlace: strs,
    rulings: strs, findings,
    // For needs-review: what the change under review is for, the failure it answers, and the commit the reviewer
    // diffs from, so it judges that change and not the whole branch: for a gate or CI fix the commit the fix was
    // made on; for a conflict resolution the pre-rebase head (the old remote SHA), since a diff from origin/main
    // would show the branch's whole change to the file and hide a dropped main-side line.
    reason: { type: 'string', enum: ['', 'conflict', 'gate', 'ci'] }, failure: str, fixBase: str,
  },
  required: ['state', 'parkNote', 'batchHalt', 'mergeWorktree', 'conflictedFiles', 'leftInPlace', 'rulings',
    'findings', 'reason', 'failure', 'fixBase'],
}

// merge-queue.md is one agent's whole instruction set, so its prompt names the file, not a section of it.
const prompt = (file, section, inputs) =>
  `You are a /work-backlog-parallel workflow agent with no human watching. Read ${SKILL}/common.md, then follow ` +
  (section ? `the section "${section}" of ${SKILL}/${file}.` : `the whole of ${SKILL}/${file}.`) +
  ` Inputs (JSON): ${JSON.stringify(inputs)}`

// The cap bounds build and fix agents together: each runs the full pre-push gate when it pushes. Fix agents go
// ahead of builds still waiting, so ready work is not held behind issues that have not started. The merge agent
// takes no slot: serial() already bounds it to one, so at most cap + 1 gates run, and a merge never waits for a
// build to finish (a slot freed by a build goes to the next queued build before the unslotted review can ask).
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
// Notes are joined into one paragraph, so each part must end its own sentence.
const sentence = (text) => (/[.!?]$/.test(text) ? text : `${text}.`)
// The agent that hit a batch-level failure reports it in batchHalt and may leave parkNote empty. A park with no note
// at all still needs one: the summary and leftovers.md ask the user the note's question. A skip with no note is the
// user closing the issue or its PR (the only skip an agent makes without a note), and the summary must say so.
const writerNote = (result) => result.parkNote || (result.batchHalt ? `Batch halted: ${result.batchHalt}` : '') ||
  (result.state === 'parked' ? 'The agent parked the issue without saying why; read its PR and branch.' : '') ||
  (result.state === 'skipped' ? 'Skipped: the agent found the issue or its PR closed.' : '')

// An agent returns null on terminal API errors and when the user skips it from /workflows; the script cannot tell
// the two apart. One that stalls on every attempt or exceeds the StructuredOutput retry cap throws. Either way one
// retry is safe because every agent starts by reconciling against
// git and GitHub, and parking at once would also park every dependent. A second throw propagates; ask() below turns
// it into a result. Once a batch halt is set there is no retry, and the caller reports the halt, not a death (dead()).
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
// A second throw must name the agent, not read as a script defect: runIssue's catch would write "The script failed".
const ask = (text, opts) => call(text, opts).catch((error) => ({ thrown: String(error) }))
const failed = (who, result) => `The ${who} failed twice: ${result.thrown}`

const records = {}
const settle = {}
const finished = {}
for (const item of A.batch) {
  finished[item.n] = new Promise((resolve) => { settle[item.n] = resolve })
}

const TERMINAL = ['merged', 'parked', 'skipped', 'ready-to-merge']
// The parked budget counts issues that parked on their own work: a dependent parked for its prerequisite, or an
// issue parked by a batch halt, says nothing new about how the run is going.
let parkedCount = 0
// Kept off the records, which the result returns as SKILL.md "The run" step 5 lists them.
const parkCounted = new Set()
const budgetStopped = new Set()
// A park is counted while its agent still holds the slot, so the build that takes the slot next already sees it. A
// dead agent (null, or a second throw) parks its issue too, so it counts here for the same reason; end() counts the
// rest (a contradictory output) and skips a park counted here.
function countPark(rec, result) {
  const parks = !result || result.thrown || (result.state === 'parked' && !result.batchHalt)
  if (parks && !batchHalt && !parkCounted.has(rec.n)) {
    parkCounted.add(rec.n)
    parkedCount++
  }
  return result
}
function end(rec, state, note, { counts = true } = {}) {
  if (TERMINAL.includes(state)) {
    rec.state = state
    if (note) rec.note = note
  } else {
    rec.state = 'parked'
    rec.note = `The issue stopped in state "${state}". ${note || ''}`.trim()
  }
  if (rec.state === 'parked' && counts && !batchHalt && !parkCounted.has(rec.n)) {
    parkCounted.add(rec.n)
    parkedCount++
  }
  settle[rec.n](rec.state)
  return rec
}
const budgetNote = () => `Not started: the run's parked budget (${A.maxParked}) was spent.`
// The build agent's deadline note is free text; leftovers.md and the summary key on this wording.
const deadlineNote = () => `Not started: the run's deadline (${A.deadline}) passed before this issue started.`
// A prerequisite that stopped for a budget, that the user closed, or that waits for the user's merge, leaves its
// dependent unstarted, not broken: it is skipped, naming the reason, and builds in a later session.
const UNSTARTED = {
  'ready-to-merge': (m) => `Not started: #${m} waits for the user's OK to merge.`,
  skipped: (m) => `Not started: #${m} did not merge: ${records[m].note}`,
}

const READY_NOTE = 'Reviewed; waits for the user\'s OK to merge.'

// stopAtMerge carries the shakedown: the merge agent rebases, runs the gate and waits for green CI, then returns
// ready-to-merge without merging, so the shakedown exercises the queue and the controller asks before the merge.
async function mergeIssue(rec, item) {
  if (batchHalt) return { state: 'parked', parkNote: haltNote(), batchHalt: '' }
  const inputs = {
    n: rec.n, branch: rec.branch, pr: rec.pr, worktrees: rec.worktrees, kind: item.kind, spec: item.spec,
    plan: rec.plan, haltAnswers: item.haltAnswers, stopAtMerge: A.stopBeforeMerge, date: A.date, main: A.main,
  }
  let resolutionReviewed = false
  let resolutionFindings = []
  let mergeWorktree = ''
  // The previous call's reason: after a `ci` fix the rebased branch is already pushed, which changes the lease SHA
  // the next call uses and what a park note must say about the worktree.
  let lastReason = ''
  // A call that returns needs-review has not pushed, so its rulings are not in the PR body yet; the next call writes
  // them before its push.
  const pendingRulings = []
  // Parking here, between merge calls, leaves work no agent pushed; the note must say where it is and what it is,
  // because leftovers.md keys its recommendation on these sentences.
  const park = (note, cause = '') => ({
    state: 'parked', batchHalt: cause,
    parkNote: !mergeWorktree ? note
      : lastReason === 'ci'
        ? `${sentence(note)} The worktree ${mergeWorktree} holds a rebased ${rec.branch} that is ahead of ` +
          `origin/${rec.branch} by an unpushed fix.`
        : `${sentence(note)} The worktree ${mergeWorktree} holds an unpushed, rebased ${rec.branch} that diverges from ` +
          `origin/${rec.branch}.`,
  })
  // A conflict resolution, or a fix for the gate or CI, goes to review and back; main can move during CI and force
  // another, so three at most. No slot (see slot()): a second failure parks through park(), which keeps the
  // worktree sentence leftovers.md relies on.
  for (const attempt of [1, 2, 3]) {
    const merge = await unlessHalted(() => ask(
      prompt('merge-queue.md', '', { ...inputs, resolutionReviewed, resolutionFindings, pendingRulings, lastReason }),
      { label: `merge #${rec.n}${attempt > 1 ? ` a${attempt}` : ''}`, phase: 'Merge', schema: MERGE })
      .then((r) => countPark(rec, r)))
    if (merge === HALTED) return park(haltNote())
    if (!merge) return park(dead('merge agent'))
    if (merge.thrown) return park(failed('merge agent', merge))
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
    lastReason = merge.reason
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
    if (review.thrown) return park(failed('resolution review agent', review))
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

  for (const m of item.after) {
    const state = await finished[m]
    if (budgetStopped.has(m)) {
      budgetStopped.add(n)
      return end(rec, 'skipped', records[m].note)
    }
    if (UNSTARTED[state]) return end(rec, 'skipped', UNSTARTED[state](m))
    // After a batch halt every note leads with the halt, so the summary's first line explains them all.
    if (state !== 'merged') {
      const why = `Depends on #${m}, which ended ${state}.`
      return end(rec, 'parked', batchHalt ? `${haltNote()} (${why})` : why, { counts: false })
    }
  }

  // The budget is checked once a slot is free, so a build queued behind the cap does not start after it is spent.
  const BUDGET = { budget: true }
  const build = await slot(() => (parkedCount >= A.maxParked ? BUDGET : unlessHalted(() => ask(
    prompt('agents.md', 'Build', {
      n, kind: item.kind, spec: item.spec, piece: item.piece, after: item.after, haltAnswers: item.haltAnswers,
      date: A.date, main: A.main, deadline: A.deadline,
    }),
    { label: `build #${n}`, phase: 'Build', schema: WRITER, isolation: 'worktree' }).then((r) => countPark(rec, r)))))
  if (build === BUDGET) {
    budgetStopped.add(n)
    return end(rec, 'skipped', budgetNote())
  }
  if (build === HALTED) return end(rec, 'parked', haltNote())
  if (!build) return end(rec, 'parked', dead('build agent'))
  if (build.thrown) return end(rec, 'parked', failed('build agent', build))
  rec.branch = build.branch
  rec.pr = build.pr
  rec.plan = build.plan
  if (build.worktree) rec.worktrees.push(build.worktree)
  rec.rulings.push(...build.rulings)
  rec.unrecorded = [...build.findings]
  halt(build.batchHalt)
  // A build that skipped at its first step changed nothing, so its worktree is not left in place. One that found the
  // deadline passed stopped for the run's time budget, and its dependents stop with it.
  if (build.state === 'skipped' && build.deadlinePassed) {
    budgetStopped.add(n)
    return end(rec, 'skipped', deadlineNote())
  }
  // A build that reconciled to merged (its merge agent died after the merge) still needs the queue's pull and
  // cleanup: the merge agent's section 1 takes a MERGED PR straight there.
  if (build.state === 'merged') return mergeAndEnd(rec, item)
  if (build.state !== 'ready') return end(rec, build.state, writerNote(build))
  // Every review, fix and merge prompt names the branch and PR; without them the rest would act on nothing.
  if (!build.branch || !build.pr) return end(rec, 'parked', 'The build agent returned ready without a branch or PR.')

  let last = build
  let head = build.worktree
  for (const round of [1, 2]) {
    if (batchHalt) return end(rec, 'parked', haltNote())
    const review = await ask(
      prompt('agents.md', 'Review', {
        n, round, kind: item.kind, spec: item.spec, piece: item.piece, haltAnswers: item.haltAnswers,
        branch: rec.branch, pr: rec.pr, headWorktree: head, plan: rec.plan, worktrees: rec.worktrees, build: last,
        main: A.main,
      }),
      { label: `review #${n} r${round}`, phase: 'Review', schema: REVIEW })
    if (!review) return end(rec, 'parked', dead(`round ${round} review agent`))
    if (review.thrown) return end(rec, 'parked', failed(`round ${round} review agent`, review))
    // A review that recorded the writer's findings wrote only the ones in its `build` input, so only those leave the
    // unrecorded list; what an earlier review failed to write stays. A review stopped by a batch halt may not have
    // recorded them at all; keep them for the controller unless it says it did.
    const recorded = () => {
      if (review.recordedWriterFindings) rec.unrecorded = rec.unrecorded.filter((f) => !last.findings.includes(f))
    }
    if (review.batchHalt) {
      halt(review.batchHalt)
      recorded()
      rec.unrecorded.push(...review.otherFindings)
      return end(rec, 'parked', haltNote())
    }
    recorded()
    if (!review.changeFindings.length) break
    const fix = await slot(() => unlessHalted(() => ask(
      prompt('agents.md', 'Fix', {
        n, round, kind: item.kind, branch: rec.branch, pr: rec.pr, spec: item.spec, piece: item.piece,
        haltAnswers: item.haltAnswers, plan: rec.plan, changeFindings: review.changeFindings, date: A.date,
        main: A.main,
      }),
      { label: `fix #${n} r${round}`, phase: 'Fix', schema: WRITER, isolation: 'worktree' })
      .then((r) => countPark(rec, r))), { priority: true })
    if (fix === HALTED) return end(rec, 'parked', haltNote())
    if (!fix) return end(rec, 'parked', dead(`round ${round} fix agent`))
    if (fix.thrown) return end(rec, 'parked', failed(`round ${round} fix agent`, fix))
    if (fix.worktree) rec.worktrees.push(fix.worktree)
    rec.rulings.push(...fix.rulings)
    // The next review writes these down and removes them; what no review writes stays for the controller.
    rec.unrecorded.push(...fix.findings)
    halt(fix.batchHalt)
    // A fix skipped after the build pushed leaves the build's worktree behind.
    if (fix.state === 'skipped') rec.leftInPlace = [...rec.worktrees]
    if (fix.state !== 'ready') return end(rec, fix.state, writerNote(fix))
    last = fix
    head = fix.worktree
    // A fix that pushed nothing (every finding ruled on, none fixed) leaves the reviewed code as it was.
    if (!fix.changed) break
  }

  return mergeAndEnd(rec, item)
}

async function mergeAndEnd(rec, item) {
  const merged = await serial(() => mergeIssue(rec, item))
  halt(merged.batchHalt)
  // The shakedown's merge is the controller's, after the user's OK (SKILL.md "Ending"); its worktree stays for it.
  if (merged.state === 'ready-to-merge') return end(rec, 'ready-to-merge', READY_NOTE)
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
  readyToMerge: all.filter((r) => r.state === 'ready-to-merge').map((r) => r.n),
  batchHalt: batchHalt || null,
  records: all,
}
