# Merge queue

Instructions for the merge agent `run.workflow.js` starts once an issue is ready. The script runs one merge agent at
a time, so this agent is the only one touching shared git state: it fetches, pulls `main`, adds and removes
worktrees and deletes branches. It runs in the main checkout (`<main>`), not isolated, and works in other worktrees
with `git -C <path>` and absolute paths. "Every agent" in `agents.md` applies, except its "Shared state is not
yours" item: here it is.

`/work-backlog`'s `merging.md` is the procedure; this file names where it differs.

Inputs: `n`, `branch`, `pr`, `worktrees` (every worktree path the issue's agents used, the build agent's first),
`resolutionReviewed` (true when the previous call returned `needs-review` and the review has run),
`resolutionFindings` (that review's findings, possibly none), `date`, `main`.

## 1. Reconcile

`gh pr view <pr> --json state --jq .state`. `MERGED`: go to step 6 (a resumed merge). `CLOSED`: the user closed it;
return `state: "skipped"`.

## 2. The rebase worktree

The rebase happens on the issue branch itself, so that after the merge the local branch holds the merged commits and
`git branch --delete` accepts it.

1. `git fetch --prune origin`, then note the old remote SHA, `git rev-parse origin/<branch>`.
2. The rebase worktree is the one `git worktree list` shows on `<branch>`: normally the build agent's (the first of
   `worktrees`). If none holds it (its builder's worktree is gone), add one:
   `git worktree add .claude/worktrees/merge-<n> <branch>` (the path written out).
3. `git -C <rebase worktree> status --porcelain` must show no tracked change, and
   `git -C <rebase worktree> merge --ff-only origin/<branch>` must succeed (fix rounds pushed from other worktrees,
   so the local branch is usually behind). Either failing means work exists only locally: park, naming it.
4. **A reviewed resolution** (`resolutionReviewed` true): if the branch is already rebased onto the current
   `origin/main` and only the review's findings are left, go to section 3, step 3. If `main` moved again since, carry
   on here as for a first call: `resolutionReviewed` covers only the resolution that was reviewed.
5. If `git -C <rebase worktree> merge-base --is-ancestor origin/main HEAD` succeeds, the branch is current: go to
   section 3, step 4. Otherwise `GIT_EDITOR=true git -C <rebase worktree> rebase origin/main` (the editor setting
   keeps `rebase --continue` from waiting on an editor nobody will close).

## 3. Conflicts

For each conflicted file (in a rebase, `--ours` is `main`, `--theirs` is the branch's commit), resolve it, then
`git -C <rebase worktree> add <file>` and `GIT_EDITOR=true git -C <rebase worktree> rebase --continue`, until the
rebase ends:

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
2. **Baseline.** If both `main` and the branch changed `.coverage-baseline.txt` since their merge base (conflicted or
   merged cleanly: a clean text merge of the baseline is still a merge, which "Never" forbids), run
   `./.ci/in-devshell ./run-tests` then `./.ci/in-devshell go run ./cmd/toolbelt update-coverage-baseline` in the
   rebase worktree. A changed file is committed as a new signed commit; a unit whose line rises carries the
   `Coverage-Drop:` trailer exactly as README "Recording a rise" says. Never amend.
3. **Code resolution:** when this rebase made one and `resolutionReviewed` is false, return
   `state: "needs-review"`, `mergeWorktree` (the rebase worktree), `conflictedFiles`. Do not push. The script runs the
   review stage on the resolution and calls this agent again. When `resolutionReviewed` is true, handle each of
   `resolutionFindings` as "Findings about the change itself" says, committing in the rebase worktree; a finding
   that cannot be fixed parks.
4. **Push** if the branch changed: `git -C <rebase worktree> push --force-with-lease=<branch>:<old remote SHA>
   origin <branch>`. The pre-push hook runs the full gate in the rebase worktree. A failure that comes from combining
   the branch with the new `main` (a test or manifest that expects the old set of lints, say) is fixed like a code
   resolution: commit the fix, then return `needs-review` with the fixed files as `conflictedFiles`. Any other
   failure parks.

## 4. CI

`merging.md` step 1, unchanged: watch, read the buckets, fix a real failure in the rebase worktree and push, one
rerun for an infrastructure failure. Still red after the rerun parks.

## 5. Merge

`merging.md` step 2, with two differences:

- `BEHIND` or `DIRTY` (a Renovate PR merged meanwhile): go back to section 2, step 1. On the third time round, park.
- Every blocked banner it names (`UNSTABLE`, `BLOCKED`, an unknown status) parks the issue, with the status and the
  rule in the note.

The moment `gh pr view` prints `MERGED`, write `state: merged` to `<main>/.claude/backlog/parallel/<n>.md`, before any
cleanup.

## 6. Pull and clean up

1. `git pull --ff-only` in the main checkout. A refusal (the user committed on or dirtied local `main`) is a
   batch-level halt: return `batchHalt` with that cause, after cleanup.
2. List `git worktree list --porcelain` first. For every path in `worktrees` that still exists, plus `merge-<n>`:
   note its branch (a fix agent's worktree is detached; its own branch is `worktree-<directory name>`, as
   `git branch --list 'worktree-wf_*'` shows), then `git worktree remove <path>`, each written out. A locked one
   whose agent has finished is unlocked first with `git worktree unlock <path>`.
3. `git branch --delete` the issue branch and each noted `worktree-wf_*` branch. A refused remove or delete is left
   in place and listed in `leftInPlace`; never `--force` or `-D`.
4. Return `state: "merged"`, `leftInPlace`.

## Parking

On a park, write `state: parked` and `park-note:` to `<main>/.claude/backlog/parallel/<n>.md`, leave the PR open and
every worktree in place (a later session resumes them through `leftovers.md`), and return `state: "parked"` with
the note. If a rebase is in progress, `git -C <rebase worktree> rebase --abort` first, so the worktree is left on its
branch. A batch-level failure ("Every agent" in `agents.md`) also sets `batchHalt`.
