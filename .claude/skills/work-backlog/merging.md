# Merging

The commands behind "Merging" in `SKILL.md`. Every command runs inside the worktree unless it names the main checkout
with `-C`. The CI watch is a long command ("Long commands" in `SKILL.md`): run it in the background.

1. **Wait for CI** with `gh pr checks <number> --watch --fail-fast`, then read the result from
   `gh pr checks <number> --json bucket`, since the exit code alone does not tell the cases apart (a cancelled check
   exits 0). CI is green when every bucket is `pass` or `skipping`. Otherwise:
   - `[]` means no checks reported yet, which happens right after the PR opens: wait a minute and run again, up to
     three times; still none is a failed `gh` call (blocked banner).
   - A `fail` bucket is a check to fix: fix the cause and push again. A coverage-ratchet failure is fixed by a test
     where one is possible, otherwise by a new signed commit (never an amend) that raises the baseline and carries the
     `Coverage-Drop:` trailer exactly as README "Recording a rise" says, other trailers directly under it.
   - A `cancel` bucket, or a failure that is plainly infrastructure (such as a Nix setup step failing before any
     check runs, or a job that hangs until its timeout), gets one `gh run rerun <run-id> --failed`, with the id from
     `gh run list --branch <branch> --workflow ci.yml --limit 1 --json databaseId --jq '.[0].databaseId'`, then this
     step repeats once from the watch. If CI is still red after the rerun, blocked banner.
   - A non-zero exit with no JSON is a failed `gh` call (blocked banner).
2. **Merge** with `gh pr merge <number> --merge`: the repo allows merge commits only and deletes the remote branch
   itself. Do not pass `--delete-branch`; the local branch is checked out in the worktree and the delete would fail.
   Confirm with `gh pr view <number> --json state --jq .state` printing `MERGED`. A refused merge is read from
   `gh pr view <number> --json mergeStateStatus --jq .mergeStateStatus`:
   - `BEHIND`: the ruleset requires the branch to be up to date with `main`, and Renovate merges its own PRs
     unattended, so a branch that was current when CI started can fall behind. Rebase as the global GitHub rule
     says, inside the worktree: `git fetch origin`, note `git rev-parse origin/<branch>` as the old remote SHA,
     `git rebase origin/main`, confirm `git log --format='%G?' origin/main..HEAD` prints only `G` (any other letter:
     do not push, blocked banner), push with `git push --force-with-lease=<branch>:<old-remote-sha>` (the pre-push
     hook runs the full gate again), and go back to step 1 for the new CI run. A rebase that conflicts is a change
     the user must see: `git rebase --abort`, blocked banner. Never `gh pr update-branch`; it rewrites the commits
     unsigned.
   - `BLOCKED` with green CI: a rule other than up-to-date refused it, such as an unresolved review thread (the
     ruleset requires every thread resolved) or an unsigned commit. Blocked banner, saying which. Never pass
     `--admin`, which `gh` itself suggests, and never resolve a thread; both are the user's.
3. **Pull `main`** with `git -C "<main checkout>" pull --ff-only`, so the merge is on local `main`. If the pull is
   refused (the user has committed on or dirtied local `main`), run `git -C "<main checkout>" fetch --prune origin`
   instead, leave `main` as the user left it, and say in the summary that the next session's preconditions will fail
   until the user reconciles it; the next worktree starts from `origin/main` either way.
4. **Clean up.** `ExitWorktree` with `action: keep` (it cannot remove a worktree entered by `path`), then from the
   main checkout `git worktree remove .claude/worktrees/<N-description>` and `git branch --delete
   <type/N-description>`. Either may refuse: the delete does when local `main` does not yet hold the merge (step 3's
   refused pull), the remove when the worktree has a stray file. Leave a refused one in place and name it in the
   summary; never pass `--force` or `-D`. The next session's step 3 classifies it once `main` is current. Record the
   merge in `current-session.md`.
