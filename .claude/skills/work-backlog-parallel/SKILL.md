---
name: work-backlog-parallel
description: >-
  Works this repo's GitHub issue backlog with independent issues built concurrently by a workflow and merged one at a
  time on green CI, after the user plans and approves the batch in one sitting. Use when asked to work the backlog
  with several issues built at once, for a short session or a long unattended batch. Arguments: `plan` for a
  read-only proposal, `short` or `long` for the batch size, issue numbers to choose the batch. For two or three
  small bugs, /work-backlog is about as fast and simpler. The first, shakedown run builds one issue, so start with
  /work-backlog and switch once shakedown is lifted.
argument-hint: "[plan] [short|long] [<issue>...]"
disable-model-invocation: true
---

# Working the backlog in parallel

One run of this skill is one parallel backlog session: the `/work-backlog` session, with independent issues built
concurrently by a workflow and merged one at a time. Everything `/work-backlog` says governs here unless this file
or a file beside it names an exception, and every exception exists because a workflow builds the issues; any other
rule lives in `/work-backlog` and is cited here. The names below ("Terms", "State", "The filing bar", "Merging" and
the rest) are sections of `.claude/skills/work-backlog/SKILL.md`, and "PRs", "Permissions", "Long commands" and the
like are bullets of its "Doing the work".

## What to read, when

Read `/work-backlog` `SKILL.md` in full before step 1: it governs this session. Its sub-files and the files beside
this one are read at the step that uses them, not up front; the agents read the build, fix and merge procedures, so
the controller does not.

- **Before step 1:** `/work-backlog` `SKILL.md`.
- **After a compaction, or when a halt is recorded:** `/work-backlog` `resume.md` ("On invocation" below says
  which of its steps apply here).
- **"The session" step 1:** the files `/work-backlog` steps 1–5 name as they come up (`leftovers.md`,
  `findings-walkthrough.md`, and `triage.md` when the stall check fires); leftover work finished with
  `/work-backlog`'s own flow reads what that flow reads.
- **The permission mode (step 1) and "The run" step 2:** `/work-backlog` `permissions.md`.
- **"The session" step 2:** `planning.md` and `/work-backlog` `halt-questions.md`; for a feature batch,
  `feature-flow.md` steps 1–5 and `sub-skill-overrides.md` (its brainstorming entries replace the hand-off and the
  hard gate, and the Fable review runs as a subagent of this session).
- **"Ending":** `leftovers.md` "A parallel run's files"; `findings-walkthrough.md`; `bug-flow.md` and
  `sub-skill-overrides.md` for a "fix it now".

`common.md`, `agents.md` and `merge-queue.md` are the agents' instructions; read one only to answer a question about
what an agent did.

## How the session runs

All of the user's part comes first, in one sitting: the batch OK, every feature brainstorm, one combined question
round, the permission request and the agent-count estimate (`planning.md`). Then one workflow builds, reviews and
merges the batch with nobody watching. The user comes back to merged PRs, the findings walkthrough and the summary.

Merging stays serial, because the `protect-main` ruleset requires a branch to be up to date with `main` and there is
no GitHub merge queue: every merge puts every other ready branch behind, so each one rebases, re-runs the pre-push
gate and waits for CI in turn. What runs in parallel is building and review. The proposal's wall-clock estimate says
so.

A workflow run belongs to the session that started it: its journal lives in that session's directory, and only that
session can resume it. A workflow never survives the process exiting: a `claude --resume` of that session is told the
run stopped with no completion record, and relaunches it with the same `scriptPath` and `args` plus
`resumeFromRunId` ("The run" step 4); nothing resumes it on its own. A new session cannot resume it at all.

## Arguments

`/work-backlog` "Arguments" holds: `plan`, issue numbers and an unknown argument behave as it says. `short` or `long`
is this skill's own (a named exception below) and picks the batch size `planning.md` "Choosing the batch" defines;
without one the proposal is the short batch. Named issues make the batch, so a size beside them is ignored.

In plan-only mode, step 1's two checks report instead of asking (a missing Workflow tool and the permission mode go
in the printed proposal), and step 2 prints `planning.md`'s proposal (graph, cap, budgets, wall-clock estimate and
agent count included) with no brainstorm, no spec and no `current-session.md`. A feature batch's proposal names its
families and the specs they would need. The run and "Ending" never start.

