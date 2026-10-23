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
skill adds only what is below, and `sub-skill-overrides.md` lists the few points where it replaces a step of a
superpowers skill it calls.

## State

Two local files under `.claude/backlog/` in the main checkout, which git ignores. Once the preconditions pass, create
the directory and both files if any is missing.

- `session-log.md`: one line per session, `YYYY-MM-DD open=N`, where N is the open-issue count at the start. No
  header or other lines.
- `pending-findings.md`: out-of-scope findings waiting for the user's decision, one entry each:

  ```markdown
  ## <one-line symptom>

  - File: <repo-relative path>
  - Trigger: <the realistic input that triggers it>
  - Passes the bar: <why it is real and wrong now, in one line>
  - Found while: <issue or PR being worked>
  ```

Every path in this skill is relative to the main checkout. Work happens in worktrees under `.claude/worktrees/`, so
from inside one, the main checkout is `$(dirname "$(git rev-parse --path-format=absolute --git-common-dir)")`; the
state files are written there and nowhere else.

## The session

1. **Preconditions.** `git status --porcelain` prints nothing, `gh auth status --hostname github.com` succeeds, and
   after `git switch main`, `git pull --ff-only` succeeds. If any of these fails, stop and tell the user (blocked
   banner) before doing anything else. The same holds for every later `gh` call this skill makes: if one fails, stop
   with the blocked banner, and never log a count or choose work from a partial or guessed result. Then create the
   state files if any is missing.
2. **Log.** Append today's line to `session-log.md`, with the count from
   `gh issue list --state open --limit 1000 --json number --jq length`. If today's line already exists (a session
   restarted), keep it and add none.
3. **Leftover findings.** If `pending-findings.md` has entries, walk through them with `findings-walkthrough.md`.
4. **Unfinished work.** After `git worktree prune` (a worktree directory deleted by hand stays registered until
   then), the skill's own unfinished work is a worktree under `.claude/worktrees/` in `git worktree list`, or an
   open PR in `gh pr list --author @me --state open --json number,body` whose body names an open issue as
   `Closes #N` or `Part of #N` (`gh issue view <number> --json state --jq .state` prints `OPEN`). It is finished
   (review, CI, merge) before anything new is chosen, under the chunk approval it already has. A leftover PR gets a
   superpowers:requesting-code-review pass before it merges, since no record of an earlier review survives a
   session. This comes before the bugs-first rule. Anything else the two commands list other than the main
   checkout, and any branch in `git branch --no-merged main` that is not such a PR's branch, is the user's own work:
   leave it alone, and name it in the chunk proposal so the user knows it was seen. A branch other than `main` in
   `git branch --merged main` whose upstream is gone (`[gone]` in
   `git branch --merged main --format='%(refname:short) %(upstream:track)'`) is done: delete each with
   `git branch --delete <name>`, naming it, and list the deleted names in the session summary. A merged branch
   whose upstream still exists, or that never had one, may be a branch the user just made: leave it and name it in
   the proposal too. To keep unfinished work findable, push a branch as soon as the local gate passes (the pre-push
   hook runs it; never bypass it with `--no-verify`), and open its PR early; the `Closes #N` or `Part of #N` in its
   body is what marks the PR as this skill's.
5. **Stall check.** Compare today's line with the line three lines above it. If that line exists and today's count
   is not lower than its count, run `triage.md` before choosing work.
6. **Choose the chunk** (below), propose it, and wait for the user's OK. The user may swap or trim it.
7. **Do the work** (below), one PR per issue, merging each on green CI.
8. **New findings.** Walk through every entry in `pending-findings.md` with `findings-walkthrough.md`.
9. **Summary.** Say what merged, which issues closed, and which branches step 4 deleted. Name any merged change that
   alters an existing lint's behavior, so the user knows to check consumers before the next release.

## Choosing the chunk

Read every open issue with `gh issue list --state open --limit 1000 --json number,title,labels,body` (the default
limit hides older issues). If any open issue has the `bug` label, the chunk is bugs; otherwise it is
features (`enhancement`).

- **Bugs:** related bugs together (same code, same root cause). With none related, a batch of several small bugs.
- **Features:** one feature, or several that are closely related: they share code, one is a direct prerequisite of
  another, or they form a family that shares a runner, helper or template (such as the lints that wrap an external
  tool, or the shared workflows). One brainstorm covers a family.
