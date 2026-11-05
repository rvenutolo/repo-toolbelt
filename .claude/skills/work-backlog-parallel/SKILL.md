---
name: work-backlog-parallel
description: >-
  Use when asked to work through this repo's GitHub issue backlog with several issues built at once, for a short
  session or a long unattended batch.
disable-model-invocation: true
---

# Working the backlog in parallel

One run of this skill is one parallel backlog session: the `/work-backlog` session, with independent issues built
concurrently by a workflow and merged one at a time. Everything `/work-backlog` says governs here unless this file
or a file beside it names an exception. Read `.claude/skills/work-backlog/SKILL.md` and its sub-files before step 1;
the names below ("Terms", "State", "The filing bar", "PRs", "Merging" and the rest) are sections of that file.

**After a context compaction**, re-read this file, `/work-backlog`'s `SKILL.md` and `.claude/backlog/current-session.md`.
If it names a `workflow-run`, the run may still be going: go to "The run" before doing anything else.

## How the session runs

All of the user's part comes first, in one sitting: the batch OK, every feature brainstorm, one combined question
round, the permission request and the agent-count estimate (`planning.md`). Then one workflow builds, reviews and
merges the batch with nobody watching. The user comes back to merged PRs, the findings walkthrough and the summary.

Merging stays serial, because the `protect-main` ruleset requires a branch to be up to date with `main` and there is
no GitHub merge queue: every merge puts every other ready branch behind, so each one rebases, re-runs the pre-push
gate and waits for CI in turn. What runs in parallel is building and review. The proposal's wall-clock estimate says
so.

## Named exceptions to `/work-backlog`

- **Issues run concurrently** within the dependency graph `planning.md` builds, up to the cap; merges are serial.
  Replaces "Strictly one issue at a time" ("Doing the work") and its red flag.
- **No chunk size limit.** Replaces "Size" in "Choosing the chunk".
- **Feature batches** get one brainstorm and spec per family, then one combined question round and one permission
  request (`planning.md`).
- **A halt parks the issue and the batch goes on.** Every point where `/work-backlog` or a file it cites would send a
  banner halt during the unattended part (including `bug-flow.md` step 3, the plan-too-broken and three-failed-fixes
  entries of `sub-skill-overrides.md`, and every blocked banner in `merging.md`) instead sets that issue's state to
  `parked` with a one-paragraph note: what happened, what was tried, the question the user would have been asked
  and the recommended answer. Issues that depend on a parked issue park too. Batch-level halts end the run
  ("Merge queue").
- **Rebase conflicts are resolved**, not halted on (`merge-queue.md`). Replaces the conflict blocked banners in
  `merging.md` step 2.
- **Related bugs never share a PR.** Replaces that allowance in "PRs", so parking one bug never leaves its
  `Closes #N` in a PR that merges.
- **Out-of-scope findings go to per-issue files**, never straight to `pending-findings.md`; the controller judges and
  folds them. Replaces the append in "Out-of-scope findings".
- **Worktrees are made by the workflow.** Build and fix agents run in tool-made worktrees
  (`.claude/worktrees/wf_<runId>-<n>`); the merge queue makes `.claude/worktrees/merge-<N>`. Replaces "Worktrees"
  creation and `EnterWorktree`/`ExitWorktree`; branch names still follow "Branches".
- **No nested subagents.** Workflow agents cannot dispatch subagents, so superpowers:subagent-driven-development is
  never chosen, superpowers:executing-plans' final review is skipped (its "With a subagent tool" and "Without a
  subagent tool" paragraphs both), and superpowers:requesting-code-review is replaced by the review stage
  (`agents.md` "Review"), which uses its `code-reviewer.md` with the "Subagents" clause. The two-round cap holds: a
  round is one review stage plus one fix stage.
- **Nothing shared is written mid-run by a builder.** No build or fix agent edits the spec, `.claude/rules/`,
  `current-session.md` or `pending-findings.md` (they cannot: their writes are confined to their own worktree). A
  spec/code divergence (`feature-flow.md` step 6.3) is a `Ruling:` in the PR body; the controller corrects the spec
  at the end. The plan is copied to the main checkout by the review stage.
- **`current-session.md` survives a run with parked issues**, trimmed to them ("Ending"). Replaces its deletion in
  step 9 while anything is parked.

## State

`/work-backlog`'s "State", plus:

- `current-session.md` also holds `cap: <n>`, the batch as `issue <N> after <M>,<K>` lines (or `issue <N>` with no
  prerequisite), `workflow-script: <path>`, `workflow-args: <path>` and `workflow-run: <runId>`.
