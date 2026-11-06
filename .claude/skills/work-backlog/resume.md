# Resuming

What a session does when its context is compacted, when it halts for the user, and when it ends before the chunk is
done. Every case reads `current-session.md` ("State" in `SKILL.md`; format in `state.md`), which is why a halt is
recorded there before the user sees it.

## After a compaction

Nothing about the session survives a context compaction except what is on disk, and the skill cannot be re-invoked by
the model. So, in order:

1. Re-read `SKILL.md` (its pointer brought you here), then come back to step 2 of this file.
2. Re-read `.claude/backlog/current-session.md`, which records the approved chunk and where the session is in it,
   then the spec, plan and ledger it names, if any. Check where the session is: `git rev-parse --show-toplevel`
   names the main checkout or a worktree.
3. If the record's last banner halt has no answer, stay in (or return to) the main checkout and ask it again: the
   user's reply may have been lost with the context. Go no further until it is answered.
4. Otherwise carry on from the record. A compaction can land while the session has left a worktree to write
   ("Leaving to write" in `SKILL.md`): if the record shows an issue whose worktree still exists and whose merge is
   not recorded, and the session is in the main checkout, re-enter that worktree. Re-read the reference file of the
   current step (`feature-flow.md`, `bug-flow.md`, `merging.md`, …); if a sub-skill was in progress, re-invoke it and
   re-read `sub-skill-overrides.md`, since its pauses would otherwise run as the sub-skill writes them.

## Halting

Before sending a banner, append the halt (the issue, the question, the options) to `current-session.md`, and the
answer once it arrives: a compaction while the message waits would otherwise hand the user's one-word reply to a
session that no longer knows the question ("After a compaction" step 3). This applies once `current-session.md`
exists. A banner sent before that (the chunk proposal, a step 3 to 5 question, the bad-argument question) is not
recorded: after a compaction with no file, run `SKILL.md` steps 1 to 6 again (step 2 keeps the log line) and the user
answers the re-sent message. From inside a worktree both writes leave it first ("Leaving to write" in `SKILL.md`).
Before a blocked banner the session stays out of the worktree, and re-enters only when the user's reply resumes that
issue.

## Ending early

If the session ends before the chunk is done, the unfinished issues stay open and step 3 of the next session picks up
their worktrees or PRs, with the approval, halt answers and spec path from `current-session.md`. Findings already
judged are in `pending-findings.md`; findings a plan sub-skill parked may still be in the leftover worktree's ledger,
which `leftovers.md` has the next session sweep before that PR merges.
