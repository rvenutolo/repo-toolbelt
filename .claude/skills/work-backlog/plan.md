# The plan argument

`plan` rehearses a run so the proposal can be read before a real session is started. It changes nothing but
remote-tracking refs (the fetches) and scratch files in a temporary directory outside the repo. Step numbers are "The
session" in `SKILL.md`; section names in quotes without a file are `SKILL.md`'s.

- Step 0 is reported, not asked: the permission mode goes in the `Rehearsal notes` section below.
- Step 1's `permission-diff` result is printed in `Rehearsal notes`, not asked, and no rule is written; a bypassed
  rehearsal prints it too, since the real run may not be bypassed. Step 1 runs `backlog-preflight` and prints its
  output in full in `Rehearsal notes`, `FAIL` lines included (they are reported, not halted on, since nothing is
  about to run), then `git fetch --prune origin` only: no switch, no pull, and a dirty tree is fine.
- Step 2 takes the open-issue count for the proposal's trend line but appends no log line and creates no state file.
- Step 3 runs `git worktree prune --dry-run` in place of the prune, prints `backlog-status`'s output in `Rehearsal
  notes`, and acts on nothing it finds: every delete, removal or unlock `leftovers.md` would make is named on a
  `Would clean up:` line in place of the proposal's `Cleaned up:` line.
- Step 4 names the pending findings without walking them.
- Step 5 compares the count step 2 took, standing in for today's line, and says whether the trend line would be
  marked stalled and `triage` offered.
- Step 6 runs `backlog-index` with `--out "$(mktemp --directory)/issues.json"`, so the index and any long body it writes
  land outside the repo; every later read of `.claude/tmp/issues.json` or `.claude/tmp/issue-<N>.md` reads the temporary
  directory's copy instead. It prints the proposal in its layout ("The proposal" in `choosing.md`, or the printing
  skill's own), halt questions included; for a feature chunk, whose questions and request belong to `feature-flow.md`
  step 5, it prints the ones it can already see and names the spec it would write. For `/work-backlog`, the
  `Permissions` section prints the full request even in a bypassed rehearsal, marked "(this session is bypassed; a
  session started without bypass would ask for these)". In place of the `Reply` section it prints a `Run` section and
  then `Rehearsal notes`, and the run ends there.

The `Run` section gives two invocations: the same skill with the chunk's issue numbers (`/work-backlog 160`), which
runs this chunk, and the skill bare (`/work-backlog`), which chooses again from the backlog as it then stands. It
notes that a real run proposes again from fresh state and accepts the reply forms of the printing skill's own
proposal ("The proposal" in `choosing.md` for this skill; `/work-backlog-parallel`'s `planning.md` for that one).
`Rehearsal notes` holds the step 0 mode, step 1's `permission-diff` and preflight output, and step 3's
`backlog-status` output, in that order, after the proposal, so the proposal reads as the real one will.

`plan`'s one message begins with the done banner, and the proposal is printed without its question banner, since
nothing is awaited. The printed proposal is not approvable, because the skill cannot re-invoke itself: to run it,
invoke the same skill again (`/work-backlog` or `/work-backlog-parallel`, whichever printed it) with a command the
`Run` section gives; a reply of `OK` to the rehearsal gets those commands back, not a run. Nothing is written under
`.claude/`, so a rehearsal leaves nothing the next real run's step 3 would sort as a leftover.
