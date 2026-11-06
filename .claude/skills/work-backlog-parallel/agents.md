# Workflow agents

The Build, Review and Fix sections for the agents `run.workflow.js` starts for each issue; `common.md` comes first
for every one of them, and the merge agent is `merge-queue.md`. Each agent's prompt names its section and passes its
inputs as JSON.

| Role | Runs in | Writes |
| --- | --- | --- |
| Build | its own isolated worktree (`isolation: 'worktree'`) | that worktree, its branch, the PR |
| Review | the main checkout, not isolated | `<main>/.claude/backlog/parallel/`, `<main>/docs/superpowers/plans/` |
| Fix | its own isolated worktree | that worktree, the branch (by push), the PR body |

An isolated worktree starts from `origin/main` as the main checkout last fetched it (Claude Code's default
`worktree.baseRef: fresh`), never from the main checkout's `HEAD`, so the user's local commits never reach it.

## Build

Inputs: `n`, `kind` (`bug` or `feature`), `spec` (repo-relative, read at `<main>/<spec>`; empty for a bug), `piece`
(for an umbrella issue, the spec's piece to build; otherwise empty), `after` (the batch issues this one waited on,
merged before it started), `haltAnswers` (the user's answers to this batch's halt questions; may be empty), `date`,
`main`.

1. **Reconcile.** An earlier agent may have started this issue (a resumed run re-runs agents that were mid-run).
   - `gh pr list --state all --search "#<n> in:body" --json number,state,headRefName,body`, keeping PRs whose body
     ends with `Backlog-Session: <date>` and has a line that is exactly `Closes #<n>` or `Part of #<n>` (the "PRs"
     bullet says why exactly).
   - A `MERGED` one with `Closes #<n>`, or with `Part of #<n>` for this `piece` (its title or body names the piece):
     return `state: "merged"`. A merged `Part of #<n>` PR for another piece is an earlier piece, not this build.
   - An `OPEN` one is a finished build, since a build agent opens its PR only at step 4. Return `state: "ready"` at
     once, building nothing (a bug whose fix is already on the branch would no longer reproduce and park as "not a
     bug"): `branch` its `headRefName`, `pr` its number, `rulings` the body's `Ruling:` lines, no `findings`. `worktree`
     is the dead builder's worktree that `git worktree list` shows on the branch, when one exists (it holds the plan),
     otherwise your own. `plan` is the `plan:` line of `<main>/.claude/backlog/parallel/<n>.md`, otherwise the plan
     under that worktree's `docs/superpowers/plans/`, otherwise empty.
   - Otherwise a local branch from a dead builder: one matching `*/<n>-*` that `git worktree list --porcelain` shows
     checked out in a `.claude/worktrees/wf_*` or `.claude/worktrees/merge-<n>` worktree. A matching branch anywhere
     else is the user's: leave it alone, and park if its name is the one this issue would take.
   - With a branch found: `git fetch origin <branch>` (when the remote has it), then `git switch --detach` to
     whichever of `<branch>` and `FETCH_HEAD` contains the other (`git merge-base --is-ancestor`); if neither does,
     park, since work exists in two places. A branch checked out in a dead builder's worktree cannot be checked out
     here, hence detached. Copy that worktree's plan (`docs/superpowers/plans/`) and ledger
     (`.superpowers/sdd/<plan-basename>/progress.md`) to the same relative paths here, since executing-plans looks
     for the ledger under the current worktree only and would otherwise start over. Work detached from here on and
     push with `git push origin HEAD:refs/heads/<branch>`. Resume the plan from the ledger; work never committed in
     the dead worktree is lost and is redone. With no ledger, the branch's commits beyond `origin/main` say how far
     it got: a feature whose plan tasks are all committed, or a bug with its `fix:` commit, goes straight to step 4.
2. **Fresh start** (nothing found). Read the issue with `gh issue view <n> --json state,title,body,comments`:
   `CLOSED` returns `state: "skipped"`. The comments are part of the issue: a design or a requirement the user
   settled there binds you as the body does. `haltAnswers` binds you too: never re-decide a question it answers.
   Rename the branch with `git branch -m <type>/<n>-<description>` (the "Branches" bullet). Copy
   `<main>/.claude/CLAUDE.md` and `<main>/.claude/rules/` to the same relative paths here, and for a feature
   `<main>/<spec>` as well (`mkdir --parents .claude docs/superpowers/specs docs/superpowers/plans` first); read the
   copied rules that apply to the files you will change. Then run the baseline the "Worktrees" bullet names, before
   changing anything; red is never fixed as part of this issue.
3. **Build.**
   - **Bug:** `bug-flow.md` steps 2–4. "Not a bug" (step 3) parks, with "close as completed" or "close as not
     planned" as the recommended answer; skip step 3's cleanup (no worktree remove or branch delete: the merge
     queue and `leftovers.md` own those).
   - **Feature:** `feature-flow.md` step 6.3, planning only this issue's part of the spec (`piece`, when set), then
     step 6.4 with superpowers:executing-plans (the only executor available: superpowers:subagent-driven-development
     needs a subagent tool this agent lacks). The earlier pieces 6.3 tells you to read are the merged PRs of the
     issues in `after` and the merged `Part of #<n>` PRs. Skip using-git-worktrees' baseline step: step 2 ran it.
     Skip executing-plans' "Final Review" entirely and treat the final review as clean: the review stage replaces
     it, so write no `Final review:` ledger line and send no message about it. Run the ledger sweep (the "Review"
     bullet: in-scope deferred findings are fixed, known-gap-commented or dropped, out-of-scope notes go in
     `findings`, every `Ruling:` goes in `rulings`), then "Finish" except its workspace deletion, which waits for
     step 4: a death before the push would otherwise leave a finished branch with no ledger to resume from. Where
     the merged code of an earlier piece contradicts the spec, the code wins and the divergence is a ruling.
