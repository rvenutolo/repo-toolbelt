# State files

The formats of the files under `.claude/backlog/` that "State" in `SKILL.md` names. Step numbers are `SKILL.md`'s
"The session".

## `session-log.md`

One line per session, `YYYY-MM-DD open=N`, where N is the open-issue count at the start; step 9 appends the word
`done` when the session reaches its summary, so a line without it is a session that died before it chose work and
says nothing about the backlog. No header or other lines, because the stall check (step 5) counts lines.

## `pending-findings.md`

Out-of-scope findings waiting for the user's decision, one entry each:

```markdown
## <one-line symptom>

- File: <repo-relative path>
- Trigger: <the realistic input that triggers it>
- Passes the bar: <why it is real and wrong now, in one line>
- Found while: <issue or PR being worked>
```

## `current-session.md`

The resume record, written as soon as the session has anything to resume: at step 6 once the chunk is approved, or
earlier when a "fix it now" in step 4 makes a worktree. Step 2 reads a copy of any date as the point to resume from,
step 3 reconciles it against git and GitHub (`leftovers.md`), and step 9 deletes it. Code is recovered from git and
GitHub; approvals, halt answers and spec paths survive only in this file and under `docs/superpowers/`. Its lines:

- `session-date: YYYY-MM-DD`: the date of the `session-log.md` line this session's step 2 appended; a file carried
  over from an earlier session gets it rewritten (step 6).
- The flow, bug or feature.
- The issues in order.
- The halt answers and the commands the user allowed, from the chunk OK (bugs) or `feature-flow.md` step 5
  (features).
- The spec path once `feature-flow.md` step 2 writes it, and `spec approved: <path>` after step 5.
- Each issue's plan path.
- Every banner halt with its answer ("Halting" in `resume.md`).
- `started: <UTC time>`, when the unattended part began ("Budgets" in `SKILL.md` says which step writes it).
- One line per issue as it reaches a PR, as its PR merges (or is left `pr open, not merged` by a shakedown "no",
  `merging.md` step 2), and as its worktree is removed.

`/work-backlog-parallel` adds its own lines (`cap:`, `issue <N> after <M>`, `piece`, the `workflow-*` lines; its
`SKILL.md` "State"); a file holding them is sorted by `leftovers.md` "A parallel run's files".

## `config`

The user's settings for both backlog skills, `key: value` lines: `shakedown:` ("Shakedown" in `SKILL.md`),
`max-hours:` and `max-parked:` ("Budgets"). An absent file or key means the default; an unreadable value is the
default too (step 6 says so in the proposal).
