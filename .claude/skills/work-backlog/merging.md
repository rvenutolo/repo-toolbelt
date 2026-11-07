# Merging

The commands behind "Merging" in `SKILL.md`. Steps 0 to 2 run inside the worktree; step 3 leaves it, and the rest
runs in the main checkout, because a session in a worktree may not write to the main checkout or run git there
("Leaving to write" in `SKILL.md`). The CI watch is a long command ("Long commands" in `SKILL.md`): run it in the
background.

0. **Reconcile** before the first CI watch, and again when a resumed session reaches this file, since the user may
   have acted while nobody watched:
   - `gh pr view <number> --json state --jq .state`: `MERGED` means a resumed session is past the merge: go to step
     3. `CLOSED`: the user closed the PR; leave the issue and its worktree, name both in the summary, and go on.
   - `gh issue view <N> --json state --jq .state` printing `CLOSED`: the user closed the issue; leave the PR open for
     them, name it in the summary, and go on. A PR whose body carries no `Closes #N` or `Part of #N` line (a "fix it
     now") skips this check.
   - `gh pr view <number> --json isDraft,reviewDecision,comments,reviews`: a draft, a non-empty review decision, or
     any comment or review means the user has touched the PR (this skill never comments or reviews on GitHub). Ask
     with the question banner whether to go on with it or leave it, quoting what they wrote, and do not touch it
     before the answer.
1. **Wait for CI.** Before the first watch, `git fetch origin`, then `git merge-base --is-ancestor origin/main HEAD`:
   if it fails the branch is behind `main`, and the ruleset will refuse the merge however CI ends, so take step 2's
   `BEHIND` path now and come back here, one CI run instead of two; it counts as one of step 2's three. Then
   `timeout 30m gh pr checks <number> --watch --fail-fast`, and read the result from
   `gh pr checks <number> --json bucket`, since the exit code alone does not tell the cases apart (a cancelled check
   exits 0). CI is green when every bucket is `pass` or `skipping`. Otherwise:
   - Exit 124 is the `timeout`: CI has not finished in 30 minutes (a queued or hung job). Blocked banner naming the
     checks still `pending`; "Long commands" in `SKILL.md`.
   - A non-zero exit saying `no checks reported on the '<branch>' branch` means no check has started yet, which
     happens right after the PR opens (`--watch` fails the same way; `gh` never prints `[]` here): wait a minute and
     run again, up to three times. Still none means CI did not trigger for this PR (blocked banner, saying so); it is
     not a failed `gh` call, which is a `gh` command that errors.
   - A `fail` bucket is a check to fix: fix the cause ("Merge-time fixes" below) and push again. A coverage-ratchet
     failure is fixed by a test where one is possible, otherwise by a new signed commit (never an amend: it rewrites
     the pushed head, which needs a force push and discards the review and CI that head already had) that raises the
     baseline and carries the `Coverage-Drop:` trailer exactly as README "Recording a rise" says, other trailers
     directly under it.
   - A `cancel` bucket, or a failure that is plainly infrastructure (such as a Nix setup step failing before any
     check runs, or a job that hangs until its timeout), gets one `gh run rerun <run-id> --failed`, with the id from
     `gh run list --branch <branch> --workflow ci.yml --limit 1 --json databaseId --jq '.[0].databaseId'`, then this
     step repeats once from the watch. If CI is still red after the rerun, blocked banner.
   - A non-zero exit with no JSON and any other message is a failed `gh` call (blocked banner).
2. **Merge.** First `gh pr view <number> --json state --jq .state`: `MERGED` means a resumed session is past this step:
   go to step 3 (step 0 also catches this when it runs first). While shakedown is on ("Shakedown" in `SKILL.md`), ask
   first with the question banner whether to merge, naming the PR and its green checks, and record the halt and its
   answer as "Halting" in `resume.md` says. On "no": `ExitWorktree` with `action: keep`, record `pr open, not merged`
   against the
   issue in `current-session.md`, leave the PR, worktree and branch for the next session's step 3, name the PR in
   the summary, and go on with the next issue that does not depend on this one (a dependent would start from `main`
   without it, so it waits for the next session); steps 3 and 4 are the merge path and do not run. The PR stays this
   skill's unfinished work: the next session's step 3 asks once more and, on a second no, leaves it to the user and
   goes on to choose a chunk. Coming back here after
   a `BEHIND` rebase asks again, since the user approved a different head. Then `gh pr merge <number> --merge`: the repo
   allows merge commits only and deletes the remote branch itself. Do not pass `--delete-branch`; the local branch is
   checked out in the worktree and the delete would fail. Confirm with the same `gh pr view` printing `MERGED`, then go
   straight to step 3, which records the merge; a compaction before that record is caught by this `MERGED` check. A
   refused merge is read from `gh pr view <number> --json mergeStateStatus --jq .mergeStateStatus`:
   - `UNKNOWN`: GitHub is still computing the status after a push. Wait a minute and read it again, up to three
     times.
   - `BEHIND`: the ruleset requires the branch to be up to date with `main`, and Renovate merges its own PRs
     unattended, so a branch that was current when CI started can fall behind. Rebase as the global GitHub rule
     says, inside the worktree: `git fetch origin`, note `git rev-parse origin/<branch>` as the old remote SHA,
     `git -c rerere.enabled=false rebase origin/main` (every rebase in both backlog skills carries
     `-c rerere.enabled=false`, and here the abort does too, so one rule covers both: the user's git config enables
     rerere with autoupdate, which replays a recorded resolution and stages it, so a conflict would be resolved and
     hidden before any rule here sees it), regenerate
     the baseline when "Baseline after a rebase" below says so,
     confirm `git log --format='%G?' origin/main..HEAD` prints only `G` (any other letter: do not push, blocked banner),
     push with `timeout 30m git push --force-with-lease=<branch>:<old-remote-sha>` (the pre-push hook runs the full
     gate again, a long command;
     a fix for a gate failure here is a merge-time fix, "Merge-time fixes" below), and go back to step 1 for the new
     CI run. A rebase
     that conflicts is a change the user must see: `git -c rerere.enabled=false rebase --abort`, blocked banner. Never
     `gh pr update-branch`; it rewrites the commits unsigned. The third `BEHIND` or `DIRTY` on one PR is a blocked
     banner: `main` is moving faster than CI, and the user decides when to try again.
   - `DIRTY`: the branch conflicts with `main` (a Renovate merge touched the same file). Take the `BEHIND` path; the
     rebase conflicts and ends in its blocked banner, or resolves cleanly and runs CI again.
   - `UNSTABLE`, or any status not named here: blocked banner naming the status.
   - `BLOCKED` with green CI: a rule other than up-to-date refused it, such as an unresolved review thread (the
     ruleset requires every thread resolved), an unsigned commit, or any other ruleset rule: name the parameter
     `gh api repos/{owner}/{repo}/rules/branches/main` reports for it. Blocked banner, saying which. Never pass
     `--admin`, which `gh` itself suggests, and never resolve a thread; both are the user's.

   **Merge-time fixes.** A fix made after the PR's review (for a `fail` bucket in step 1, or for a pre-push gate
   failure after a rebase in step 2) gets one superpowers:requesting-code-review pass scoped to the fix, with the base
   at the commit the fix was made on, before it is pushed: a base at the last pushed head would, after a rebase, also
   show every commit `main` gained. The fix rounds of "Review" in `SKILL.md` are over by now, so nothing else reviews
   it. The third such fix on one PR is a blocked banner: a change that keeps failing at merge time needs the user.

   **Baseline after a rebase.** A rebase never leaves a text-merged `.coverage-baseline.txt` ("Never" in
   `SKILL.md`). Regenerate it when the rebase took `main`'s copy in a conflict (the case `merge-queue.md`
   "3. Conflicts" produces; this file aborts every conflicting rebase), or when both sides changed it since their
   merge base. Measure that from the branch as it was pushed, since after the rebase the merge base is
   `origin/main` itself: with `base` from `git merge-base origin/main <old-remote-sha>`, both
   `git diff --quiet <base> <old-remote-sha> -- .coverage-baseline.txt` and
   `git diff --quiet <base> origin/main -- .coverage-baseline.txt` exit 1. Skip it when `HEAD` already has a
   commit after `origin/main` that changes the baseline and nothing has been rebased since (an earlier pass
   regenerated it). To regenerate, run `timeout 30m ./run-tests` then
   `./.ci/in-devshell go run ./cmd/toolbelt update-coverage-baseline` ("Long commands" in `SKILL.md`); a changed
   file is committed as a new signed commit, never an amend, and a unit whose line rises carries the
   `Coverage-Drop:` trailer exactly as README "Recording a rise" says.
3. **Leave, record and pull.** `ExitWorktree` with `action: keep` (it cannot remove a worktree entered by `path`).
   From the main checkout, record the merge in `current-session.md` if it is not there yet, so a compaction from
   here on does not lead to a second merge attempt. Then `git pull --ff-only`, so the merge is on local `main`. If
   the pull is refused (the user has committed on or dirtied local `main`), run `git fetch --prune origin` instead,
   leave `main` as the user left it, and say in the summary that the next session's preconditions will fail until
   the user reconciles it; the next worktree starts from `origin/main` either way.
4. **Clean up.** From the main checkout, `git worktree remove .claude/worktrees/<N-description>` and
   `git branch --delete <type/N-description>`. Either may refuse: the delete does when local `main` does not yet
   hold the merge (step 3's refused pull), the remove when the worktree has a stray file. Leave a refused one in
   place and name it in the summary; never pass `--force` or `-D`. The next session's step 3 classifies it once
   `main` is current. A PR built by a parallel run has several worktrees, listed on its `<N>.md` `worktrees:` line:
   remove each by its own path and delete each `worktree-<runId>-<k>` branch they held, as `merge-queue.md` section
   6 does. Record the cleanup in `current-session.md`.
