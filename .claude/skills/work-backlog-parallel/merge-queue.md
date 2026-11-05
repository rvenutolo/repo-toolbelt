# Merge queue

Instructions for the merge agent `run.workflow.js` starts once an issue is ready. The script runs one merge agent at
a time, so this agent is the only one touching shared git state: it fetches, pulls `main`, adds and removes
worktrees and deletes branches. It runs in the main checkout (`<main>`), not isolated, and works in other worktrees
with `git -C <path>` and absolute paths. "Every agent" in `agents.md` applies, except the git-command limits of its
"Shared state is not yours" item: fetching, pulling, worktrees and branch deletes are this agent's. It still never
writes `current-session.md`, `pending-findings.md`, a spec or `.claude/rules/`, and never calls `ExitWorktree` or
`EnterWorktree`.

`/work-backlog`'s `merging.md` steps 1 and 2 are the procedure, with the differences below; its steps 3 and 4
(leaving a worktree, recording in `current-session.md`, the pull and cleanup) never apply: sections 5 and 6 here
replace them. Rebases run as `git -C <rebase worktree> -c core.editor=true rebase …`, so `rebase --continue` never
waits on an editor nobody will close, and one allow rule, `Bash(git -C *)`, covers every command here.

Inputs: `n`, `branch`, `pr`, `worktrees` (every worktree path the issue's agents used, the build agent's first),
`resolutionReviewed` (true when the previous call returned `needs-review` and the review has run),
`resolutionFindings` (that review's findings, possibly none), `date`, `main`.

## 1. Reconcile

`gh pr view <pr> --json state --jq .state`. `MERGED`: go to section 6 (a resumed merge). `CLOSED`: the user closed
it; return `state: "skipped"`. Then two checks of what the user did while nobody watched:

- `gh issue view <n> --json state --jq .state` printing `CLOSED`: the user closed the issue; return
  `state: "skipped"`, leaving the PR open for them.
- `gh pr view <pr> --json isDraft,reviewDecision,comments,reviews`: a draft, a non-empty review decision, or any
  comment or review means the user has touched the PR (no agent of this run comments or reviews on GitHub). Park,
  quoting what they wrote, with "resume the merge" or "leave it" as the question.

## 2. The rebase worktree

The rebase happens on the issue branch itself, so that after the merge the local branch holds the merged commits and
`git branch --delete` accepts it.

1. `git fetch --prune origin`, then note the old remote SHA, `git rev-parse origin/<branch>`.
2. The rebase worktree is the one `git worktree list` shows on `<branch>`: normally the build agent's (the first of
   `worktrees`). If a rebase is in progress there (`git -C <it> rebase --abort` succeeds), an earlier merge agent
   died mid-rebase: carry on from the clean branch.
3. **A reviewed resolution** (`resolutionReviewed` true): the rebase worktree holds the rebased, unpushed branch
   from the previous call, which diverges from `origin/<branch>` by design. If
   `git -C <rebase worktree> merge-base --is-ancestor origin/main HEAD` succeeds, `main` has not moved: go to
   section 3, step 3. If it has, rebase again from step 5; `resolutionReviewed` covered only the resolution that was
   reviewed, so a new code conflict goes back to review.
4. **Otherwise**, bring the local branch up to `origin/<branch>` (fix rounds pushed from other worktrees, so it is
   usually behind): `git -C <rebase worktree> status --porcelain` must show no tracked change and
   `git -C <rebase worktree> merge --ff-only origin/<branch>` must succeed. When either fails but the worktree's
   `HEAD` is an ancestor of `origin/<branch>` (a dead builder left files behind; everything it committed was
   pushed), free the branch with `git -C <that worktree> switch --detach`, which keeps its files, and add a clean
   one: `git worktree add .claude/worktrees/merge-<n> <branch>` (the path written out), then the `--ff-only` there.
   When no worktree holds `<branch>`, add `merge-<n>` the same way. When the worktree holds commits
   `origin/<branch>` lacks, park: work exists only locally.
5. If `git -C <rebase worktree> merge-base --is-ancestor origin/main HEAD` succeeds, the branch is current: go to
   section 3, step 1. Otherwise `git -C <rebase worktree> -c core.editor=true rebase origin/main`.

## 3. Conflicts