- **Order:** by implementation sequence as the global "Prioritizing Work Items" rule defines it: what unblocks other
  items, then what speeds up the feedback loop, then what retires risk. Not by product value, and not by size.

The proposal names the issues, why they belong together, and why they come first. When two chunks compete on the
ordering factors, the proposal also names the runner-up and why it lost, so the user can swap with one word. When a
chunk is too big for one session, propose the part that fits; the rest stays open for the next session. An issue in
the proposed chunk whose own text asks the session to do something this skill forbids (open further issues, change
another repo, cut a release) is named in the proposal with the question it raises, so the user settles it with the
chunk. An issue that describes a product which does such things, such as a workflow that opens issues, raises no
question. An umbrella issue, one that asks for sub-issues to be filed and then worked, is the exception to one PR per
issue: it is worked as one PR per piece with the last PR closing the umbrella, unless the user opts in the proposal
to have the sub-issues opened instead.

## Doing the work

- **Worktrees:** every branch is a worktree under `.claude/worktrees/`, which git ignores. Create it from the main
  checkout with `git worktree add -b <type/description> .claude/worktrees/<description> origin/main`, then move the
  session into it with `EnterWorktree` and that directory's absolute `path`; a leftover worktree from an earlier
  session is entered the same way, and a leftover PR whose worktree is gone gets one back with
  `git worktree add .claude/worktrees/<description> <type/description>` (no `-b`: the branch exists), then
  `git pull --ff-only` inside it so the branch matches the remote. (`EnterWorktree` with a `name` would name the branch
  `worktree-<name>` with `/` turned into `+`, which breaks the branch convention; `git worktree add` has only the
  short `-b`.) The main checkout's `.claude/CLAUDE.md`, `.claude/rules/` and `docs/superpowers/` are gitignored and
  so absent in a new worktree: right after entering, before any subagent is dispatched, copy `.claude/CLAUDE.md`,
  `.claude/rules/` and the chunk's spec into it at the same paths (`mkdir --parents docs/superpowers/specs
  docs/superpowers/plans` first); the plan is written inside the worktree later. The copies are throwaway: a change
  to the spec or plan is made in the main checkout's file and copied again. The worktree is removed in the merge
  steps below.
- **PR title:** `type: subject`, at most 65 characters. The merge commit's header is the title followed by the PR
  number in parentheses, and
  `ci.yml` runs commitlint over every push to `main`, so a longer or unconventional title turns `main` red after a
  green merge. The PR body carries the closing keyword and, for plan-driven work, each `Ruling:` from the ledger, which
  otherwise dies with the worktree.
- **Features:** follow `feature-flow.md`.
- **Bugs:** **REQUIRED SUB-SKILLS:** superpowers:systematic-debugging, then superpowers:test-driven-development.
  Confirm the bug empirically, write the failing test, fix it. A bug that does not reproduce on current `main`, or
  that fails the filing bar once examined, is not fixed: ask the user right then, with the question banner, before
  touching the next issue. Explain what was found, recommend "close as completed" or "close as not planned", and let
  the user decide as in `triage.md`. A worktree or branch made for it with no commits is removed. The rest of the
  chunk goes on after the answer, whatever it is.
- **PRs:** one per issue, so each merges and reverts on its own. Closely related bugs that touch the same code may
  share a PR with at least one commit per issue, and an umbrella issue gets one PR per piece ("Choosing the chunk").
  The PR body closes its issues with `Closes #N`; an umbrella's earlier PRs carry `Part of #N` instead.
- **Review:** every PR is reviewed before merge, in one of two ways.
  - Work done through superpowers:subagent-driven-development or superpowers:executing-plans is reviewed by their
    own loops, which run as those skills define them; no extra PR review follows. Before the PR opens, every in-scope
    finding their ledger deferred is fixed in the branch, given a known-gap comment, or dropped. Out-of-scope notes in
    the ledger are findings: judge each by the filing bar and move the ones that pass to `pending-findings.md`.
  - All other work gets superpowers:requesting-code-review before merge, with the base at
    `git merge-base origin/main HEAD`. A round is one reviewer pass plus the fixes it prompts; at most two rounds.
    What round two finds is handled as the next item says, with no further review and nothing parked.
