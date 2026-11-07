# End-of-run report

Step 9 of "The session" in `SKILL.md` writes its summary to `.claude/backlog/reports/<session-date>.md`, named by the
`session-date:` line's date (`-2`, `-3` and so on when that date already has one), before sending it, so it survives a
closed terminal and two runs can be compared. The message sent is the same text. `plan` writes no report. Section and
bullet names in quotes without a file are `SKILL.md`'s. Sections, each `none` when empty:

- `Merged`: what merged and which issues it closed.
- `Not merged`: every chunk issue that did not merge and why (a halt, a skipped piece, an issue the user closed); for a
  parked issue its question and recommended answer; a PR left open after a shakedown "no".
- `Rulings`: every `Ruling:` the session made.
- `Leftovers`: a refused `git branch --delete` or `git worktree remove` left in place, and a local `main` left behind
  because its pull was refused, with the note that the next run is blocked until the user reconciles it.
- `Findings`: every finding filed, dropped or still pending, and every issue the user's decision opened or closed this
  session (the walkthrough, `bug-flow.md` step 3, `triage.md`).
- `Consumer-impacting changes`: any merged change that alters an existing lint's or formatter's behavior, naming the
  consumers it would fail, so the user knows to fix them before the next release reaches them.
- `Budgets and shakedown`: a budget that stopped new issues from starting and the issues it left ("Budgets"); while
  shakedown is on, that it is on and that a follow-up question offers to lift it ("Shakedown").
- `Session notes`:
  - the branches and worktrees step 3 deleted;
  - a CI rerun and why;
  - a copied-rule conflict settled by satisfying the rule ("Copied rules");
  - a sub-skill pause whose quoted phrase the installed superpowers version no longer has (`sub-skill-overrides.md`);
  - a piece skipped because the user closed its issue;
  - an issue the brainstorm added to or dropped from the chunk;
  - every item in `.claude/backlog/parallel/summary.md`, which `leftovers.md` writes when step 3 finishes a parallel
    run's "Ending" (delete that file once the summary is sent).