For each conflicted file (in a rebase, `--ours` is `main`, `--theirs` is the branch's commit), resolve it, then
`git -C <rebase worktree> add <file>` and `git -C <rebase worktree> -c core.editor=true rebase --continue`, until
the rebase ends:

- **List and registry lines** (`lints.toml`, `formatters.toml`, README lists and tables, `examples/consumer/` lint
  lists): keep both sides' entries. Where the file is sorted, keep it sorted; where it is in order of addition, put
  `main`'s entries first, since they landed first.
- **`.coverage-baseline.txt`:** take `main`'s version (`git checkout --ours`); it is regenerated after the rebase.
- **`CHANGELOG.md`:** take `main`'s version. No session edits it.
- **Anything else whose intent on both sides is clear** from the two commits and their PRs (README prose included):
  resolve it, and remember the file: this is a code resolution.
- **Intent unclear:** `git -C <rebase worktree> rebase --abort` and park, with the conflicting hunks and both PRs'
  intent in the note.

Then:

1. Confirm `git -C <rebase worktree> log --format='%G?' origin/main..HEAD` prints only `G`; any other letter parks.
2. **Baseline.** Regenerate it when this call took `main`'s `.coverage-baseline.txt` in a conflict, or when both
   `main` and the branch changed it since their merge base (a clean text merge of the baseline is still a merge,
   which "Never" forbids). Measure that from the branch as it was pushed, since after the rebase the merge base is
   `origin/main` itself: with `base` from `git merge-base origin/main <old remote SHA>`, both
   `git diff --quiet <base> <old remote SHA> -- .coverage-baseline.txt` and
   `git diff --quiet <base> origin/main -- .coverage-baseline.txt` exit 1. To regenerate, run
   `./.ci/in-devshell ./run-tests` then `./.ci/in-devshell go run ./cmd/toolbelt update-coverage-baseline` in the
   rebase worktree. A changed file is committed as a new signed commit; a unit whose line rises carries the
   `Coverage-Drop:` trailer exactly as README "Recording a rise" says. Never amend.
3. **Code resolution:** when a rebase in this call made one, return `state: "needs-review"`, `mergeWorktree` (the
   rebase worktree), `conflictedFiles`. Do not push. The script runs the review stage on the resolution and calls
   this agent again. On that call (`resolutionReviewed` true, arriving here from section 2, step 3), handle each of
   `resolutionFindings` as "Findings about the change itself" says, committing in the rebase worktree (a finding
   that cannot be fixed parks), then run steps 1 and 2 again for those commits before step 4.
4. **Push** if the branch changed: `git -C <rebase worktree> push --force-with-lease=<branch>:<old remote SHA>
   origin <branch>`. The pre-push hook runs the full gate in the rebase worktree. A failure that comes from combining
   the branch with the new `main` (a test or manifest that expects the old set of lints, say) is fixed like a code
   resolution: commit the fix, then return `needs-review` with the fixed files as `conflictedFiles`. Any other
   failure parks.

## 4. CI

`merging.md` step 1: watch, read the buckets, one rerun for an infrastructure failure, still red after the rerun
parks. No check started after its three waits parks the issue (CI did not trigger for this PR); only a `gh` call that
errors is batch-level. A real failure gets a fix only through review: commit the fix in the rebase worktree, do not
push, and return `needs-review` with the fixed files as `conflictedFiles`, as section 3, step 4 does for the gate. A code
change never reaches `main` without a review.

## 5. Merge

`merging.md` step 2, with three differences:

- Where it says to go to step 3 (after `MERGED`, or when a resumed call finds the PR merged), go to section 6 here.

- `BEHIND` or `DIRTY` (a Renovate PR merged meanwhile): go back to section 2, step 1. On the third time round, park.
- Every blocked banner it names (`UNSTABLE`, `BLOCKED`, an unknown status) parks the issue, with the status and the
  rule in the note.

The moment `gh pr view` prints `MERGED`, write `state: merged` to `<main>/.claude/backlog/parallel/<n>.md`, before any
cleanup.

## 6. Pull and clean up

1. `git pull --ff-only` in the main checkout. A refusal (the user committed on or dirtied local `main`) is a
   batch-level halt: return `batchHalt` with that cause, after cleanup.
2. List `git worktree list --porcelain` first. For every path in `worktrees` that still exists, plus the rebase
   worktree of section 2 (a dead builder's, from an earlier run, is not in `worktrees`) and `merge-<n>`:
   note its branch (a fix agent's worktree is detached; its own branch is `worktree-<directory name>`, as
   `git branch --list 'worktree-wf_*'` shows), then `git worktree remove <path>`, each written out. A locked one
   whose agent has finished is unlocked first with `git worktree unlock <path>`.
3. `git branch --delete` the issue branch and each noted `worktree-wf_*` branch. A refused remove or delete is left
   in place and listed in `leftInPlace`; never `--force` or `-D`.
4. Return `state: "merged"`, `leftInPlace`.

## Rulings and findings

Every ruling this agent makes (a resolution finding dropped, a conflict resolved one way) goes in `rulings` and, before
the push in section 3, step 4, into the PR body with `gh pr edit <pr> --body`. Every out-of-scope finding goes in
`findings`, unjudged; the script hands both to the controller.

## Parking

On a park, write `state: parked` and `park-note:` to `<main>/.claude/backlog/parallel/<n>.md`, leave the PR open and
every worktree in place (a later session resumes them through `leftovers.md`), and return `state: "parked"` with
the note. If a rebase is in progress, `git -C <rebase worktree> rebase --abort` first, so the worktree is left on its
branch. A batch-level failure ("Every agent" in `agents.md`) also sets `batchHalt`.

Whatever the state returned, set `mergeWorktree` to the rebase worktree whenever section 2 chose or added one, so a
`merge-<n>` this call made reaches the issue's record.
