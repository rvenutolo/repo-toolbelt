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

**On invocation and after a context compaction**, read `.claude/backlog/current-session.md` first (after a
compaction, re-read this file and `/work-backlog`'s `SKILL.md` too; this paragraph, not that file's "After a context
compaction", says where to go, and the session stays in the main checkout). Take the first that matches:

- **`workflow-result` and no `workflow-done`:** the run finished and its result is on disk: go to "The run", step 5.
- **`workflow-script` and no `workflow-done`:** a run was started and not recorded as finished: go to "The run", step
  4 before anything else, `/work-backlog` step 1 included. Step 4 alone decides whether steps 2 and 3 run again.
  `leftovers.md` must not take that run's issues over while the run can be resumed.
- **`cap:` and `issue` lines but no `workflow-*` line:** the batch was planned and approved but never launched (the
  usual cause is a restart into bypass mode). Run `/work-backlog` step 1, then go to "The run", step 1, under the
  approval the file records. `leftovers.md` does not resume these issues one by one.

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
- **A batch may mix bugs and features and hold unrelated families.** Replaces the bugs-first exclusivity and the
  "Features" relatedness rule in "Choosing the chunk"; bugs still come first in the order. `planning.md` says how an
  issue's kind is decided.
- **Feature batches** get one brainstorm and spec per family, then one combined question round and one permission
  request (`planning.md`). A mixed batch asks its bug halt questions in that round.
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
  (`.claude/worktrees/wf_<runId>-<k>`, where `k` counts the run's agents, not issues); the merge queue makes
  `.claude/worktrees/merge-<N>`. Replaces "Worktrees" creation and `EnterWorktree`/`ExitWorktree`; branch names
  still follow "Branches". From step 2 of "The session" until "Ending" step 3, neither the controller nor any
  workflow agent calls `EnterWorktree` or `ExitWorktree`. That also replaces "Leaving to write" (every write it
  would make is returned as `findings` or `rulings`, or made by the review stage) and the re-enter clause of "After
  a context compaction" (the controller follows this file's "On invocation" paragraph instead).
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
  prerequisite), `workflow-script: <path>`, `workflow-args: <path>`, `workflow-run: <runId>`,
  `workflow-result: <path>` once the result is saved, and `workflow-done: <runId>` once it is recorded.
- `.claude/backlog/parallel/result.json`: the workflow's result, verbatim, saved the moment it arrives, so a
  compaction while it is recorded loses nothing.
- `.claude/backlog/parallel/<N>.md`: one issue's progress, written by the review stage and the merge queue during
  the run and completed by the controller after it ("The run" step 5). An issue with no file has not reached its
  first review. Lines `state: ready|merged|parked|skipped` (skipped: the user closed the issue or its PR mid-run),
  `branch:`,
  `pr:`, `plan:`, `worktrees:` (every worktree path the issue's agents used), `ruling:` (one per ruling) and
  `park-note:`.
- `.claude/backlog/parallel/<N>-findings.md`: the issue's out-of-scope findings, unjudged, in `pending-findings.md`'s
  entry format.
- `.claude/backlog/parallel/run-args.json`: the workflow's `args` ("The run").

## The session

1. **`/work-backlog` steps 1–5:** preconditions, log, unfinished work, leftover findings, stall check. Leftover work
   found here is finished with `/work-backlog`'s own flow, worktrees included, before planning starts; the session
   is back in the main checkout before step 2. From step 2 on it stays there: a session isolated in a worktree is
   refused `git -C` on the main checkout. Two checks come before planning, so the user's sitting is not wasted:
   - **The Workflow tool is available** (its description states the session's workflow size guideline). If it is
     not, stop with the blocked banner and recommend `/work-backlog`.
   - **The permission mode.** Unless the session runs with permissions bypassed, recommend with the question banner
     restarting in bypass mode now (`planning.md` "The permission request" says why), or going on and adding allow
     rules later. A restart after planning also works ("On invocation"), but costs the user a second visit.
2. **Plan the batch** with the user (`planning.md`), then write `current-session.md`.
3. **The run** (below).
4. **Ending** (below): fold the findings, the walkthrough, the summary.

## The run

1. Write `.claude/backlog/parallel/run-args.json` with the workflow's `args`:
   `date` (`YYYY-MM-DD`), `main` (the main checkout's absolute path), `cap`, and `batch`: a list, in proposal order,
   of `{"n": <N>, "kind": "bug" | "feature", "after": [<M>, ...], "spec": "<path>"}`, where `n` and every `after`
   entry are integers, `after` is always present (empty for none) and `spec` is the spec's repo-relative path
   (`docs/superpowers/specs/…`), empty for a bug. The script rejects malformed args before any agent starts.
   Record `workflow-script: .claude/skills/work-backlog-parallel/run.workflow.js` and the args path in
   `current-session.md`.
2. **Check before launching.** Nobody will answer a prompt, and one prompt stalls every issue.
   - **Permissions.** The run proceeds only when the session runs with permissions bypassed, or when every rule
     string in the permission request (`planning.md`) is matched, literally, by an allow rule in
     `$CLAUDE_CONFIG_DIR/settings.json` or the main checkout's `.claude/settings*.json`, read now. A rule matches a
     command by prefix: `Bash(git rebase *)` does not cover `git -C <path> rebase`. Otherwise ask the user to add
     the missing ones, naming them, or to restart in bypass mode ("On invocation" picks the batch up again).
   - **Signing.** `ssh-add -L` lists the key `git config user.signingkey` names; every commit and rebase overnight
     signs with it. If it does not, or the key was added with a lifetime that could expire during the run, ask the
     user to load it without one.
3. Invoke the Workflow tool with `scriptPath` set to the script's absolute path and `args` set to the file's
   contents (as a JSON value, not a string). The moment the tool returns, record `workflow-run: <runId>`. Then wait
   for the completion notification; do not poll. When it arrives, first write the result verbatim to
   `.claude/backlog/parallel/result.json` and record `workflow-result:` with that path, then go to step 5. A run that
   fails with "run-args.json is invalid" started no agent: correct the file, drop the `workflow-run` line and launch
   again.
4. **Resume.** First, `workflow-done` recorded: the result is in; go to "Ending". `workflow-result` recorded: go to
   step 5. A compaction does not stop a
   background workflow, and its completion notification still arrives: after one, keep waiting. The run is gone
   only when the session itself restarted (the conversation begins anew, and the user has just invoked the skill),
   since background tasks die with their session. The user is present then, so ask with the question banner
   before relaunching, recommending it. Run step 2 again first: the new session may not run with the permissions or
   the signing key the old one had.
   - With a `workflow-run` recorded, relaunch with the same `scriptPath`, the same `args` and `resumeFromRunId`, and
     record the run id the relaunch returns in place of the old one. Replay covers the longest unchanged prefix of
     agent calls; in a concurrent script that prefix ends early, so expect most agents to run again live. That is
     safe because every agent prompt starts by reconciling against git and GitHub, which is the reason those steps
     exist: a build whose PR is open returns at once. If the tool rejects `resumeFromRunId` (the old run's record
     did not survive), treat it as the lost-id case below.
   - With `workflow-script` but no `workflow-run` (the session died between recording the script and recording the
     id), check for traces of a run: `gh pr list --state all --search "Backlog-Session: <date> in:body"` and any
     `.claude/worktrees/wf_*`. None: nothing ran; launch from step 2. Any: the run's id is lost, so end it as a
     batch-level halt, cause "run id lost", go to step 5 with no result, and sort every issue from its files and git
     as `leftovers.md` does.

   Never start a fresh run for a batch whose run left traces.
5. **Record**, from `result.json` (or with no result, after a lost run id). The result is
   `{merged, parked: [{n, note}], skipped, batchHalt, records}`, one record per issue
   with its `state`, `note`, `branch`, `pr`, `plan`, `worktrees`, `rulings`, `unrecorded` findings (those no review
   stage wrote down) and `leftInPlace` (worktrees and branches the merge queue could not remove). For each record,
   write its `<N>.md`: the record's state wins over the file's, except that a file saying `merged` wins (the merge
   queue writes it before cleanup), and a parked record with a `pr` whose `gh pr view <pr> --json state --jq .state`
   prints `MERGED` is `merged` (a merge agent that died after the merge); keep the file's other lines and add the
   record's. Such an issue's dependents parked for nothing: say so in the summary. Append its `unrecorded` findings to
   `<N>-findings.md`. Then record `workflow-done: <runId>` in `current-session.md`, so a compaction during "Ending"
   does not wait for a notification that already came.

## Ending

1. **Fold findings.** For each `<N>-findings.md`, judge every entry by "The filing bar"; append the ones that pass to
   `pending-findings.md` and drop the rest. Then delete the folded file by its literal path.
2. **Correct the spec** in the main checkout for every divergence ruling in a merged PR's body.
3. Run `/work-backlog` step 8 (the walkthrough). When the batch halted because `gh` failed, skip it and say so: the
   findings wait in `pending-findings.md` for the next session.
4. **Summary:** everything `/work-backlog` step 9 names, led by the batch-level halt cause if there was one, then
   every parked issue with its note and recommended answer, then every `leftInPlace` entry and every path a merged
   issue's `worktrees:` line names that still exists (a merge agent that died after the merge). A note the script wrote
   itself (a dependency, a batch halt, a dead agent) carries no question: give it one, normally "resume #N in the
   next session" once its cause is settled.
5. **Trim or delete `current-session.md`.** With parked issues, keep the date and flow lines and, for parked issues
   only, their issue lines, halt answers, spec paths, plan paths (each parked `<N>.md`'s `plan:` line, copied in as
   `/work-backlog` records plans), allowed commands and park notes, without `workflow-*` lines; a later session of
   either skill resumes them through `leftovers.md`. With none, delete it. Delete every `<N>.md` of a merged issue,
   and `result.json`. Append `done` to the `session-log.md` line whose date is the `date` in `run-args.json`: a long
   run crosses midnight, so today's date may name no line.

## Never

`/work-backlog`'s "Never", plus:

- Run two merges at once, or let any agent but the merge queue fetch `main`, pull, add or remove a worktree, or
  delete a branch.
- Start a second workflow run for a batch whose run left traces ("The run", step 4).
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
