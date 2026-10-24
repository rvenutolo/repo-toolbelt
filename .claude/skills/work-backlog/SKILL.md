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

The user's global instructions and this repo's `.claude/CLAUDE.md` and `.claude/rules/` still apply, with two
exceptions this skill makes on purpose: `sub-skill-overrides.md` lists the points where it replaces a pause of a
superpowers skill it calls, and `feature-flow.md` step 5 settles a copied-rule conflict found during unattended work
by satisfying the rule instead of asking, as `.claude/CLAUDE.md` otherwise requires. Read `sub-skill-overrides.md`
before the first sub-skill invocation of the session; its first two entries (the subagent clause and the skill that is
never invoked) apply to every session, plan-driven or not.

**After a context compaction**, re-read this file and `.claude/backlog/current-session.md` ("State"), which records
the approved chunk and where the session is in it. Nothing else about the session survives the compaction, and the
skill cannot be re-invoked by the model.

## How the session runs

The user takes part at fixed points and the rest runs unattended. A decision needs the user only when it changes
what gets built; everything else the session settles itself and records as a `Ruling:`. The user's points:

- **Bug chunk:** the chunk OK (which also answers the halt questions the proposal carries), then the findings
  walkthrough at the end.
- **Feature chunk:** the chunk OK, the brainstorm, one batched question round, then the findings walkthrough.
- **Either:** a question-banner halt when a decision changes what gets built, and a blocked-banner halt when
  something outside the session's control stops it (a failed `gh` call, red CI after a rerun, a refused merge).

## Terms

- **Chunk:** the set of issues the user approves for this session ("Choosing the chunk").
- **Approval:** what authorizes the unattended part, every push and merge included. For a bug chunk it is the chunk
  OK. For a feature chunk it is the chunk OK plus the question round of `feature-flow.md` step 5; nothing is pushed
  or merged before that round is answered.
- **Controller:** this session, as opposed to the subagents it dispatches.
- **Banners:** the `❓❓❓❓❓` (question) and `🚧🚧🚧🚧🚧` (blocked) lines the global "Attention Banners" rule
  requires on a message that waits for the user. A question-banner halt resumes after the answer; a blocked-banner
  halt ends the unattended part until the user has acted.
- **Filing bar:** the two-part test a finding must pass to be recorded ("The filing bar").
- **Known-gap comment:** a one-line comment beside code stating a gap in the present tense. It cites no issue or PR
  number because the repo's own issue-numbers lint fails CI on one, and because the docs rules bar history from
  comments.
- **Ledger:** the progress file a plan sub-skill keeps at `.superpowers/sdd/<plan-basename>/progress.md` in the
  worktree (both superpowers:subagent-driven-development and superpowers:executing-plans use it). It holds task
  completions, every `Ruling:`, and the findings those skills park or defer. The sub-skill deletes it at its own
  finish, so the ledger sweep ("Review") runs first.
- **Ledger sweep:** the controller's pass over the ledger before a plan-driven PR merges: in-scope deferred findings
  are fixed, known-gap-commented or dropped, and out-of-scope notes go through the filing bar.
- **`Ruling:`:** one line recording a decision the session made where a sub-skill would have asked the user. Every
  ruling goes in the PR body, since nothing else survives the worktree.
- **The marker:** the line `Backlog-Session: YYYY-MM-DD` that ends every PR body this skill writes. The user never
  writes it, so it is what tells this skill's PRs from the user's.

## State

Local files under `.claude/backlog/` in the main checkout. Everything this skill writes outside a PR is local and
untracked: `.claude/` apart from `.claude/skills/`, `docs/superpowers/` and `.superpowers/` are ignored by the user's
global git excludes, not this repo's `.gitignore`. So the state files, specs, plans, ledgers, worktrees and rule
copies never appear in `git status` and are never force-added. Step 2 creates the directory and the first two files
if any is missing.

