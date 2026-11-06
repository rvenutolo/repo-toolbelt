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
or a file beside it names an exception, and every exception exists because a workflow builds the issues; any other
rule lives in `/work-backlog` and is cited here. Read `.claude/skills/work-backlog/SKILL.md` and its sub-files before
step 1; the names below ("Terms", "State", "The filing bar", "Merging" and the rest) are sections of that file, and
"PRs", "Permissions", "Long commands" and the like are bullets of its "Doing the work".

**On invocation and after a context compaction**, read `.claude/backlog/current-session.md` first (after a
compaction, re-read this file and `/work-backlog`'s `SKILL.md` too; this paragraph, not that file's "After a context
compaction", says where to go, and the session stays in the main checkout). Take the first that matches:

- **`workflow-done`:** the run is recorded: go to "Ending" and redo each step whose output is missing (findings not
  yet folded, the walkthrough, the summary, the trim).
- **`workflow-result`:** the run finished and its result is on disk: go to "The run", step 5.
- **`workflow-script`:** a run was started and not recorded as finished: go to "The run", step 4 before anything
  else, `/work-backlog` step 1 included. Step 4 alone decides whether steps 2 and 3 run again. `leftovers.md` must
  not take that run's issues over while the run can be resumed.
- **`cap:` and `issue` lines but no `workflow-*` line:** the batch was planned and approved but never launched. Run
  `/work-backlog` step 1, then go to "The run", step 1, under the approval the file records. (`leftovers.md`, run by
  either skill, offers launching it, recommended, or a take-over that strips `cap:`.)

## How the session runs

All of the user's part comes first, in one sitting: the batch OK, every feature brainstorm, one combined question
round, the permission request and the agent-count estimate (`planning.md`). Then one workflow builds, reviews and
merges the batch with nobody watching. The user comes back to merged PRs, the findings walkthrough and the summary.

Merging stays serial, because the `protect-main` ruleset requires a branch to be up to date with `main` and there is
no GitHub merge queue: every merge puts every other ready branch behind, so each one rebases, re-runs the pre-push
gate and waits for CI in turn. What runs in parallel is building and review. The proposal's wall-clock estimate says
so.

A workflow run belongs to the session that started it: its journal lives in that session's directory, and only that
session can resume it. A clean exit hands a running workflow to the session's next wake (`claude --resume
<session>`), which resumes it on its own. A new session cannot resume it at all.

## Named exceptions to `/work-backlog`

- **Issues run concurrently** within the dependency graph `planning.md` builds, up to the cap; merges are serial.
  Replaces "Strictly one issue at a time" ("Doing the work") and its red flag.
- **No chunk size limit.** Replaces "Size" in "Choosing the chunk".
- **A batch may mix bugs and features and hold unrelated families.** Replaces the bugs-first exclusivity and the
  "Features" relatedness rule in "Choosing the chunk"; bugs still come first in the order. An issue's kind is
  decided as "Choosing the chunk" says.
- **Feature batches** get one brainstorm and spec per family, then one combined question round and one permission
  request (`planning.md`). A mixed batch asks its bug halt questions in that round.
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
- **Rebase conflicts are resolved**, not halted on (`merge-queue.md`). Replaces the conflict blocked banners in
  `merging.md` step 2.
- **Out-of-scope findings are never judged mid-run.** Agents return them unjudged and the controller judges and
  folds them at the end, so there is no "Same PR" fix and no known-gap comment for them. Replaces all of
  "Out-of-scope findings".
- **Worktrees are made by the workflow.** Build and fix agents run in tool-made worktrees
  (`.claude/worktrees/wf_<runId>-<k>`, where `k` counts the run's agents, not issues), which start from
  `origin/main`; the merge queue makes `.claude/worktrees/merge-<N>`. Replaces "Worktrees" creation and
  `EnterWorktree`/`ExitWorktree`; branch names still follow "Branches". From step 2 of "The session" until "Ending"
  step 3 begins, neither the controller nor any workflow agent calls `EnterWorktree` or `ExitWorktree`. That also
  replaces "Leaving to write" (every write it would make is returned as `findings` or `rulings`, or made by the
  review stage) and the re-enter clause of "After a context compaction" (the controller follows this file's "On
  invocation" paragraph instead).
- **No nested subagents.** Workflow agents cannot dispatch subagents, so superpowers:subagent-driven-development is
  never chosen, superpowers:executing-plans' final review is skipped (its "With a subagent tool" and "Without a
  subagent tool" paragraphs both), and superpowers:requesting-code-review, wherever `/work-backlog` or `merging.md`
  calls for it, is replaced by the review stage (`agents.md` "Review"), which performs `code-reviewer.md`'s review
  itself. The two-round cap holds: a round is one review stage plus one fix stage.
- **Nothing shared is written mid-run by a builder.** No build or fix agent edits the spec, `.claude/rules/`,
  `current-session.md` or `pending-findings.md` (Write, Edit and git are confined to their worktree, and `common.md`
  forbids shell writes outside it). A spec/code divergence (`feature-flow.md` step 6.3) is a `Ruling:` in the PR
  body; the controller corrects the spec at the end. The plan is copied to the main checkout by the review stage.
- **`current-session.md` survives a run with parked issues**, trimmed to them ("Ending"). Replaces its deletion in
  step 9 while anything is parked.

## State

`/work-backlog`'s "State", plus:

- `current-session.md` also holds `cap: <n>`, the batch as `issue <N> after <M>,<K>` lines (or `issue <N>` with no
  prerequisite), with `piece <name>` after an umbrella issue's, `workflow-script: <path>`, `workflow-args: <path>`,
  `workflow-run: <runId>`, `workflow-result: <path>` once the result is saved, and `workflow-done: <runId>` (or
  `workflow-done: lost`) once it is recorded.
- `.claude/backlog/parallel/result.json`: the workflow's result, verbatim, saved the moment it arrives, so a
  compaction while it is recorded loses nothing.
- `.claude/backlog/parallel/<N>.md`: one issue's progress, written by the review stage and the merge queue during
  the run and completed by the controller after it ("The run" step 5). An issue with no file has not reached its
  first review. Lines `state: ready|merged|parked|skipped` (skipped: the user closed the issue or its PR mid-run),
  `branch:`, `pr:`, `plan:` (repo-relative), `worktrees:` (absolute paths of every worktree the issue's agents
  used), `ruling:` (one per ruling) and `park-note:`.
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

1. Write `.claude/backlog/parallel/run-args.json` with the workflow's `args`: `date` (the date line of
   `current-session.md`, which is the date of the `session-log.md` line this session appended, whatever today is),
   `main` (the main checkout's absolute path), `cap`, and `batch`: a list, in proposal order, of
   `{"n": <N>, "kind": "bug" | "feature", "after": [<M>, ...], "spec": "<path>", "piece": "<name>",
   "haltAnswers": "<text>"}`. `n` and every `after` entry are integers and `after` is always present (empty for
   none). `spec` is the spec's repo-relative path (`docs/superpowers/specs/…`), empty for a bug; confirm each named
   spec exists in the main checkout, since the script cannot. `piece` names an umbrella issue's piece, empty
   otherwise. `haltAnswers` holds the user's answers to the halt questions that bear on that issue, as
   `current-session.md` records them, empty when none do. The script rejects malformed args before any agent
   starts. Record `workflow-script: .claude/skills/work-backlog-parallel/run.workflow.js` and the args path in
   `current-session.md`.
2. **Check before launching.** Nobody will answer a prompt: an agent that meets one holds its slot and stalls its
   issue and every issue that depends on it.
   - **Permissions:** `/work-backlog` "Permissions" (literal-prefix matching, the re-read right before the
     unattended part), against the rule strings `planning.md` "The permission request" names. The run proceeds when
     the session runs with permissions bypassed or every rule string is matched; otherwise ask the user to add the
     missing ones, naming them, or to restart in bypass mode ("On invocation" picks the batch up again). Workflow
     agents run with the session's permission mode and allow rules.
   - **Signing:** the signing-key check in "Permissions".
   - **Local `main`:** `git fetch origin main`, then `git merge-base --is-ancestor HEAD origin/main` in the main
     checkout. Workflow worktrees start from `origin/main`, so the user's local commits never reach a build, but the
     merge queue's pull of `main` would refuse them: if the check fails, ask the user to move them off `main` first.
3. Invoke the Workflow tool with `scriptPath` set to the script's absolute path and `args` set to the file's
   contents (as a JSON value, not a string). The moment the tool returns, record `workflow-run: <runId>`. Then wait
   for the completion notification; do not poll. A run that fails with "run-args.json is invalid" started no agent:
   write no result, correct the file, drop the `workflow-run` line and launch again. Any other result: first write it
   verbatim to `.claude/backlog/parallel/result.json` and record `workflow-result:` with that path, then go to step 5.
4. **Resume.** A compaction does not stop a background workflow, and its completion notification still arrives:
   after one, keep waiting. A session restart is different, and what is possible depends on which session this is.
   The user is present then; ask with the question banner before relaunching, and run step 2 again first, since the
   restart may have changed the permission mode or the signing key.
   - **The run's own session** (resumed with `claude --resume`; `/workflows` lists the run): the wake may already
     have resumed it. If `/workflows` shows it running, keep waiting. Otherwise relaunch with the same
     `scriptPath`, the same `args` and `resumeFromRunId` set to the recorded `workflow-run` (after stopping the old
     run with TaskStop if it still shows as running), and record the run id the relaunch returns in place of the old
     one. Replay is per issue: every finished agent whose issue's earlier calls are unchanged returns its cached
     result, and agents that were mid-run at the stop run again live, safely, because every agent starts by
     reconciling against git and GitHub.
   - **A new session**, or a relaunch the tool rejects ("journal … is not on disk"): the run cannot be resumed.
     Tell the user that resuming the old session with `claude --resume` would have replayed it, then check for
     traces of the run: `gh pr list --state all --search "Backlog-Session: <date> in:body"` and any
     `.claude/worktrees/wf_*`. None: nothing ran; launch from step 2 with the same args. Any: end it as a
     batch-level halt, cause "run lost", go to step 5 with no result, and sort every issue from its files and git as
     `leftovers.md` does.

   Never start a fresh run for a batch whose run left traces.
5. **Record**, from `result.json` (or with no result, after a lost run). The result is
   `{merged, parked: [{n, note}], skipped, batchHalt, records}`, one record per issue with its `state`, `note`,
   `branch`, `pr`, `plan`, `worktrees`, `rulings`, `unrecorded` findings (those no review stage wrote down) and
   `leftInPlace` (worktrees and branches the merge queue could not remove). This step may run twice (a compaction
   partway through), so it only adds what is missing. For each record, write its `<N>.md`: the record's state wins
   over the file's, except that a file saying `merged` wins (the merge queue writes it before cleanup), and a parked
   record with a `pr` whose `gh pr view <pr> --json state --jq .state` prints `MERGED` is `merged` (a merge agent
   that died after the merge). A line the file already has is replaced by the record's, never added twice; every
   path stays as "State" gives it. Such an issue's dependents parked for nothing: say so in the summary. Append each
   `unrecorded` finding not already in `<N>-findings.md`. Then record `workflow-done: <runId>` (`workflow-done:
   lost` after a lost run) in `current-session.md`.

## Ending

1. **Fold findings and correct the spec** as `leftovers.md` "A parallel run's files" says for `<N>-findings.md` and
   for the divergence rulings of a merged `<N>.md`.
2. Run `/work-backlog` step 8 (the walkthrough), skipped as "Merging" says when the batch halted on a failed `gh`
   call.
3. **Summary:** everything `/work-backlog` step 9 names, led by the batch-level halt cause if there was one, then
   every parked issue with its note and recommended answer, then every `leftInPlace` entry and every path a merged
   issue's `worktrees:` line names that still exists (a merge agent that died after the merge). A note the script
   wrote itself (a dependency, a batch halt, a dead agent) carries no question: give it one, normally "resume #N in
   the next session" once its cause is settled.
4. **Log, then trim or delete `current-session.md`.** First append `done` to the `session-log.md` line as
   `/work-backlog` step 9 says, while `current-session.md` still holds the date. Then, with parked issues, keep the date
   and flow lines and, for parked issues only, their issue and piece lines, halt answers, spec paths, plan paths (each
   parked `<N>.md`'s `plan:` line, copied in as `/work-backlog` records plans), allowed commands and park notes, without
   `workflow-*` or `cap:` lines; a later session of either skill resumes them through `leftovers.md`. With none, delete
   it. Delete every `<N>.md` of a merged issue, `result.json` and `run-args.json`.

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
| "The baseline conflicts; merge the numbers" | Regenerate it; a rise needs a `Coverage-Drop:` commit. |
| "I'll enter the worktree to check on it" | The session stays in the main checkout; `git -C` reads it. |
| "The agent should run requesting-code-review" | It cannot dispatch; the review stage is the review. |