4. **Push and open the PR.** Only here: the "PRs" bullet's early push and early PR do not apply to a build agent,
   because step 1 takes any open PR for a finished build, and a resumed run would merge a half-built one.
   `git push --set-upstream origin <branch>` (or the detached form from step 1). A pre-push gate failure is fixed
   and pushed again, as a failing test would be. Then `gh pr create` per the "PRs" and "PR title" bullets:
   `Closes #<n>`, or `Part of #<n>` for an umbrella piece that is not the spec's last, the rulings (not the
   `Summary:` lines), ending with `Backlog-Session: <date>`. Once the PR exists, delete the plan's workspace
   (`.superpowers/sdd/<plan-basename>/`).
5. **Return** `state: "ready"`, `branch`, `pr`, `plan` (repo-relative, or empty), `worktree`, `rulings`
   (`Summary:` lines included, below), `findings`, `changed: true`.

**`Summary:` lines** are returned as `common.md` "Summary items" says, by Build and Fix alike; the merge agent's are
in `merge-queue.md`.

## Review

Inputs: `n`, `round` (1 or 2), `kind`, `spec` (repo-relative, empty for a bug), `branch`, `pr`, `headWorktree`
(where the latest commit was made), `plan` (the build agent's repo-relative plan path, or empty), `worktrees` (every
worktree path the issue's agents used so far), `build` (the latest build or fix agent's output, verbatim), `main`.

1. **Record.** Write `<main>/.claude/backlog/parallel/<n>.md` (`SKILL.md` "State") from the inputs: `state:
   ready`, `branch:`, `pr:`, `plan:`, `worktrees:`, and every ruling already in the file plus each of
   `build.rulings` not already there. Every path in the file is repo-relative or, for a worktree, absolute; never
   `<main>/` plus a repo-relative path. Append each entry of `build.findings` not already in
   `<main>/.claude/backlog/parallel/<n>-findings.md` to it (a re-run review must not add them twice). The plan comes
   from the `plan` input, never from `build`, whose `plan` a fix agent leaves empty: when it is set and
   `<main>/<plan>` does not exist yet, copy `<headWorktree>/<plan>` there; write `plan: <plan>`.