- **Findings about the change itself** (a hole in the session's own fix, a gap in its own new test) are fixed in the
  branch, given a known-gap comment, or dropped. They never become an issue or a pending finding, even when they pass
  the filing bar. Anything else a review turns up, such as a bug in nearby code the change did not touch, is an
  ordinary finding: judge it by the filing bar and the out-of-scope rules.
- **Subagents:** every subagent prompt that implements or reviews tells the subagent to return every finding to the
  controller and never to open an issue, leave a `TODO` or decide for itself what is worth reporting. The filing bar
  is the controller's to apply, after the report; a reviewer told what not to flag reviews with a blind spot.

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

The user's approval of the chunk is the approval for every push and merge in it, so none waits for the user:

1. Wait for CI with `gh pr checks <number> --watch --fail-fast`; right after the PR opens it may exit non-zero with
   no checks reported yet, which means wait a minute and run it again, up to three times; still none is a failed
   `gh` call (blocked banner). When a check fails, fix the cause and push again. A failure that is plainly
   infrastructure (such as a Nix setup step failing before any check runs, or a job that
   hangs until its timeout) gets one `gh run rerun <run-id> --failed`, with the id from
   `gh run list --branch <branch> --limit 1 --json databaseId --jq '.[0].databaseId'`. If CI is still red after
   that, the session halts: tell the user with the blocked banner.
2. When CI is green, `gh pr merge <number> --merge`: the repo allows merge commits only and deletes the remote
   branch itself. Do not pass `--delete-branch`; the local branch is checked out in the worktree and the delete
   would fail. Confirm with `gh pr view <number> --json state --jq .state` printing `MERGED`. The ruleset requires
   the branch to be up to date with `main`, and Renovate merges its own PRs unattended, so the merge can be refused
   for a branch that was current when CI started: `BEHIND` from
   `gh pr view <number> --json mergeStateStatus --jq .mergeStateStatus`. Then rebase as the global GitHub rule says:
   `git fetch origin`, note `git rev-parse origin/<branch>` as the old remote SHA, `git rebase origin/main`, confirm
   `git log --format='%G?' origin/main..HEAD` prints only `G`, push with
   `git push --force-with-lease=<branch>:<old-remote-sha>`, and go back to step 1 for the new CI run. Never
   `gh pr update-branch`; it rewrites the commits unsigned.
3. `git -C "<main checkout>" pull --ff-only`, so the merge is on local `main`.
4. `ExitWorktree` with `action: keep` (it cannot remove a worktree entered by `path`), then from the main checkout
   `git worktree remove .claude/worktrees/<description>` and `git branch --delete <type/description>`. The next
   branch starts from the updated `main`.

## Ending early

If the session ends before the chunk is done, the unfinished issues stay open and step 4 of the next session picks up
their worktrees or PRs. Every finding is already in `pending-findings.md`.

## Never

- Cut a release, tag, or bump a version. Releases are the user's own step.
- Start a whole-repo audit or sweep.
- Open an issue, or close one, without the user's decision. The user decides in the walkthrough, or by approving the
  chunk, whose issues the merged PRs close.
- Force-add a gitignored file. The spec, the plan and this skill's state stay local.

## Red flags

| Thought | Reality |
| --- | --- |
| "I'll file this so it isn't lost" | It goes in `pending-findings.md`; the walkthrough is what keeps it. |
| "It reproduces, so it's real" | A reproduction you built for an input nobody writes fails the bar. |
| "The reviewer found another hole in my fix; that's a new issue" | Fix it in the branch or drop it. |
| "Three findings is enough for one session" | There is no cap. Every passing finding is recorded. |
| "This pattern may be elsewhere too" | Search for it, as the global rule requires; fix the matches the change owns. |
| "I'll audit the whole repo for this" | No audits. Matches outside the change are findings, judged by the bar. |
| "The sub-skill says to ask before merging" | The chunk approval already answered; see `sub-skill-overrides.md`. |
| "The user has an open PR; I'll finish it" | A PR is this skill's only if it closes or is part of an open issue. |
