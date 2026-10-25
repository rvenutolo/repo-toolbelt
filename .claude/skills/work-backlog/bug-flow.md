# Bug flow

**REQUIRED SUB-SKILLS:** superpowers:systematic-debugging, then superpowers:test-driven-development. Their own pauses
(three failed fixes, an exception to test-first, not knowing how to test, an external cause) are settled in
`sub-skill-overrides.md`. The halt questions and the permission request were answered with the chunk OK ("Choosing
the chunk" in `SKILL.md`). For a "fix it now" from `findings-walkthrough.md`, the user's answer stands in for the
chunk OK, the permission request is made first if this session has not made it yet, and the branch is named for
the finding ("Branches" in `SKILL.md`).

Per bug, in the order the chunk proposal gave:

1. Create and enter a worktree, and copy the instruction files into it ("Worktrees" in `SKILL.md`). Closely related
   bugs that share a PR share one worktree, named for the first of them in chunk order ("Branches").
2. **Confirm it empirically** on the branch, with the realistic input the issue describes.
3. **Not a bug.** A bug that does not reproduce on current `main`, or that fails the filing bar once examined, is not
   fixed: ask the user right then, with the question banner (the halt goes in `current-session.md` first, "Banners"
   in `SKILL.md`), before touching the next issue. Explain what was found, recommend "close as completed" or "close
   as not planned", and let the user decide as in `triage.md`. Then clean up: a worktree or branch made for it with
   no commits is removed (`git worktree remove`, `git branch --delete`); when it shares a PR with another bug, drop
   its `Closes #N` line with `gh pr edit <number> --body`, and drop its commits (its test included) with a
   non-interactive rebase, since the Bash tool runs no interactive command: `git rebase --onto <parent-of-first>
   <last-dropped>` when they are contiguous, otherwise `GIT_SEQUENCE_EDITOR='sed --in-place "/<sha-prefix>/d"'
   git rebase --interactive <merge-base>` once per dropped commit. The branch is already pushed, so push the result
   with `git push --force-with-lease=<branch>:<old-remote-sha>` as `merging.md` step 2 does, while the branch stays
   for the other issue. The rest of the chunk goes on after the answer, whatever it is.
4. **Fix it:** write the failing test, then the fix, in separate `test:` and `fix:` commits, as the global testing
   rule asks.
5. **PR**, reviewed with superpowers:requesting-code-review and merged on green CI as `SKILL.md` describes.

Then return to `SKILL.md` step 8 for the findings walkthrough, which needs the user.
