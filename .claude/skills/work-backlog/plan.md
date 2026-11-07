# The plan argument

`plan` rehearses a run without changing anything, so the proposal can be read before a real session is started.
Step numbers are "The session" in `SKILL.md`; section names in quotes without a file are `SKILL.md`'s.

- Step 1 runs `backlog-preflight` and prints its output in full, `FAIL` lines included (they are reported, not halted
  on, since nothing is about to run), then `git fetch --prune origin` only: no switch, no pull, and a dirty tree is
  fine.
- Step 2 takes the open-issue count for the proposal's trend line but appends no log line and creates no state file.
- Step 3 runs `git worktree prune --dry-run` in place of the prune and lists what it finds without acting on it.
- Step 4 names the pending findings without walking them.
- Step 5 compares the count step 2 took, standing in for today's line, and says whether the trend line would be
  marked stalled and `triage` offered.
- Step 6 prints the proposal in its layout ("The proposal" in `choosing.md`), halt questions and permission request
  included; for a feature chunk, whose questions and request belong to `feature-flow.md` step 5, it prints the ones it
  can already see and names the spec it would write. The `Reply` section is printed too, so the user sees what a real
  run would accept, though nothing is awaited. The run ends there.

`plan`'s one message begins with the done banner, and the proposal is printed without its question banner, since
nothing is awaited. The printed proposal is not approvable, because the skill cannot re-invoke itself: to run it,
invoke `/work-backlog` again, with the issue numbers to keep this chunk. Scratch files go to `.claude/tmp/` only;
nothing is written under `.claude/backlog/`, so a rehearsal leaves nothing the next real run's step 3 would sort as
a leftover.