## Shakedown and budgets

`/work-backlog` "Shakedown" and "Budgets" hold, carried into the run by its args ("The run" step 1):

- **While the shakedown is on**, the batch is one issue (`planning.md` gives the cap) and `stopBeforeMerge` is true:
  the script stops the issue after its review as `ready-to-merge`, and "Ending" step 1 asks the user before merging
  it. A workflow agent cannot ask anyone, so the merge moves to the controller.
- **The time budget** is `deadline`: `max-hours` from the launch time, computed as "Budgets" computes it from
  `started:`, right before launch. A build agent that would start fresh after it skips its issue (`agents.md` "Build"
  step 2); the script cannot read the clock. `deadline` stands in for the `started:` line, which this skill does not
  write.
- **The parked budget** is `maxParked`: once that many issues have parked on their own work (not for a prerequisite
  or a batch halt), no further build starts. An agent that dies (no result twice, or two failures) parks its issue
  and spends the budget too, so an API outage stops the batch rather than parking every issue in turn.

A budget skips, never parks: the issue was never started, so there is no question to ask, and it is named in the
summary for the next session. An issue whose prerequisite skipped for any reason (a budget, the user closing it or its
PR) skips with it, and so does one whose prerequisite stopped as `ready-to-merge`: its build would start from code the
user has not yet let in. Work already started is finished whatever the budgets say. A result the Workflow tool
reports as the script failing is handled as a lost run ("The run" step 4).

## Named exceptions to `/work-backlog`

- **Issues run concurrently** within the dependency graph `planning.md` builds, up to the cap; merges are serial.
  Replaces "Strictly one issue at a time" ("Doing the work") and its red flag.
- **`short` and `long` arguments.** A batch has no fixed size, so the user picks one of the two `planning.md`
  defines; the serial chunk size needs no argument. Adds to "Arguments".
- **No chunk size limit.** Replaces "Size" in "Choosing the chunk". Builds overlap and every question is asked before
  the run, so a longer batch costs the user no extra visit; only the serial merges grow with it, and the proposal's
  estimate shows that.
- **A batch may mix bugs and features and hold unrelated families.** Replaces the bugs-first exclusivity and the
  "Features" relatedness rule in "Choosing the chunk". Bugs-first stays only as a rule about what the batch holds (a
  backlog with doable bugs puts them in the batch); the order is "Order"'s, by leverage, not by kind. Every
  brainstorm and the
  one question round happen before the run, so unrelated work no longer means a second sitting, and the dependency
  graph, not relatedness, keeps conflicting work apart. An issue's kind is decided as "Choosing the chunk" says.
- **Feature batches** get one brainstorm and spec per family, then one combined question round and one permission
  request (`planning.md`). A mixed batch asks its bug halt questions in that round. Every likely halt question is
  asked, as `/work-backlog` asks it: a park answers nothing, it only moves the question to a later session.
- **An umbrella issue builds one piece per batch.** The script runs one build per issue number, so a batch holds at
  most one piece of an umbrella issue, named by its `piece`; its PR says `Part of #N` unless the piece is the
  spec's last. Replaces the run of every piece in one session that "PRs" describes.
- **A halt parks the issue and the batch goes on.** Every point where `/work-backlog` or a file it cites would send a
  banner halt during the unattended part (including `bug-flow.md` step 3, the plan-too-broken and three-failed-fixes
  entries of `sub-skill-overrides.md`, and every blocked banner in `merging.md`) instead sets that issue's state to
  `parked` with a one-paragraph note: what happened, what was tried, the question the user would have been asked
  and the recommended answer. Issues that depend on a parked issue park too. Batch-level failures (`common.md`) end
  the run.
- **A refused pull of local `main` ends the run.** `merging.md` step 3 carries on from `origin/main`; here the next
  merges would all hit the same refusal, so the merge queue halts the batch.
- **Some rebase conflicts are resolved**, not halted on (`merge-queue.md`): every conflict in the list and registry
  lines, `.coverage-baseline.txt` and `CHANGELOG.md` that `merge-queue.md` "3. Conflicts" names, whatever commit it
  meets, and a code conflict
  whose conflicting hunks both come from the batch's own PRs. Concurrent builds touch those files by design, so
  halting on them would park most feature batches. Any other conflict parks the issue, as `merging.md` step 2's
  blocked banner would halt it. Replaces the conflict blocked banners in `merging.md` step 2 for those conflicts
  only.
