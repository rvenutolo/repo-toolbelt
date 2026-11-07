# The plan argument

`plan` rehearses a run without changing anything, so the proposal can be read before a real session is started.
Step numbers are "The session" in `SKILL.md`; section names in quotes without a file are `SKILL.md`'s.

- Step 1's `permission-diff` result is printed, not asked, and no rule is written. Step 1 runs `backlog-preflight`
  and prints its output in full, `FAIL` lines included (they are reported, not halted
  on, since nothing is about to run), then `git fetch --prune origin` only: no switch, no pull, and a dirty tree is
  fine.
- Step 2 takes the open-issue count for the proposal's trend line but appends no log line and creates no state file.
- Step 3 runs `git worktree prune --dry-run` in place of the prune and lists what it finds without acting on it.
- Step 4 names the pending findings without walking them.
- Step 5 compares the count step 2 took, standing in for today's line, and says whether the trend line would be
  marked stalled and `triage` offered.
- Step 6 prints the proposal in its layout ("The proposal" in `choosing.md`), halt questions and permission request
  included; for a feature chunk, whose questions and request belong to `feature-flow.md` step 5, it prints the ones it
  can already see and names the spec it would write. In place of the `Reply` section it prints a `Run` section: the
  exact invocation that runs this chunk, the same skill with the chunk's issue numbers (`/work-backlog 160`), and a
  note that a real run proposes again from fresh state and accepts the reply forms "The proposal" in `choosing.md`
  lists. The run ends there.

`plan`'s one message begins with the done banner, and the proposal is printed without its question banner, since
nothing is awaited. The printed proposal is not approvable, because the skill cannot re-invoke itself: to run it,
invoke the same skill again (`/work-backlog` or `/work-backlog-parallel`, whichever printed it) with the command
the `Run` section gives; a reply of `OK` to the rehearsal gets that command back, not a run. Scratch files go to
`.claude/tmp/` only; nothing is written under `.claude/backlog/`, so a rehearsal leaves nothing the next real run's step
3 would sort as a leftover.