2. **Review the pushed branch yourself**, following the review prompt inside superpowers:requesting-code-review's
   `code-reviewer.md` with the "Subagents" clause. Do not dispatch the subagent the template wraps, and do not read
   that skill's `SKILL.md`. Fill the template's placeholders:
   - `[DESCRIPTION]`: the PR's title and body (`gh pr view <pr> --json title,body`).
   - `[PLAN_OR_REQUIREMENTS]`: the issue, body and comments (`gh issue view <n> --json title,body,comments`; the
     `--comments` form prints only the comments when not on a terminal) and, for a feature,
     `<main>/<spec>` and `<main>/<plan>`.
   - `[BASE_SHA]`: `git merge-base origin/main origin/<branch>`, after `git fetch origin <branch>` in the main
     checkout (your issue's branch, which "Shared state" in `common.md` allows).
   - `[HEAD_SHA]`: `origin/<branch>`, inspected with `git diff` and `git show` only. The pushed branch, not
     `headWorktree`, is what merges, and a writer's worktree may already be gone.

   The PR body's `Ruling:` lines are settled: a finding one of them already rules on is not raised again unless the
   code since contradicts the ruling. Edit no file in any issue worktree.
3. **Sort** each finding: about the change itself (a hole in this PR's code or tests) goes in `changeFindings`;
   anything else is appended to `<n>-findings.md` and listed in `otherFindings`. Each "Declined to judge" line
   `code-reviewer.md` produces goes in `changeFindings` with its summary starting `Declined to judge:`, so the fix
   agent rules on it. The pre-push gate already ran the tests; the review does not rerun them.
4. **Return** `changeFindings`, `otherFindings`, `recordedWriterFindings` (true once step 1 has appended
   `build.findings`) and `batchHalt` (empty unless a batch-level failure stopped the review). When a halt stops the
   review, `otherFindings` holds only the out-of-scope findings not yet appended to `<n>-findings.md`; the
   controller writes those, and `build.findings` too when `recordedWriterFindings` is false.

**A change made at merge time** (inputs `resolution: true`, `reason`, `failure`, `fixBase`, `mergeWorktree`,
`conflictedFiles`, `branch`, `pr`, `n`, `main`, from `merge-queue.md`): skip step 1 and return
`recordedWriterFindings: true`. Review only that change, with `git -C <mergeWorktree> diff <fixBase> HEAD --
<files>`: `fixBase` is the commit it was made on, so code an earlier review already passed is not reviewed again.

- `reason: conflict`: against what each side's PR meant (`gh pr view <pr>` and the `main` commits that touched
  those files). A wrong or lossy resolution is a `changeFindings` entry.
- `reason: gate` or `ci`: against `failure`, the failing output. A fix that hides the failure instead of curing it,
  or that changes behavior the PR did not set out to change, is a `changeFindings` entry.

A finding outside that change follows step 3 and is not this review's business beyond that.

## Fix

Inputs: `n`, `round`, `kind`, `branch`, `pr`, `spec` (repo-relative, empty for a bug), `plan` (repo-relative, or
empty), `changeFindings`, `date`, `main`.

1. **Reconcile and check out:** `git fetch origin <branch>`, then `git switch --detach FETCH_HEAD`. Fix commits
   carry a `Review-round: <round>` trailer; a commit on the branch with this round's trailer means a resumed fix:
   continue from the findings it does not cover. Copy `<main>/.claude/CLAUDE.md` and `<main>/.claude/rules/` in,
   and for a feature `<main>/<spec>`, as Build step 2 does. For a feature, `<main>/<plan>` says what the change is
   for.
2. **Handle each finding** as the "Findings about the change itself" bullet says: fixed (test first, then fix, as
   separate commits where a test applies), given a known-gap comment, or dropped with a ruling. A
   `Declined to judge:` entry always gets a ruling: the behavior stands, or it is now a finding and handled as one.
   Three failed attempts at one finding park the issue (`sub-skill-overrides.md`, the systematic-debugging entry).
3. **Push**, when you made commits: first add any new rulings, not the `Summary:` lines, to the PR body (`common.md` "PR
   bodies"), then `git push origin HEAD:refs/heads/<branch>` (the pre-push gate runs). With no commits, only the body
   edit.
4. **Return** `state: "ready"`, `worktree`, `rulings`, `findings`, and `changed` (true when you pushed commits).
   With `changed` false, the script skips the next review round: the code it would review is unchanged.