- `.claude/backlog/parallel/<N>.md`: one issue's progress, written by the review stage and the merge queue during
  the run and completed by the controller after it ("The run" step 5). Lines
  `state: building|ready|merged|parked|skipped` (skipped: the user closed the issue or its PR mid-run), `branch:`,
  `pr:`, `plan:`, `worktrees:` (every worktree path the issue's agents used), `ruling:` (one per ruling) and
  `park-note:`.
- `.claude/backlog/parallel/<N>-findings.md`: the issue's out-of-scope findings, unjudged, in `pending-findings.md`'s
  entry format.
- `.claude/backlog/parallel/run-args.json`: the workflow's `args` ("The run").

## The session

1. **`/work-backlog` steps 1–5:** preconditions, log, unfinished work, leftover findings, stall check. The session
   stays in the main checkout for the whole run and never calls `EnterWorktree`: a session isolated in a worktree is
   refused `git -C` on the main checkout.
2. **Plan the batch** with the user (`planning.md`), then write `current-session.md`.
3. **The run** (below).
4. **Ending** (below): fold the findings, the walkthrough, the summary.

## The run

1. Write `.claude/backlog/parallel/run-args.json` with the workflow's `args`:
   `date` (`YYYY-MM-DD`), `main` (the main checkout's absolute path), `cap`, and `batch`: a list, in proposal order,
   of `{"n": <N>, "kind": "bug" | "feature", "after": [<M>, ...], "spec": "<spec path, or empty for a bug>"}`.
   Record `workflow-script: .claude/skills/work-backlog-parallel/run.workflow.js` and the args path in
   `current-session.md`.
2. Invoke the Workflow tool with `scriptPath` set to the script's absolute path and `args` set to the file's
   contents (as a JSON value, not a string). The moment the tool returns, record `workflow-run: <runId>`.
3. Wait for the completion notification; do not poll. A workflow pauses for a permission prompt, so one prompt
   stalls every issue: that is why the permission request covers every agent's commands.
4. **Resume.** After a compaction with the run still going, keep waiting. If the run is gone (a restart killed it),
   relaunch with the same `scriptPath`, the same `args` and `resumeFromRunId`; finished agents replay from cache and
   every agent prompt starts by reconciling against git and GitHub, so an agent interrupted mid-work picks up where
   its branch is. Never start a fresh run for a batch that has a `workflow-run`.
5. **Record.** The result is `{merged, parked: [{n, note}], skipped, batchHalt, records}`, one record per issue
   with its `state`, `note`, `branch`, `pr`, `worktrees`, `rulings` and `unrecorded` findings (those no review
   stage wrote down). For each record, write its `<N>.md` where the file is missing or says less (a builder that
   parked before any review never had one written), and append its `unrecorded` findings to `<N>-findings.md`.
   Where a file says `merged` and the record does not, the file wins: the merge queue writes it before cleanup.

## Ending

1. **Fold findings.** For each `<N>-findings.md`, judge every entry by "The filing bar"; append the ones that pass to
   `pending-findings.md` and drop the rest. Then delete the folded file by its literal path.
2. **Correct the spec** in the main checkout for every divergence ruling in a merged PR's body.
3. Run `/work-backlog` step 8 (the walkthrough).
4. **Summary:** everything `/work-backlog` step 9 names, led by the batch-level halt cause if there was one, then
   every parked issue with its note and recommended answer.
5. **Trim or delete `current-session.md`.** With parked issues, keep only them: their issue lines, halt answers,
   spec and plan paths, allowed commands and park notes, without `workflow-*` lines; a later session of either skill
   resumes them through `leftovers.md`. With none, delete it. Delete every `<N>.md` of a merged issue. Append `done`
   to today's line in `session-log.md`.

## Never

`/work-backlog`'s "Never", plus:

- Run two merges at once, or let any agent but the merge queue fetch `main`, pull, add or remove a worktree, or
  delete a branch.
- Start a second workflow run for a batch that has a `workflow-run` recorded.
- Text-merge `.coverage-baseline.txt` or edit `CHANGELOG.md`.

## Red flags

| Thought | Reality |
| --- | --- |
| "The workflow died; I'll start it again" | Resume with `resumeFromRunId`; a fresh run redoes and double-pushes. |
| "One issue failed, so stop everything" | Park it; only a batch-level halt ends the run. |
| "This conflict is just code; keep both sides" | Keep-both is for list and registry lines; code is reviewed. |
| "The baseline conflicts; merge the numbers" | Regenerate it; a rise needs a `Coverage-Drop:` commit. |
| "I'll enter the worktree to check on it" | The session stays in the main checkout; `git -C` reads it. |
| "The agent should run requesting-code-review" | It cannot dispatch; the review stage is the review. |