- **Out-of-scope findings are never judged mid-run by an agent.** Agents return them unjudged and the controller
  judges and folds them at the end, so there is no "Same PR" fix and no known-gap comment for them. Replaces
  "Out-of-scope findings" for findings a workflow agent makes during the run; before the run (the Fable review's
  non-design findings) and from "Ending" on (a "fix it now", a leftover finished serially), the serial section
  holds.
- **Worktrees are made by the workflow.** Build and fix agents run in tool-made worktrees
  (`.claude/worktrees/<runId>-<k>`, where the run id itself begins `wf_` and `k` counts the run's agents, not
  issues), which start from `origin/main`; the merge queue makes `.claude/worktrees/merge-<N>`. Replaces
  "Worktrees" creation; branch names still follow "Branches", and the baseline test run the "Worktrees" bullet names
  is the build agent's (`agents.md` "Build").
- **No `EnterWorktree` or `ExitWorktree` during the run.** From step 2 of "The session" until `workflow-done` is
  recorded ("The run" step 5), neither the controller nor any workflow agent calls either; from "Ending" on, the
  shakedown merge and a "fix it now" in the walkthrough enter a worktree as `/work-backlog` says. Replaces the
  re-enter step of "After a compaction" in `resume.md` during the run (the controller follows "On invocation"
  instead).
- **No "Leaving to write" during the run.** Every write an agent would make to the main checkout is returned as
  `findings` or `rulings`, or made by the review stage, which runs in the main checkout. Replaces "Leaving to write"
  for workflow agents.
- **Agents wait with `wait-for`.** Replaces "Long commands" for workflow agents (`common.md`): a background
  notification reaches an agent only while it keeps working, and nothing that arrives after its final message changes
  it.
- **A permission-mode question before planning.** Adds a halt to "The session" step 1: an agent that meets a prompt
  stalls its issue and every dependent.
- **A permission refusal is not an answer.** `permissions.md` lets a serial run go on after "skip the permission
  stuff"; here the run proceeds only when the session is bypassed or every rule string is matched ("The run" step 2),
  because an agent that meets a prompt stalls its issue and every dependent.
- **The shakedown merge question is asked at "Ending"**, not about fifteen minutes after the PR opens as "Choosing
  the chunk" has the proposal say: the controller runs `merging.md` steps 0–1 in the issue's worktree, asks on green
  as its step 2 does, then runs steps 2–4 ("Ending" step 1). `planning.md`'s proposal says so in place of the
  fifteen-minute line.
- **`workflow-launching:`** is a `current-session.md` line written right before the Workflow tool is invoked
  ("State"), so a death between the invocation and its return leaves a trace that "On invocation" sends to the
  trace check instead of planning again.
- **No early push or early PR for a build agent.** A build pushes and opens its PR only when the build is finished
  (`agents.md` "Build" step 4): a resumed run takes any open PR as a finished build and would merge a half-built
  one. Replaces the early push and early PR of "PRs" for build agents.
- **The plan's workspace is deleted after the PR exists**, not at the end of the plan's "Finish": a build that dies
  before its push would otherwise leave a finished branch with no ledger to resume from (`agents.md` "Build"
  step 3).
- **Fix commits carry a `Review-round: <round>` trailer** (`agents.md` "Fix"), so a fix agent re-run after a death
  sees which findings the dead one already covered; the serial controller remembers that itself.
- **No nested subagents.** Workflow agents are given no subagent tool here, so superpowers:subagent-driven-development
  is never chosen, superpowers:executing-plans' final review is skipped (its "With a subagent tool" and "Without a
  subagent tool" paragraphs both), and superpowers:requesting-code-review, wherever `/work-backlog` or `merging.md`
  calls for it from a workflow agent, is replaced by the review stage (`agents.md` "Review"), which performs
  `code-reviewer.md`'s review itself. The two-round cap holds: a round is one review stage plus one fix stage. The
  controller, which can dispatch, still runs superpowers:requesting-code-review where the serial flow calls for it
  (a merge-time fix at the shakedown merge, a leftover finished serially).
