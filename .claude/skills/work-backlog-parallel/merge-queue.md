# Merge queue

Instructions for the merge agent `run.workflow.js` starts once an issue is ready; `common.md` comes first. The
script runs one merge agent at a time, so this agent is the only one touching shared git state: it fetches, pulls
`main`, adds and removes worktrees and deletes branches. It runs in the main checkout (`<main>`), not isolated, and
works in other worktrees with `git -C <path>` and absolute paths. `common.md` applies except the git-command limits
of its "Shared state is not yours" item: fetching, pulling, worktrees and branch deletes are this agent's. It still
never writes `current-session.md`, `pending-findings.md`, a spec or `.claude/rules/`, and never calls `ExitWorktree`
or `EnterWorktree`.

`/work-backlog`'s `merging.md` steps 0–2 are the procedure, with the differences below: a blocked or question banner
there is a park here, with the status and the rule in the note. Its steps 3 and 4 (leaving a worktree, recording in
`current-session.md`, the pull and cleanup) never apply: sections 5 and 6 here replace them. Git commands in the
rebase worktree run as `git -C <rebase worktree> …`, rebases as `git -C <rebase worktree> -c core.editor=true
rebase …`, so `rebase --continue` never waits on an editor nobody will close. Devshell commands run there as
`cd <rebase worktree> && ./.ci/in-devshell …`. Long commands (the push, the baseline regeneration and its tests,
`gh pr checks --watch`) follow `common.md` "Long commands", with their status files in the rebase worktree's
`.claude/tmp/`, never the main checkout's.

Inputs: `n`, `branch`, `pr`, `worktrees` (every worktree path the issue's agents used, the build agent's first),
`resolutionReviewed` (true when the previous call returned `needs-review` and the review has run), `resolutionFindings`
(that review's findings, possibly none), `pendingRulings` (the rulings earlier calls for this issue returned with
`needs-review` (`Summary:` lines excluded); a call that stops for review may not have pushed, so they may not be in the
PR body yet), `date`, `main`. Nothing else carries over between calls: what a call needs from an earlier one (the old
remote SHA, the rebase worktree) it reads again from git.

## 1. Reconcile

`merging.md` step 0, with two differences: the issue or PR closed by the user returns `state: "skipped"`, leaving
the PR open; a PR the user touched parks, quoting what they wrote, with "resume the merge" or "leave it" as the
question (no agent of this run comments or reviews on GitHub).

A PR already `MERGED` is a resumed merge whose agent died after the merge: write `state: merged` to
`<main>/.claude/backlog/parallel/<n>.md`, take as the rebase worktree whichever worktree `git worktree list` shows
on `<branch>` (if any), and go to section 6.

## 2. The rebase worktree

The rebase happens on the issue branch itself, so that after the merge the local branch holds the merged commits and
`git branch --delete` accepts it.

1. `git fetch --prune origin`, then note the old remote SHA, `git rev-parse origin/<branch>`, for the
   force-with-lease in section 3. It is the pre-rebase head, except after a call that returned `reason: "ci"`: that
   call had already pushed the rebased branch, so it is that push.
2. The rebase worktree is the one `git worktree list` shows on `<branch>`: normally the build agent's (the first of
   `worktrees`). If a rebase is in progress there (`git -C <it> rebase --abort` succeeds), an earlier merge agent
   died mid-rebase: carry on from the clean branch.
3. **A reviewed change** (`resolutionReviewed` true): the rebase worktree holds the previous call's work, not yet on
   the remote. After `reason: "conflict"` or `"gate"` that is the whole rebased branch, which diverges from
   `origin/<branch>`; after `reason: "ci"` the rebase was pushed and only the fix is local, so the worktree is ahead
   of `origin/<branch>`. Either way this is expected. First handle each of `resolutionFindings` as the
   "Findings about the change itself" bullet says, committing in the rebase worktree (a finding that cannot be
   fixed parks), whatever happens next. Then, if `git -C <rebase worktree> merge-base --is-ancestor origin/main
   HEAD` succeeds, `main` has not moved: go to section 3, step 1. If it has, rebase again from step 5; the review
   covered only the change it saw, so a new code resolution goes back to review.
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
- **`.coverage-baseline.txt`:** take `main`'s version (`git -C <rebase worktree> checkout --ours <file>`); it is
  regenerated after the rebase.
- **`CHANGELOG.md`:** take `main`'s version. No session edits it.

These three rules hold whoever made the `main` side: they lose nothing, so a Renovate bump or the user's own commit
is handled the same way as a batch PR.

- **Anything else, between two of this batch's own PRs:** every commit since the branch's base that wrote `main`'s
  side of the conflicting hunk belongs to a PR whose body ends with `Backlog-Session: <date>`. Find them with
  `git -C <rebase worktree> blame -L <first>,<last> origin/main -- <file>`, where `<first>,<last>` is the hunk's
  line range in `origin/main`'s version, keeping the commits after
  `git -C <rebase worktree> merge-base origin/<branch> origin/main`; map each to its PR with `gh api
  repos/{owner}/{repo}/commits/<sha>/pulls --jq '.[].body'`. Test the hunk, not the file: an outside commit
  elsewhere in the same file did not make this conflict. When the intent on both sides is clear from the two commits
  and their PRs (README prose included), resolve it and remember the file: this is a code resolution. The batch's
  own concurrency made this conflict, so the run settles it, behind the review stage.
