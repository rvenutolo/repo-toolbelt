# State files

The formats of the files under `.claude/backlog/` that "State" in `SKILL.md` names. Step numbers are `SKILL.md`'s
"The session".

## `session-log.md`

One line per session, `YYYY-MM-DD open=N`, where N is the open-issue count at the start; step 9 appends the word
`done` when the session reaches its summary, so a line without it is a session that did not reach its summary and
says nothing about the backlog. No header or other lines, because the stall check (step 5) counts lines.

Which line is this session's, for step 2: a line dated today without `done` is this session restarted, so step 2
keeps it and appends none. A line whose date is the `session-date:` of a carried-over `current-session.md` holding
`workflow-*` lines belongs to that parallel run, even when dated today, so step 2 appends this session's own line. Any
other line, whatever its date, is another session's.

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
earlier when a "fix it now" in step 4 makes a worktree. Step 3 reads a copy of any date as the point to resume from
and reconciles it against git and GitHub (`leftovers.md`), and step 9 deletes it. Code is recovered from git and
GitHub; approvals, halt answers and spec paths survive only in this file and under `docs/superpowers/`. Its lines:

- `session-date: YYYY-MM-DD`: the date of the `session-log.md` line this session's step 2 appended; a file carried
  over from an earlier session gets it rewritten (step 6).
- `flow: bug`, `flow: feature`, or `flow: mixed` (a `/work-backlog-parallel` batch holding both kinds).
- `issue <N> [after <M>,<K>] [piece <name>] [kind bug|feature]`, one line per issue in chunk order: `after` (the
  parallel skill's) names the issues it depends on, `piece` an umbrella issue's piece, and `kind` is written for every
  issue under `flow: mixed` (optional otherwise), so a session that resumes one parked issue knows its flow. These
  forms are fixed because `backlog-status` reads them; `issue` lines are only the chunk list.
- The halt answers and the commands the user allowed, from the chunk OK (bugs) or `feature-flow.md` step 5
  (features).
- The spec path once `feature-flow.md` step 2 writes it, and `spec approved: <path>` after step 5.
- Each issue's plan path.
- Every banner halt with its answer ("Halting" in `resume.md`).
- `started: <UTC time>`, when the unattended part began ("Budgets" in `SKILL.md` says which step writes it).
- Progress lines, each `#<N> <event>`, so `backlog-status` can tie an event to its issue by the whole token `#<N>`:
  `#<N> pr <number>` as the issue reaches a PR, `#<N> merged` as it merges, `#<N> pr open, not merged` when a
  shakedown "no" leaves it (`merging.md` step 2), `#<N> left to user` after a second "no" (`leftovers.md` "To
  resume"; the line is all that stays of the issue, so its marker PR is no longer this skill's), `#<N> worktree
  removed` after cleanup, and the counts a compaction
  must not reset: `#<N> review round <k>` ("Review" in `SKILL.md`), `#<N> merge retry <k>` for each `BEHIND` or
  `DIRTY` rebase and `#<N> merge-time fix <k>` (`merging.md` step 2). A resumed session reads its counts from the
  highest `k` recorded.
- `at: <file> step <n>`: where the session is (`SKILL.md` "The session" step numbers, or a sub-file's:
  `at: merging.md step 1`), so a reader of a dead session's file sees where it stopped without inferring it from the
  other lines. It is rewritten in place whenever the session already writes `current-session.md`, and on entering or
  leaving a worktree; never with a round trip out of a worktree of its own ("Leaving to write" in `SKILL.md`), which
  would cost four tool calls a step. So a stale `at:` means "at or after" that step.

Every line after `session-date:` starts with the UTC time it was written, `date --utc +%FT%TZ`, then a space; the
`at:` line carries the time of its last rewrite. The times give `backlog-status` (`SKILL.md` "State") something to
show while a run is going and let a resumed session tell a fresh halt from a stale one. An example, mid-chunk:

```text
session-date: 2026-02-03
2026-02-03T14:02:11Z flow: bug
2026-02-03T14:02:11Z issue 42
2026-02-03T14:02:11Z started: 2026-02-03T14:02:11Z
2026-02-03T14:31:50Z #42 pr 57
2026-02-03T14:48:07Z at: merging.md step 1
```

`/work-backlog-parallel` adds its own lines (`cap:`, the `workflow-*` lines, its `started:` at launch and, after
its Ending, `park-answer <N>: <text>` for a parked issue the user already answered; its `SKILL.md` "State"); a file
holding them is sorted by `leftovers.md` "A parallel run's files".

## `config`

The user's settings for both backlog skills, `key: value` lines: `shakedown:` for `/work-backlog` and
`shakedown-parallel:` for `/work-backlog-parallel` ("Shakedown" in `SKILL.md`), `max-hours:` and `max-parked:`
("Budgets"). Blank lines and `#` comment lines are allowed. An absent file or key means the default; an unreadable
or unknown line is ignored and its default used, which `backlog-preflight` reports as a `WARN config` line that rides
in the proposal (step 6).