- **Nothing shared is written mid-run by a builder.** No build or fix agent edits the spec, `.claude/rules/`,
  `current-session.md` or `pending-findings.md` (Write, Edit and git are confined to their worktree, and `common.md`
  forbids shell writes outside it). A spec/code divergence (`feature-flow.md` step 6.3) is a `Ruling:` in the PR
  body; the controller corrects the spec at the end. A change the issue makes to what `.claude/CLAUDE.md` says is a
  `Ruling:` too, naming the edit; the controller applies it to the main checkout's copy at the end, as
  "Leaving to write" would serially. The plan is copied to the main checkout by the review stage.
- **`current-session.md` survives a run with parked issues**, trimmed to them ("Ending"). Replaces its deletion in
  step 9 while anything is parked.

## State

`/work-backlog`'s "State", plus:

- `current-session.md` also holds `cap: <n>`, the batch as `issue <N> after <M>,<K>` lines (or `issue <N>` with no
  prerequisite), with `piece <name>` after an umbrella issue's; `workflow-launching: <args path>`, written right
  before the Workflow tool is invoked and replaced by `workflow-script: <path>`, `workflow-args: <path>` and
  `workflow-run: <runId>`, written together the moment the launch returns ("The run" step 3), so their presence
  means a run was launched; `workflow-result: <path>` once the result is saved; and `workflow-done: <runId>` (or
  `workflow-done: lost`) once it is recorded.
- `.claude/backlog/parallel/result.json`: the workflow's result, verbatim, saved the moment it arrives, so a
  compaction while it is recorded loses nothing.