- **Any other conflict**, or intent unclear: `git -C <rebase worktree> rebase --abort` and park, with the
  conflicting hunks and both sides' intent in the note. A conflict with work from outside the batch is one
  `/work-backlog` would stop on too (`merging.md` step 2), and the park is this run's stop.

Then:

1. Confirm `git -C <rebase worktree> log --format='%G?' origin/main..HEAD` prints only `G`; any other letter is
   commit signing failing, a batch-level failure (`common.md`).
2. **Baseline:** `merging.md` step 2, "Baseline after a rebase", in the rebase worktree.
3. **Code resolution:** when a rebase in this call made one, return `state: "needs-review"`, `reason: "conflict"`,
   `fixBase` (the `origin/main` commit the rebase went onto), `mergeWorktree` (the rebase worktree) and
   `conflictedFiles`. Do not push. The script runs the review stage on
   it and calls this agent again (section 2, step 3).
4. **Push** if the branch changed: first the PR body's new rulings ("Rulings and findings"), `pendingRulings`
   included, then
   `git -C <rebase worktree> push --force-with-lease=<branch>:<old remote SHA> origin <branch>`. The pre-push hook
   runs the full gate in the rebase worktree. A failure that comes from combining the branch with the new `main` (a
   test or manifest that expects the old set of lints, say) is the merge-time fix `merging.md` step 1 describes:
   note `HEAD`, commit the fix, then return `needs-review` with `reason: "gate"`, the noted commit as `fixBase`, the
   gate's failing output (trimmed to what shows the failure) as `failure`, and the fixed files as `conflictedFiles`.
   Any other failure parks.

## 4. CI

`merging.md` step 1, the watch run as `common.md` "Long commands" says: watch, read the buckets, one rerun for an
infrastructure failure, still red after the rerun parks, and CI that did not trigger parks. A real failure is the
merge-time fix it describes, reviewed by the review stage in place of superpowers:requesting-code-review: note `HEAD`,
commit the fix in the rebase worktree, do not push, and return `needs-review` with `reason: "ci"`, the noted commit as
`fixBase`, the failing job's output as `failure`, and the fixed files as `conflictedFiles`. Every merge-time change gets
one review stage; the fixes for that review's findings (section 2, step 3) are not reviewed again, as `/work-backlog`
does not review fixes after its last review round. A CI rerun is a `Summary:` line ("Rulings and findings").

## 5. Merge

`merging.md` step 2, with two differences:

- Where it says to go to step 3 (after `MERGED`), go to section 6 here.
- Where it says to rebase on `BEHIND` or `DIRTY`, go back to section 2, step 1; its limit on how often that happens
  holds within this call. Nothing counts across calls; the script's three merge calls per issue bound the rest.

The moment `gh pr view` prints `MERGED`, write `state: merged` to `<main>/.claude/backlog/parallel/<n>.md`, before
any cleanup.

## 6. Pull and clean up

1. `git pull --ff-only` in the main checkout. A refusal (the user committed on or dirtied local `main`) is a
   batch-level halt: return `batchHalt` with that cause, after cleanup.
2. List `git worktree list --porcelain` first. For every path in `worktrees` that still exists, plus the rebase
   worktree (a dead builder's, from an earlier run, is not in `worktrees`) and `merge-<n>`: note its branch (a fix
   agent's worktree is detached; its own branch is `worktree-<directory name>`, as
   `git branch --list 'worktree-wf_*'` shows), then `git worktree remove <path>`, each written out. A locked one
   is unlocked with `git worktree unlock <path>` only when its lock PID is dead (`/work-backlog` `leftovers.md`
   "Locked worktrees" says how to tell); a live one is left in place.
3. `git branch --delete` the issue branch and each noted `worktree-wf_*` branch. A refused remove or delete is left
   in place and listed in `leftInPlace`; never `--force` or `-D`.
4. Return `state: "merged"`, `leftInPlace`.

## Rulings and findings

Every ruling this agent makes (a resolution finding dropped, a conflict resolved one way) goes in `rulings` and,
before the push in section 3, step 4, into the PR body (`common.md` "PR bodies"), with each of `pendingRulings` the
body does not have yet. A CI rerun, an outdated superpowers phrase or a settled copied-rule conflict goes in
`rulings` as a `Summary:` line (`common.md` "Summary items"), never in the PR body. Every out-of-scope finding
goes in `findings`, unjudged; the script hands both to the controller.

## Parking

On a park, write `state: parked` and `park-note:` to `<main>/.claude/backlog/parallel/<n>.md`, leave the PR open and
every worktree in place (a later session resumes them through `leftovers.md`), and return `state: "parked"` with
the note. If a rebase is in progress, `git -C <rebase worktree> rebase --abort` first, so the worktree is left on its
branch. A batch-level failure (`common.md`) also sets `batchHalt`.

When the rebase worktree's `HEAD` differs from `origin/<branch>` at the park, end the note with "The worktree
`<path>` holds an unpushed, rebased `<branch>` that diverges from `origin/<branch>`.", and add "A resolution review
passed it." when `resolutionReviewed` is true and every commit since that review handles one of
`resolutionFindings`. `leftovers.md` keys its recommendation on those two sentences.

Whatever the state returned, set `mergeWorktree` to the rebase worktree whenever section 2 chose or added one, so a
`merge-<n>` this call made reaches the issue's record; `reason`, `failure` and `fixBase` are empty unless the state
is `needs-review`.
