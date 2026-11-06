# Bug flow

**REQUIRED SUB-SKILLS:** superpowers:systematic-debugging, then superpowers:test-driven-development. Their own pauses
(three failed fixes, an exception to test-first, not knowing how to test, an external cause) are settled in
`sub-skill-overrides.md`. The halt questions and the permission request were answered with the chunk OK ("Choosing
the chunk" in `SKILL.md`). For a "fix it now" from `findings-walkthrough.md`, the user's answer stands in for the
chunk OK, the permission request is made first if this session has not made it yet, and the branch is named for
the finding ("Branches" in `SKILL.md`).

Per bug, in the order the chunk proposal gave:

1. Check the issue is still open, as the normal sequence in "Doing the work" in `SKILL.md` says. Create and enter a
   worktree, and copy the instruction files into it ("Worktrees" in `SKILL.md`). Each bug has its own worktree and
   PR, related or not ("PRs").
2. **Confirm it empirically** on the branch, with the realistic input the issue describes.
3. **Not a bug.** A bug that does not reproduce on current `main`, or that fails the filing bar once examined, is not
   fixed: ask the user right then, with the question banner (the halt goes in `current-session.md` first, "Banners"
   in `SKILL.md`), before touching the next issue. Explain what was found, recommend "close as completed" or "close
   as not planned", and let the user decide as in `triage.md`. Then clean up: a worktree or branch made for it with
   no commits is removed from the main checkout after `ExitWorktree` with `action: keep` (`git worktree remove`,
   `git branch --delete`). The rest of the chunk goes on after the answer, whatever it is.
4. **Fix it:** write the failing test, then the fix, in separate `test:` and `fix:` commits, as the global testing
   rule asks.
5. **PR**, reviewed with superpowers:requesting-code-review and merged on green CI as `SKILL.md` describes.

Then return to `SKILL.md` step 8 for the findings walkthrough, which needs the user.