- `.claude/backlog/parallel/<N>.md`: one issue's progress, written by the review stage and the merge queue during
  the run and completed by the controller after it ("The run" step 5). An issue with no file has not reached its
  first review. Lines `state: ready|ready-to-merge|merged|parked|skipped` (`ready-to-merge`: reviewed, waiting for
  the shakedown's merge OK; `skipped`: the user closed the issue or its PR mid-run, or a budget kept it from starting),
  `branch:`, `pr:`, `plan:` (repo-relative), `worktrees:` (absolute paths of every worktree the issue's agents
  used), `ruling:` (one per ruling) and `park-note:`, which holds the record's `note`, for a parked or skipped
  issue.
- `.claude/backlog/parallel/<N>-findings.md`: the issue's out-of-scope findings, unjudged, in `pending-findings.md`'s
  entry format.
- `.claude/backlog/parallel/run-args.json`: the workflow's `args` ("The run").
- `.claude/backlog/parallel/summary.md`: "Ending" step 3's items, written before step 4 deletes their sources;
  deleted by whoever gives the summary (this skill's "Ending" step 4, or `/work-backlog` step 9).

## On invocation

On invocation and after a context compaction, read `.claude/backlog/current-session.md` first. After a compaction,
re-read this file first, then follow "After a compaction" in `/work-backlog` `resume.md` (which re-reads that skill's
`SKILL.md`), with one change: until `workflow-done` is recorded the session stays in the main checkout and
re-enters no worktree; from "Ending" on, that section's step 4 applies as written. This section says where to go.
Take the first that matches:

- **`workflow-done`:** the run is recorded: go to "Ending" and redo each step whose output is missing (findings not
  yet folded, the walkthrough, the summary, the trim).
- **`workflow-result`:** the run finished and its result is on disk: go to "The run", step 5.
- **`workflow-run`:** a run was launched and not recorded as finished: go to "The run", step 4 before anything
  else, `/work-backlog` step 1 included. Step 4 alone decides whether steps 2 and 3 run again. `leftovers.md` must
  not take that run's issues over while the run can be resumed.
- **`workflow-launching:` with no `workflow-run:`:** the tool may have launched a run before the session died: go to
  "The run" step 4's trace check (its new-session case), never to planning.
- **`session-date:` and `issue` lines but no `cap:`:** a feature sitting that died before its question round
  (`planning.md` "Writing it down" writes the file at the batch OK and `cap:` after the round): run `/work-backlog`
  steps 1–5, then resume `planning.md` "Feature batches" at the first family whose spec has no `spec approved:` line,
  under the batch OK the file records.
- **`cap:` and `issue` lines but no `workflow-*` line:** the batch was planned and approved but never launched (the
  three `workflow-*` lines are written together, the moment the launch returns), so no run needs protecting. Run
  `/work-backlog` steps 1–5 as any session does; step 3's `leftovers.md` offers launching it, recommended, or a
  take-over that strips `cap:`. On launch, go to "The run", step 1, under the approval the file records.
- **None of these** (no file, or one a serial session left): start at "The session", step 1.

## The session

1. **`/work-backlog` steps 1–5:** preconditions, log, unfinished work, leftover findings, stall check. Leftover work
   found here is finished with `/work-backlog`'s own flow, worktrees included, before planning starts; the session
   is back in the main checkout before step 2. From step 2 on it stays there: a session isolated in a worktree is
   refused `git -C` on the main checkout. Two checks come before planning, so the user's sitting is not wasted:
   - **The Workflow tool is available**: listed among the session's tools, or among its deferred tools, in which
     case load it with ToolSearch `select:Workflow` (its description states the session's workflow size
     guideline). Only if it is in neither, stop with the blocked banner and recommend `/work-backlog`.
   - **The permission mode.** Unless the session runs with permissions bypassed (`permissions.md` says how a session
     tells), recommend with the question banner restarting in bypass mode now (`planning.md` "The
     permission request" says why), or going on and adding allow rules later. A restart after planning also works
     ("On invocation"), but costs the user a second visit. In plan-only mode ("Arguments"), both checks are reported
     in the proposal, not asked.
2. **Plan the batch** with the user (`planning.md`, whose "Writing it down" writes `current-session.md` as it goes).
3. **The run** (below).
4. **Ending** (below): fold the findings, the walkthrough, the summary.

## The run

1. Write `.claude/backlog/parallel/run-args.json` with the workflow's `args`: `date` (the `session-date:` line of
   `current-session.md`, whatever today is), `main` (the main checkout's absolute path), `cap`, `deadline` (a UTC
   time like `2026-01-31T18:00:00Z`), `maxParked` (a whole number of at least 1) and `stopBeforeMerge` (`true` or
   `false`), the last three as "Shakedown and budgets" says, and `batch`: a list,
   in proposal order, of `{"n": <N>, "kind": "bug" | "feature", "after": [<M>, ...], "spec": "<path>",
   "piece": "<name>", "haltAnswers": "<text>"}`. `n` and every `after` entry are positive integers and `after` is
   always present (empty for none). `spec` is the spec's repo-relative path (`docs/superpowers/specs/…`), required
   for a feature and empty for a bug;
   confirm each named spec exists in the main checkout, since the script cannot. `piece` names an umbrella issue's
   piece, empty otherwise. `haltAnswers` holds the user's answers to the halt questions that bear on that issue, as
   `current-session.md` records them, empty when none do. The script rejects malformed args before any agent
   starts. Record nothing in `current-session.md` yet: the `workflow-*` lines are written together in step 3, so a
   restart between here and the launch finds a planned batch, not a run.
2. **Check before launching.** Nobody will answer a prompt: an agent that meets one holds its slot and stalls its
   issue and every issue that depends on it.
   - **Permissions:** `/work-backlog` `permissions.md`, its "Rule strings" and "Workflow agents add", checked with
     the re-read right before the unattended part it describes. The run proceeds when the session runs with
     permissions bypassed or every rule string is matched; otherwise ask the user to add the missing ones, naming
     them, or to restart in bypass mode ("On invocation" picks the batch up again). Workflow agents run with the
     session's permission mode and allow rules.
   - **Signing:** `permissions.md` "Signing".
   - **Local `main`:** the checks `permissions.md` "When to ask" makes right before the unattended part (the main
     checkout on `main`, with no local commit `origin/main` lacks). Workflow worktrees start from `origin/main`, so
     the user's local commits never reach a build, but the merge queue's pull of `main` would refuse them.
3. Write `workflow-launching:` with the args path to `current-session.md`, then invoke the Workflow tool with
   `scriptPath` set to the script's absolute path and `args` set to the file's contents (as a JSON value, not a
   string). The tool returns at once with the run id; the moment it does, replace that line with
   `workflow-script: .claude/skills/work-backlog-parallel/run.workflow.js`, `workflow-args:` with the args path and
   `workflow-run: <runId>` together. Then wait for the completion notification; do not poll.
   Skipping a stuck agent from `/workflows` takes two skips: the script cannot tell a skip from an API failure, so it
   retries the agent once, then parks the issue; after a batch halt it is not retried. A run that fails with
   "run-args.json is invalid" started no agent: write no result, correct the file, drop the three `workflow-*` lines
   and launch again. A result that reports the script itself failing is a lost run: go to step 4's trace check (the
   search for PRs and `wf_*` worktrees), skipping its resume advice.
   Any other result: first write it verbatim to `.claude/backlog/parallel/result.json` and record `workflow-result:`
   with that path, then go to step 5.
4. **Resume.** A compaction does not stop a background workflow, and its completion notification still arrives:
   after one, keep waiting. A session restart is different: the workflow died with the process, and what is possible
   depends on which session this is. The user is present then; ask with the question banner before relaunching, and
   run step 2 again first, since the restart may have changed the permission mode or the signing key. The model
   cannot run `/workflows` itself, so the same question asks the user to run it and say whether it lists the
   recorded `workflow-run`.
   - **The run's own session** (resumed with `claude --resume`; `/workflows` lists the run as stopped, since no
     completion record was found): relaunch with the same `scriptPath`, the same `args` and `resumeFromRunId` set to
     the recorded `workflow-run`, and record the run id the relaunch returns in place of the old one. Replay is
     cached up to the first call whose order or inputs changed; the rest re-run live, which is safe because every
     agent reconciles first against git and GitHub. Concurrent issues interleave by timing, so a resume may re-run
     more agents live than had finished.
   - **A new session** (`/workflows` does not list the run), or a relaunch the tool rejects ("journal … is not on
     disk"): the run cannot be resumed. Tell the user that resuming the old session with `claude --resume` would
     have replayed it, then check for traces of the run:
     `gh pr list --state all --limit 1000 --search '"Backlog-Session: <date>" in:body'`, with the `session-date:`
     line's date (the default limit of 30 can drop the match), and any `.claude/worktrees/wf_*`. None: nothing ran;
     launch from step 1 with the same batch, so `deadline` is recomputed from now (a fresh run has no replay to
     protect). Any: first check `git worktree list --porcelain` for a `wf_*` worktree whose
     lock PID is alive (`kill -0`, `leftovers.md` "Locked worktrees"), and ask whether the run's own session is
     still open. If either says the run may be going, leave everything as it is (no step 5, no "Ending", no change
     to `current-session.md` or `session-log.md`), tell the user to resume that session, and stop. Otherwise end
     it as a batch-level halt, cause "run lost", go to step 5 with no result, and sort every issue from its files
     and git as `leftovers.md` does.

   Never start a fresh run for a batch whose run left traces. This step is the one place that decides whether a
   run can be resumed; `leftovers.md` cites it.
5. **Record**, from `result.json` (or with no result, after a lost run). The result is
   `{merged, parked: [{n, note}], skipped, readyToMerge, batchHalt, records}`, one record per issue with its
   `state` (`merged`, `parked`, `skipped` or `ready-to-merge`), `note`,
   `branch`, `pr`, `plan`, `worktrees`, `rulings`, `unrecorded` findings (those no review stage wrote down) and
   `leftInPlace` (worktrees and branches the merge queue could not remove, or that an issue skipped after its
   build left behind). This step may run twice (a compaction
   partway through), so it only adds what is missing. For each record, write its `<N>.md`: the record's state wins
   over the file's, except that a file saying `merged` wins (the merge queue writes it before cleanup), and a parked
   record with a `pr` whose `gh pr view <pr> --json state --jq .state` prints `MERGED` is `merged` (a merge agent
   that died after the merge); such an issue's dependents parked for nothing, so say so in the summary. A parked
   or skipped record's `note` is the file's `park-note:`. A line the file already has is replaced by the record's,
   never added twice; every path stays as this file's "State" gives it. Append
   each `unrecorded` finding not already in `<N>-findings.md`. Then record `workflow-done: <runId>`
   (`workflow-done: lost` after a lost run) in `current-session.md`.

## Ending

1. **Fold findings and correct the spec:** only the findings fold and the `merged` spec correction in `leftovers.md`
   "A parallel run's files", for every `<N>-findings.md` and for the divergence rulings of every merged `<N>.md`.
   Apply every merged issue's `.claude/CLAUDE.md` ruling to the main checkout's copy the same way. The worktree
   sort and the deletion of each `<N>.md` wait for step 4, since step 3 still reads them.

   **Shakedown merge.** For each `ready-to-merge` issue, in turn: take the worktree `git worktree list` shows on
   `<branch>` (normally the build agent's, the first path of its `<N>.md` `worktrees:` line; a fix agent's is
   detached, so a push from it has no branch to push); when none holds it, add `.claude/worktrees/merge-<N>` on
   `<branch>` as `merge-queue.md` section 2 step 4 does. Enter it as `/work-backlog` "Worktrees" enters an existing
   one, then run `git pull --ff-only origin <branch>` there, since fix rounds pushed from other worktrees. Then
   `merging.md` steps 0–1 (the CI watch is usually already green, since the build's push started it), its step 2's
   shakedown question on green (show the PR and what its review settled, recommending yes unless the review left
   something open), and steps 2–4, with these differences: the review stage was its review, so it gets no second
   one; a merge-time fix made here is reviewed as `merging.md` step 1 says, since this session can dispatch; step
   3's record is this issue's `<N>.md` `state: merged`, not a `current-session.md` line; step 4 removes every path
   on the `worktrees:` line. Its spec and `.claude/CLAUDE.md` rulings are then applied as above. On no, it stays
   open as this skill's unfinished work for `leftovers.md`, and the summary names it.
2. Run `/work-backlog` step 8 (the walkthrough), skipped as "Merging" says when the batch halted on a failed `gh`
   call.
3. **Summary:** the report `/work-backlog` "End-of-run report" defines, in its report file and in chat, with these
   additions: `Not merged` is led by the batch-level halt cause if there was one, naming the issue whose record's
   note carries it (a merged record may carry it too, when the merge agent's pull of `main` was refused after the
   merge), then every parked issue with its
   note and recommended answer, then every issue a budget skipped and every `ready-to-merge` issue left unmerged,
   each with its note; `Session notes` and `Consumer-impacting changes` take every record's `rulings` line that
   starts `Summary:` (`common.md` "Summary items" lists the kinds); `Leftovers` takes every `leftInPlace` entry and
   every path a merged issue's `worktrees:` line names that still exists (a merge agent that died after the merge).
   A note the script wrote itself (a dependency, a batch halt, a dead agent) carries no question: give it one,
   normally "resume the issue in the next session" once its cause is settled. Write these items to
   `.claude/backlog/parallel/summary.md` before giving them, so step 4's deletions cannot lose them: a serial
   session that runs this ending through `leftovers.md` gives the summary only at its own step 9, which reads that
   file and then deletes it.
4. **Log, then trim or delete `current-session.md`.** First append `done` to the `session-log.md` line the
   `session-date:` line names, as `/work-backlog` step 9 says, while `current-session.md` still holds it. Then sort
   the run's worktrees as `leftovers.md` "A parallel run's files" says. Then, with parked issues, keep the
   `session-date:` and flow lines and, for parked issues only, their issue and piece lines, halt answers, spec
   paths, plan paths (each parked `<N>.md`'s `plan:` line, copied in as `/work-backlog` records plans), allowed
   commands and park notes, without `workflow-*` or `cap:` lines; a later session of either skill resumes them
   through `leftovers.md`. With none, delete it. Delete every `<N>.md` of a merged issue, `result.json` and
   `run-args.json`, and `summary.md` once the summary that reads it has been given: here after step 3; when a
   `/work-backlog` session runs this step through `leftovers.md`, leave it for that session's step 9.

## Never

`/work-backlog`'s "Never", plus:

- Run two merges at once, or let any agent but the merge queue fetch `main`, pull, add or remove a worktree, or
  delete a branch.
- Start a second workflow run for a batch whose run left traces ("The run", step 4).

## Red flags

| Thought | Reality |
| --- | --- |
| "The workflow died; I'll start it again" | Resume it from its own session; from any other, sort it as a lost run. |
| "One issue failed, so stop everything" | Park it; only a batch-level failure ends the run. |
| "This conflict is just code; keep both sides" | Keep-both is for list and registry lines; code is reviewed. |
| "This code conflict is not the batch's; resolve it anyway" | A hunk from outside the batch parks the issue. |
| "The baseline conflicts; merge the numbers" | Regenerate it; a rise needs a `Coverage-Drop:` commit. |
| "I'll enter the worktree to check on it" | The session stays in the main checkout; `git -C` reads it. |
| "The agent should run requesting-code-review" | It has no subagent tool; the review stage is the review. |
