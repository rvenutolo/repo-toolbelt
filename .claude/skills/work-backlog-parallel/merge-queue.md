# Merge queue

Instructions for the merge agent `run.workflow.js` starts once an issue is ready. The script runs one merge agent at
a time, so this agent is the only one touching shared git state: it fetches, pulls `main`, adds and removes
worktrees and deletes branches. It runs in the main checkout (`<main>`), not isolated. "Every agent" in `agents.md`
applies, except its "Shared state is not yours" item: here it is.

`/work-backlog`'s `merging.md` is the procedure; this file names where it differs. The rebase happens in a worktree
of this agent's own, `<main>/.claude/worktrees/merge-<n>`, never in a builder's.

Inputs: `n`, `branch`, `pr`, `worktrees` (every worktree path the issue's agents used), `resolutionFindings` (empty,
or the review of a conflict resolution this agent asked for), `date`, `main`.

## 1. Reconcile

`gh pr view <pr> --json state --jq .state`. `MERGED`: go to step 6 (a resumed merge). `CLOSED`: the user closed it;
return `state: "skipped"`.

## 2. Rebase

1. `git fetch --prune origin`, then note the old remote SHA, `git rev-parse origin/<branch>`.
2. If `merge-<n>` exists and `resolutionFindings` is non-empty, this is the second call for a reviewed resolution:
   keep it and go to step 3. Otherwise remove a leftover `merge-<n>` (`git worktree remove`, path written out), then
   `git worktree add --detach .claude/worktrees/merge-<n> origin/<branch>`.
3. If `git -C <merge worktree> merge-base --is-ancestor origin/main HEAD` succeeds, the branch is current: go to
   step 4. Otherwise `git -C <merge worktree> rebase origin/main`.
4. **Conflicts.** For each conflicted file (in a rebase, `--ours` is `main`, `--theirs` is the branch's commit):
   - **List and registry lines** (`lints.toml`, `formatters.toml`, README lists and tables, `examples/consumer/`
     lint lists): keep both sides' entries, in the order the file keeps (sorted where it is sorted).
   - **`.coverage-baseline.txt`:** take `main`'s version and finish the rebase; it is regenerated after (step 3).
   - **`CHANGELOG.md`:** take `main`'s version. No session edits it.
   - **Anything else whose intent on both sides is clear** from the two commits and their PRs: resolve it, and
     remember the files: this is a code resolution.
   - **Intent unclear:** `git -C <merge worktree> rebase --abort` and park, with the conflicting hunks and both
     PRs' intent in the note.
5. Confirm `git -C <merge worktree> log --format='%G?' origin/main..HEAD` prints only `G`; any other letter parks.

## 3. After the rebase

1. **Baseline.** If the rebase touched `.coverage-baseline.txt`, run `./.ci/in-devshell ./run-tests` then
   `./.ci/in-devshell go run ./cmd/toolbelt update-coverage-baseline` in the merge worktree. A changed file is
   committed as a new signed commit; a unit whose line rises carries the `Coverage-Drop:` trailer exactly as README
   "Recording a rise" says. Never amend.
2. **Code resolution, first call:** return `state: "needs-review"`, `mergeWorktree`, `conflictedFiles`. Do not push.
   The script runs the review stage on the resolution and calls this agent again with `resolutionFindings`.
3. **Code resolution, second call:** handle each of `resolutionFindings` as "Findings about the change itself"
   says, committing in the merge worktree. A finding about the resolution that cannot be fixed parks.
4. **Push** if anything changed: `git -C <merge worktree> push --force-with-lease=<branch>:<old remote SHA> origin
   HEAD:refs/heads/<branch>`. The pre-push hook runs the full gate in the merge worktree; a failure caused by the
   resolution is fixed and pushed again, any other parks.

## 4. CI

`merging.md` step 1, unchanged: watch, read the buckets, fix a real failure in the merge worktree and push, one
rerun for an infrastructure failure. Still red after the rerun parks.

## 5. Merge

`merging.md` step 2, with two differences:

- `BEHIND` or `DIRTY` (a Renovate PR merged meanwhile): go back to step 2 of this file. On the third time round,
  park.
- Every blocked banner it names (`UNSTABLE`, `BLOCKED`, an unknown status) parks the issue, with the status and the
  rule in the note.

The moment `gh pr view` prints `MERGED`, write `state: merged` to `<main>/.claude/backlog/parallel/<n>.md`, before any
cleanup.

## 6. Pull and clean up

1. `git pull --ff-only` in the main checkout. A refusal (the user committed on or dirtied local `main`) is a
   batch-level halt: return `batchHalt` with that cause, after cleanup.
2. `git worktree remove` the merge worktree and every path in `worktrees` that still exists, each written out. Each
   isolated agent's worktree has a branch of its own (`worktree-wf_<runId>-<k>`, or the issue branch for the build
   agent's); read it from `git worktree list` before removing, then `git branch --delete` each, and the issue branch.
   A refused remove or delete is left in place and listed in `leftInPlace`; never `--force` or `-D`.
3. Return `state: "merged"`, `leftInPlace`.

## Parking

On a park, write `state: parked` and `park-note:` to `<main>/.claude/backlog/parallel/<n>.md`, leave the PR open and
every worktree in place (a later session resumes them through `leftovers.md`), and return `state: "parked"` with
the note. A batch-level failure ("Every agent" in `agents.md`) also sets `batchHalt`.
