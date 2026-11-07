# Bug flow

**REQUIRED SUB-SKILLS:** superpowers:systematic-debugging, then superpowers:test-driven-development. Their own pauses
(three failed fixes, an exception to test-first, not knowing how to test, an external cause) are settled in
`sub-skill-overrides.md`. The halt questions and the permission request were answered with the chunk OK ("Choosing
the chunk" in `SKILL.md`). A "fix it now" from `findings-walkthrough.md` follows this flow under what its step 6
settles: the approval, the permission request and the branch name.

Per bug, in the order the chunk proposal gave:

1. Check the issue is still open, as the loop in "Doing the work" in `SKILL.md` says. Create and enter a worktree,
   copy the instruction files into it, and run the baseline tests ("Worktrees" in `SKILL.md`). Each bug has its own
   worktree and PR, related or not ("PRs" in `SKILL.md`).
2. **Confirm it empirically** on the branch, with the realistic input the issue describes. When the issue's
   reproduction is an external tool the devShell lacks (an audit that flags this repo), verifying the condition it
   reports, locally, is the confirmation.
3. **Not a bug.** A bug that does not reproduce on current `main`, or that fails the filing bar once examined, is not
   fixed: ask the user right then, with the question banner (the halt goes in `current-session.md` first, "Halting"
   in `resume.md`), before touching the next issue. Explain what was found, recommend "close as completed" or "close
   as not planned", and let the user decide as in `triage.md`. Then clean up: a worktree or branch made for it with
   no commits is removed from the main checkout after `ExitWorktree` with `action: keep` (`git worktree remove`,
   `git branch --delete`). The rest of the chunk goes on after the answer, whatever it is.
4. **Fix it:** write the failing test, then the fix, in separate `test:` and `fix:` commits, as the global testing
   rule asks.
5. **PR**, reviewed with superpowers:requesting-code-review and merged on green CI as `SKILL.md` describes.

Then return to `SKILL.md` step 8 for the findings walkthrough, which needs the user.