- `session-log.md`: one line per session, `YYYY-MM-DD open=N`, where N is the open-issue count at the start. No
  header or other lines, because the stall check (step 5) counts lines.
- `pending-findings.md`: out-of-scope findings waiting for the user's decision, one entry each:

  ```markdown
  ## <one-line symptom>

  - File: <repo-relative path>
  - Trigger: <the realistic input that triggers it>
  - Passes the bar: <why it is real and wrong now, in one line>
  - Found while: <issue or PR being worked>
  ```

- `current-session.md`: written at step 6 once the chunk is approved and deleted at step 9. It holds the date, the
  flow (bug or feature), the issues in order, the halt answers from the chunk OK, and one line per issue as it
  reaches a PR and as it merges. A copy left by an earlier date is a dead session: step 2 deletes it, since step 3
  recovers that session's work from git and GitHub.

Every path in this skill is relative to the main checkout. Work happens in worktrees under `.claude/worktrees/`, so
from inside one, the main checkout is `$(dirname "$(git rev-parse --path-format=absolute --git-common-dir)")`; the
state files are written there and nowhere else.

## The session

1. **Preconditions.** `git status --porcelain` prints nothing, `gh auth status --hostname github.com` succeeds, and
   after `git switch main` (the user's branch stays as it was, checked out nowhere), `git fetch --prune origin` and
   `git pull --ff-only` succeed; the prune is what later marks a merged branch's upstream `[gone]`. If any of these
   fails, stop and tell the user (blocked banner) before doing anything else. The same holds for every later `gh`
   call this skill makes: if one fails, stop with the blocked banner, and never log a count or choose work from a
   partial or guessed result.
2. **Log.** Create the state files if any is missing, and delete a `current-session.md` from an earlier date. Append
   today's line to `session-log.md`, with the count from `gh issue list --state open --limit 1000 --json number --jq
   length`. If today's line already exists (a session restarted), keep it and add none.
3. **Unfinished work.** Normally there is none: `git worktree prune`, then `git worktree list` shows only the main
   checkout, `gh pr list --author @me --state open` is empty, and `git branch` shows `main` alone. Then go to step 4.
   Otherwise follow `leftovers.md`, which sorts what is there into this skill's unfinished work (finished before
   anything new is chosen), merged branches to delete, and the user's own work, which is left alone.
4. **Leftover findings.** If `pending-findings.md` has entries, walk through them with `findings-walkthrough.md`.
   This runs after step 3 so that a "fix it now" decision does not start new work while a leftover is unfinished.
5. **Stall check.** Compare today's line with the one three lines above it (today's is line N; the line at N-3 is
   three logged sessions earlier, however many calendar days apart). If that line exists and today's count is not
   lower than its count, the backlog has not shrunk over three sessions: run `triage.md` before choosing work.
6. **Choose the chunk** (below), propose it, and wait for the user's OK. The user may swap or trim it. Then write
   `current-session.md`.
7. **Do the work** (below), one PR per issue, merging each on green CI.
8. **New findings.** Walk through every entry in `pending-findings.md` with `findings-walkthrough.md`.
9. **Summary.** One message that names: what merged and which issues it closed; any merged change that alters an
   existing lint's behavior, so the user knows to check consumers before the next release; the branches and
   worktrees step 3 deleted; a refused `git branch --delete` or `git worktree remove` left in place; a local `main`
   left behind because its pull was refused, with the note that the next run is blocked until the user reconciles it;
   a CI rerun and why; a piece skipped because the user closed its issue; an issue the brainstorm added to or dropped
   from the chunk; a copied-rule conflict settled by satisfying the rule (`feature-flow.md` step 5); and a sub-skill
   pause whose quoted phrase the installed superpowers version no longer has (`sub-skill-overrides.md`). Then
   delete `current-session.md`.

## Choosing the chunk

Read every open issue with `gh issue list --state open --limit 1000 --json number,title,labels,body` (the default
limit hides older issues). If any open issue has the `bug` label, the chunk is bugs; otherwise it is features,
whatever other labels the issues carry.

- **Bugs:** related bugs together (same code, same root cause). With none related, a batch of several small bugs.
- **Features:** one feature, or several that are closely related: they share code, one is a direct prerequisite of
  another, or they form a family that shares a runner, helper or template (such as the lints that wrap an external
  tool, or the shared workflows). One brainstorm covers a family.
- **Order:** by implementation sequence as the global "Prioritizing Work Items" rule defines it: what unblocks other
  items, then what speeds up the feedback loop, then what retires risk. Not by product value, and not by size. A
  prerequisite internal to the chunk that no issue names (a shared helper the pieces need) counts under the first
  factor.
- **Size:** two to four PRs. Each PR waits on its own CI run and merges before the next starts, and the findings
  walkthrough still follows, so more than four rarely finishes in a session. When a family is bigger, propose the
  part that fits; the rest stays open for the next session, and the spec covers only the proposed part, with a
  shared helper designed so the rest can reuse it.
- **Comments:** for the issues in the proposed chunk, also read `gh issue view <number> --comments`. A decision the
  user left in a comment changes the proposal, and the list query returns bodies only.

The proposal is one message, ending with the question banner, that names:

- the issues, why they belong together, and why they come first by the three ordering factors;
- the runner-up chunk and why it lost, so the user can swap with one word;
- the user's own work step 3 saw and left alone, or "none";
- for a bug chunk, every likely halt question for the unattended part, each with a recommended answer and a one-line
  reason (the global "Unattended Execution" rule), and the commands the flow will run that may prompt for
  permission: `git worktree add`, `git worktree remove`, `git branch --delete`, `git pull --ff-only`, `git rebase`,
  `git push --force-with-lease`, `rm -rf` of a plan workspace. The OK on the chunk answers the questions and asks the
  user to confirm the commands are allowed, since a prompt mid-run stalls the unattended part (the global "Long
  Multi-Step Tasks" rule). A feature chunk asks both in `feature-flow.md` step 5 instead;
- an issue whose own text asks the session to do something this skill forbids (open further issues, change another
  repo, cut a release), with the question it raises, so the user settles it with the chunk. An issue that describes
  a product which does such things, such as a workflow that opens issues, raises no question;
- an umbrella issue in the chunk, one that asks for sub-issues to be filed and then worked: it is worked as one PR
  per piece ("PRs" below) unless the user opts in the proposal to have the sub-issues opened instead.

## Doing the work

The normal sequence per issue: create a worktree and enter it, copy the instruction files in, build the change
(`feature-flow.md` or `bug-flow.md`), push as soon as the local gate passes and open the PR, review, merge on green
CI, pull `main`, remove the worktree. Strictly one issue at a time: the next worktree is created only after the
previous PR has merged, because it starts from the merged `origin/main` and, in a family, builds on the earlier
piece's code.

- **Branches:** `<type>/<N>-<description>`, where N is the issue number, so a branch or worktree can be tied to its
  issue without a PR. Related bugs sharing one PR use the first issue in chunk order. An umbrella's pieces use the
  umbrella's number with their own descriptions. The worktree directory is `.claude/worktrees/<N-description>`.
- **Worktrees:** every branch is a worktree.
  - Create from the main checkout with
    `git worktree add --no-track -b <type/N-description> .claude/worktrees/<N-description> origin/main`. Without
    `--no-track` the branch's upstream would be `origin/main`, a plain `git push` would refuse, and a later
    `git pull --ff-only` would pull `main` into the branch. A failed `git worktree add` is a blocked-banner halt.
  - Enter with `EnterWorktree` and the directory's absolute `path`. Never `EnterWorktree` with a `name`: it would
    name the branch `worktree-<name>` with `/` turned into `+`, which breaks the branch convention.
  - Copy, right after entering and before any subagent is dispatched: `.claude/CLAUDE.md`, `.claude/rules/` and,
    for a feature chunk, the chunk's spec, at the same paths (`mkdir --parents docs/superpowers/specs
    docs/superpowers/plans` first). They are local files ("State") and so absent in a new worktree. The copies are
    throwaway: a change to the spec, the plan, or a rule or rule override is made in the main checkout's file and
    copied again; a change made only in the worktree dies with it.
  - Resume a leftover worktree or PR as `leftovers.md` says. Remove in `merging.md` step 4.
- **Long commands:** the pre-push gate, the baseline tests, and the CI watch each run for minutes, past the Bash
  tool's two-minute default and sometimes its ten-minute foreground maximum. Run them with `run_in_background` and
  read the output when the command exits; a foreground timeout is not a failure of the command.
- **Features:** follow `feature-flow.md`.
- **Bugs:** follow `bug-flow.md`.
- **PRs:** one per issue, so each merges and reverts on its own. Closely related bugs that touch the same code may
  share a PR with at least one commit per issue. An umbrella issue gets one PR per piece, each from its own worktree
  and plan (`feature-flow.md` step 6 runs once per piece); the earlier PRs carry `Part of #N` and the last one
  `Closes #N`. Push a branch with `git push --set-upstream origin <type/N-description>` as soon as the local gate
  passes (the pre-push hook runs it; never bypass it with `--no-verify`), and open its PR early, so unfinished work
  stays findable. The body uses only `Closes #N` or `Part of #N`, never another closing keyword, because
  `leftovers.md` matches exactly those two strings to tie a PR to its issue. It carries every `Ruling:` the work
  made (from the ledger for plan-driven work, from the session's own calls otherwise), and ends with the marker.
- **PR title:** `type: subject`, at most 65 characters. The merge commit's header is the title followed by the PR
  number in parentheses, and `ci.yml` runs commitlint over every push to `main`, so a longer or unconventional title
  turns `main` red after a green merge.
- **Review:** every PR is reviewed before merge, in one of two ways.
  - Work done through superpowers:subagent-driven-development or superpowers:executing-plans is reviewed by their
    own loops, which run as those skills define them; no extra PR review follows. Before the PR merges, and before
    the sub-skill deletes its workspace, the controller runs the ledger sweep: every in-scope finding the ledger
    deferred or parked is handled as "Findings about the change itself" says, and every out-of-scope note is judged
    by the filing bar, the ones that pass moving to `pending-findings.md`. For plan-driven work this sweep is the
    only time out-of-scope findings are written; a finding a subagent reports mid-plan goes in the ledger, not
    straight to the file, so nothing is recorded twice.
  - All other work gets superpowers:requesting-code-review before merge, with the base at
    `git merge-base origin/main HEAD`. A round is one reviewer pass plus the fixes it prompts; at most two rounds,
    because a small change that still draws findings after two passes is a wrong change, not an under-reviewed one,
    and the cap bounds what an unattended run spends. Round-two findings about the change itself are handled as the
    next item says, with no further review; anything else still goes through the filing bar and "Out-of-scope
    findings" (a same-PR fix made after round two also gets no review).
  - A leftover PR from an earlier session gets a superpowers:requesting-code-review pass before it merges whichever
    way it was built, since no record of an earlier review survives a session.
- **Findings about the change itself** (a hole in the session's own fix, a gap in its own new test, a finding a plan
  sub-skill parked or deferred) are fixed in the branch, given a known-gap comment, or dropped. They never become an
  issue or a pending finding, even when they pass the filing bar. Anything else a review turns up, such as a bug in
  nearby code the change did not touch, is an ordinary finding: judge it by the filing bar and the out-of-scope
  rules. Every other place in this skill that says "fixed, known-gap-commented or dropped" means this item.
- **Subagents:** every subagent prompt that implements or reviews tells the subagent to return every finding to the
  controller and never to open an issue, leave a `TODO` or decide for itself what is worth reporting. The filing bar
  is the controller's to apply, after the report; a reviewer told what not to flag reviews with a blind spot. The
  prompt templates the sub-skills ship carry no such clause, so it is appended to each (`sub-skill-overrides.md`).
  A reviewer inspects the branch with `git diff` and `git show`, never by adding a worktree of its own, which the
  next session's step 3 would report as the user's work.

## The filing bar

A finding counts only if both hold:

1. **It is real.** It actually happened, or a realistic input triggers it: input the user, a consuming repo or a bot
   would actually produce. An input built to break the tool does not count, even with a working reproduction.
2. **It is wrong now.** A hole that only a hypothetical future edit could open does not count.

A finding that fails the bar is never fixed: a guard or test for an input nobody writes is the speculative work the
spec review strikes. It gets a known-gap comment when the code is in a file the PR already changes, or is dropped. It
is never reported, recorded or filed. (A finding that passes the bar may also end as a known-gap comment, but only
by the user's choice in `findings-walkthrough.md`.)

## Out-of-scope findings

A finding that passes the bar but lies outside the chunk:

- **Same PR** when it is a bug, the fix is under about 20 changed lines plus its test, and the fix lands in files or a
  shared helper the PR already changes: its own `test:` commit, then its own `fix:` commit.
- **Otherwise** append it to `pending-findings.md` the moment it is found (for plan-driven work, at the ledger sweep
  in "Review"), and carry on.

The session never opens an issue, PR comment or `TODO` for an out-of-scope finding. In this skill, "report them" in
the global "Fixing Findings" rule means `pending-findings.md` and nothing else. The known-gap comment, and the global
rule's `# TODO:` for a low-stakes ambiguity found mid-run, are separate and still allowed.

## Merging

Approval ("Terms") covers every push and merge in the chunk, so none waits for the user. The normal case: watch CI,
confirm every check's bucket is `pass` or `skipping`, `gh pr merge <number> --merge`, pull `main` in the main
checkout, exit and remove the worktree, delete the branch. `merging.md` gives the commands and handles the
exceptions: a check that fails, a coverage-ratchet failure, an infrastructure failure worth one rerun, a branch
that fell behind `main`, a merge the ruleset refuses, and a pull or delete that is refused.

A blocked-banner halt in `merging.md` stops the chunk at that issue. Tell the user what stopped it and what the
session would do next; after the user's reply, go on with the issues that do not depend on the halted one, or stop,
as the user says. Record the halt in `current-session.md`.

## Ending early

If the session ends before the chunk is done, the unfinished issues stay open and step 3 of the next session picks up
their worktrees or PRs. Findings already judged are in `pending-findings.md`; findings a plan sub-skill parked may
still be in the leftover worktree's ledger, which `leftovers.md` has the next session sweep on resume.

## Never

- Cut a release, tag, or bump a version. Releases are the user's own step.
- Start a whole-repo audit or sweep.
- Open an issue, or close one, without the user's decision. The user decides in the walkthrough, or by approving the
  chunk, whose issues the merged PRs close.
- Force-add a local file ("State").
- Force a delete: no `git branch -D`, no `git worktree remove --force`, no `gh pr merge --admin`.

## Red flags

| Thought | Reality |
| --- | --- |
| "The sub-skill says to ask before merging" | Approval already answered; see `sub-skill-overrides.md`. |
| "The chunk is approved, so I can push this feature" | Not before the question round; see "Approval" in Terms. |
| "The user's open PR closes an issue; I'll finish it" | Without the marker it is the user's, whatever it closes. |
| "CI is watching; I'll start the next worktree meanwhile" | One issue at a time; the next starts from the merge. |
| "Checks exited 0, so CI is green" | A cancelled check exits 0 too; green is every bucket `pass` or `skipping`. |
| "`gh` says to merge with `--admin`" | Bypassing a rule is the user's call. Blocked banner. |
| "The reviewer found another hole in my fix; that's a new issue" | Fix it in the branch, known-gap it, or drop it. |
| "The push timed out, so it failed" | The pre-push gate takes minutes; run it in the background and read the result. |
