---
name: work-backlog
description: >-
  Use when starting a session to work through this repo's GitHub issue backlog, or when asked to pick up the next
  backlog issue, bug or feature.
disable-model-invocation: true
---

# Working the backlog

One run of this skill is one backlog session. The aim is a backlog that shrinks: the session closes issues and never
refills the queue with work nobody needs. Only the user's decision opens an issue.

The user's global instructions and this repo's `.claude/CLAUDE.md` and `.claude/rules/` still apply in full. This
skill adds only what is below.

## State

Two local files under `.claude/backlog/`, which git ignores. At the start of each session, create the directory and
both files if any is missing.

- `session-log.md`: one line per session, `YYYY-MM-DD open=N`, where N is the open-issue count at the start.
- `pending-findings.md`: out-of-scope findings waiting for the user's decision, one entry each:

  ```markdown
  ## <one-line symptom>

  - File: <repo-relative path>
  - Trigger: <the realistic input that triggers it>
  - Found while: <issue or PR being worked>
  ```

## The session

1. **Preconditions.** The working tree is clean, on `main`, and up to date with `origin/main`, and `gh auth status`
   succeeds. If any of these fails, stop and tell the user (blocked banner) before doing anything else.
2. **Log.** Append today's line to `session-log.md`, with the count from
   `gh issue list --state open --limit 1000 --json number --jq length`.
3. **Leftover findings.** If `pending-findings.md` has entries, walk through them with `findings-walkthrough.md`.
4. **Unfinished work.** If an open PR or a pushed branch exists for an open issue, finish it (review, CI, merge) before
   choosing anything new. This comes before the bugs-first rule.
5. **Stall check.** Compare today's line with the third line before it. If that line exists and today's count is not
   lower than its count, run `triage.md` before choosing work.
6. **Choose the chunk** (below), propose it, and wait for the user's OK. The user may swap or trim it.
7. **Do the work** (below), one PR per issue, merging each on green CI.
8. **New findings.** Walk through every entry in `pending-findings.md` with `findings-walkthrough.md`.
9. **Summary.** Say what merged and which issues closed. Name any merged change that alters an existing lint's
   behavior, so the user knows to check consumers before the next release.

## Choosing the chunk

Read the open issues with `gh issue list`. If any open issue has the `bug` label, the chunk is bugs; otherwise it is
features (`enhancement`).

- **Bugs:** related bugs together (same code, same root cause). With none related, a batch of several small bugs.
- **Features:** one feature, or several that are closely related: they share code, one is a direct prerequisite of
  another, or they form a family that shares a runner, helper or template (such as the lints that wrap an external
  tool, or the shared workflows). One brainstorm covers a family.
- **Order:** by implementation sequence as the global "Prioritizing Work Items" rule defines it: what unblocks other
  items, then what speeds up the feedback loop, then what retires risk. Not by product value, and not by size.

The proposal names the issues, why they belong together, and why they come first. When a chunk is too big for one
session, propose the part that fits; the rest stays open for the next session.

## Doing the work

- **Features:** follow `feature-flow.md`.
- **Bugs:** **REQUIRED SUB-SKILLS:** superpowers:systematic-debugging, then superpowers:test-driven-development.
  Confirm the bug empirically, write the failing test, fix it.
- **PRs:** one per issue, so each merges and reverts on its own. Closely related issues that touch the same code may
  share a PR with at least one commit per issue. The PR body closes its issues with a closing keyword.
- **Review:** every PR gets a review with superpowers:requesting-code-review before merge. A round is one reviewer
  pass plus the fixes it prompts; at most two rounds per PR. Fix what the second round finds without a third review.
  A finding from reviewing the session's own change is fixed in the branch, given a known-gap comment, or dropped,
  under the filing bar's rules. It never becomes an issue or a pending finding.
- **Subagents:** every subagent prompt that implements or reviews carries the filing bar below and tells the subagent
  to return findings to the controller. The controller applies the rules below to them.

## The filing bar

A finding counts only if both hold:

1. **It is real.** It actually happened, or a realistic input triggers it: input the user, a consuming repo or a bot
   would actually produce. An input built to break the tool does not count, even with a working reproduction.
2. **It is wrong now.** A hole that only a hypothetical future edit could open does not count.

A finding that fails the bar is fixed in the branch when it is in scope and small, or gets a one-line known-gap
comment beside the code when that code is in a file the PR already changes, or is dropped. It is never reported,
recorded or filed. A known-gap comment states the gap in the present tense and cites no issue or PR number.

## Out-of-scope findings

A finding that passes the bar but lies outside the chunk:

- **Same PR** when it is a bug, the fix is under about 20 changed lines plus its test, and the fix lands in files or a
  shared helper the PR already changes: its own `test:` commit, then its own `fix:` commit.
- **Otherwise** append it to `pending-findings.md` the moment it is found, and carry on.

The session never opens an issue, PR comment or `TODO` for an out-of-scope finding. In this skill, "report them" in
the global "Fixing Findings" rule means `pending-findings.md` and nothing else. The known-gap comment, and the global
rule's `# TODO:` for a low-stakes ambiguity found mid-run, are separate and still allowed.

## Merging

When a PR's CI is green, merge it with no wait for the user. When CI fails, fix the cause and push again. A failure
that is plainly infrastructure (such as a Nix setup step failing before any check runs) gets one re-run. If CI is still
red after that, leave that PR open, carry on with the chunk's other issues, and tell the user in the
summary.

## Ending early

If the session ends before the chunk is done, the unfinished issues stay open and step 4 of the next session picks up
their branches or PRs. Every finding is already in `pending-findings.md`.

## Never

- Cut a release, tag, or bump a version. Releases are the user's own step.
- Start a whole-repo audit or sweep.
- Open an issue, or close one, without the user's decision in the walkthrough.

## Red flags

| Thought | Reality |
| --- | --- |
| "I'll file this so it isn't lost" | It goes in `pending-findings.md`; the walkthrough is what keeps it. |
| "It reproduces, so it's real" | A reproduction you built for an input nobody writes fails the bar. |
| "The reviewer found another hole in my fix; that's a new issue" | Fix it in the branch or drop it. |
| "Three findings is enough for one session" | There is no cap. Every passing finding is recorded. |
| "I'll sweep the repo for this pattern" | Fix the matches the change owns. The rest are findings, judged by the bar. |
