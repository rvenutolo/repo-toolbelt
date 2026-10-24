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
before the first sub-skill invocation of the session; it also carries rules that apply to every session.

## Terms

- **Chunk:** the set of issues the user approves for this session ("Choosing the chunk").
- **Filing bar:** the two-part test a finding must pass to be recorded ("The filing bar").
- **Known-gap comment:** a one-line comment beside code stating a gap in the present tense, citing no issue or PR
  number.
- **Ledger:** the findings file superpowers:subagent-driven-development and superpowers:executing-plans keep while a
  plan runs. It lives in the worktree and dies with it.
- **`Ruling:`:** one line recording a decision the session made where a sub-skill would have asked the user. Every
  ruling goes in the PR body, since nothing else survives the worktree.
- **The marker:** the line `Backlog-Session: YYYY-MM-DD` that ends every PR body this skill writes. The user never
  writes it, so it is what tells this skill's PRs from the user's.

## State

Two local files under `.claude/backlog/` in the main checkout. The path is ignored by the user's global git excludes
(not this repo's `.gitignore`), so it is untracked on this machine and stays local. Step 2 creates the directory and
both files if any is missing.

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
   after `git switch main` (the user's branch stays as it was, checked out nowhere), `git fetch --prune origin` and
   `git pull --ff-only` succeed; the prune is what later marks a merged branch's upstream `[gone]`. If any of these
   fails, stop and tell the user (blocked banner) before doing anything else. The same holds for every later `gh`
   call this skill makes: if one fails, stop with the blocked banner, and never log a count or choose work from a
   partial or guessed result.
2. **Log.** Create the state files if any is missing. Append today's line to `session-log.md`, with the count from
   `gh issue list --state open --limit 1000 --json number --jq length`. If today's line already exists (a session
   restarted), keep it and add none.
3. **Unfinished work.** Run `git worktree prune` first (a worktree directory deleted by hand stays registered until
   then). Then sort what `git worktree list`, `gh pr list --author @me --state open --json number,body,headRefName`
   and `git branch --merged main --format='%(refname:short) %(upstream:track)'` show into three kinds:
   - **This skill's unfinished work:** an open PR whose body ends with the marker and names an open issue as
     `Closes #N` or `Part of #N` (`gh issue view <number> --json state --jq .state` prints `OPEN`), and a worktree
     under `.claude/worktrees/` whose branch has no PR, when the issue its branch name starts with ("Branches" below)
     is still open. A worktree under `.claude/worktrees/` with no commits beyond `origin/main` is this skill's too:
     remove it (`git worktree remove`, then `git branch --delete`) and name it in the summary. Unfinished work is
     finished (review, CI, merge) before anything new is chosen, under the chunk approval it already has; the spec
     and plan that record that approval are under `docs/superpowers/` in the main checkout. This comes before the
     bugs-first rule. Before resuming a leftover PR, read `gh pr view <number> --json isDraft,reviewDecision,comments,
     reviews`: a draft, a non-empty review decision, or any comment or review means the user has touched it since;
     ask with the question banner whether to resume it or leave it, and do not touch it before the answer.
   - **Done:** a branch other than `main` that the `--merged` listing shows with upstream `[gone]`. If a worktree
     under `.claude/worktrees/` still holds it (a session that died between the merge and the cleanup),
     `git worktree remove .claude/worktrees/<N-description>` first, since the delete fails while the branch is
     checked out; then `git branch --delete <name>`, naming each, and list the deleted names in the session summary.
   - **The user's own work**, left alone and named in the chunk proposal so the user knows it was seen: every other
     worktree, every other open PR (one without the marker is the user's even when it says `Closes #N`), every branch
     in `git branch --no-merged main` that is not a leftover PR's branch, a merged branch whose upstream still exists
     or that never had one (it may be a branch the user just made), and a worktree whose issue has since been closed.
4. **Leftover findings.** If `pending-findings.md` has entries, walk through them with `findings-walkthrough.md`.
   This runs after step 3 so that a "fix it now" decision does not start new work while a leftover is unfinished.
5. **Stall check.** Compare today's line with the one three lines above it (today's is line N; the line at N-3 is
   three logged days earlier). If that line exists and today's count is not lower than its count, run `triage.md`
   before choosing work.
6. **Choose the chunk** (below), propose it, and wait for the user's OK. The user may swap or trim it.
7. **Do the work** (below), one PR per issue, merging each on green CI.
8. **New findings.** Walk through every entry in `pending-findings.md` with `findings-walkthrough.md`.
9. **Summary.** One message that names: what merged and which issues it closed; any merged change that alters an
   existing lint's behavior, so the user knows to check consumers before the next release; the branches and
   worktrees session step 3 deleted; a refused `git branch --delete` or `git worktree remove` left in place (Merging
   step 4); a local `main` left behind because its pull was refused, with the note that the next run is blocked until
   the user reconciles it (Merging step 3); a CI rerun and why (Merging step 1); a piece skipped because the user
   closed its issue (`feature-flow.md` step 6); a copied-rule conflict settled by satisfying the rule
   (`feature-flow.md` step 5); and a sub-skill pause whose quoted phrase the installed superpowers version no longer
   has (`sub-skill-overrides.md`).

## Choosing the chunk

Read every open issue with `gh issue list --state open --limit 1000 --json number,title,labels,body` (the default
limit hides older issues). If any open issue has the `bug` label, the chunk is bugs; otherwise it is features,
whatever other labels the issues carry.

- **Bugs:** related bugs together (same code, same root cause). With none related, a batch of several small bugs.
- **Features:** one feature, or several that are closely related: they share code, one is a direct prerequisite of
  another, or they form a family that shares a runner, helper or template (such as the lints that wrap an external
  tool, or the shared workflows). One brainstorm covers a family.
- **Order:** by implementation sequence as the global "Prioritizing Work Items" rule defines it: what unblocks other
  items, then what speeds up the feedback loop, then what retires risk. Not by product value, and not by size.

The proposal names the issues, why they belong together, and why they come first. When two chunks compete on the
ordering factors, the proposal also names the runner-up and why it lost, so the user can swap with one word. When a
chunk is too big for one session, propose the part that fits; the rest stays open for the next session, and the
brainstorm and spec cover only the proposed part. A bug chunk's proposal also carries every likely halt question
for the unattended part, each with a recommended answer and a one-line reason, and the tools and permissions the
work will need (the global "Unattended Execution" and "Long Multi-Step Tasks" rules); the OK on the chunk answers
them. A feature chunk asks these in `feature-flow.md` step 5 instead. An issue in the proposed chunk whose own text
asks the session to do something this skill forbids (open further issues, change another repo, cut a release) is
named in the proposal with the question it raises, so the user settles it with the chunk. An issue that describes a
product which does such things, such as a workflow that opens issues, raises no question. An umbrella issue, one
that asks for sub-issues to be filed and then worked, is worked as one PR per piece ("PRs" below) unless the user
opts in the proposal to have the sub-issues opened instead.

## Doing the work

- **Branches:** `<type>/<N>-<description>`, where N is the issue number, so a branch or worktree can be tied to its
  issue without a PR. An umbrella's pieces use the umbrella's number with their own descriptions. The worktree
  directory is `.claude/worktrees/<N-description>`.
- **Worktrees:** every branch is a worktree. Create, enter, copy, and later remove, as follows.
  - Create from the main checkout with
    `git worktree add --no-track -b <type/N-description> .claude/worktrees/<N-description> origin/main`. Without
    `--no-track` the branch's upstream would be `origin/main`, a plain `git push` would refuse, and a later
    `git pull --ff-only` would pull `main` into the branch. A failed `git worktree add` is a blocked-banner halt.
  - Enter with `EnterWorktree` and the directory's absolute `path`. Never `EnterWorktree` with a `name`: it would
    name the branch `worktree-<name>` with `/` turned into `+`, which breaks the branch convention.
  - Resume a leftover worktree the same way. A leftover PR whose worktree is gone gets one back with
    `git worktree add .claude/worktrees/<N-description> <type/N-description>` (no `-b`: it checks out the local
    branch, or, when only `origin/<type/N-description>` exists, creates it tracking that), then
    `git pull --ff-only origin <type/N-description>` inside it so the branch matches the remote.
  - Copy, right after entering and before any subagent is dispatched: `.claude/CLAUDE.md`, `.claude/rules/` and,
    for a feature chunk, the chunk's spec, at the same paths (`mkdir --parents docs/superpowers/specs
    docs/superpowers/plans` first). They are gitignored in the main checkout and so absent in a new worktree. The
    copies are throwaway: a change to the spec, the plan, or a rule or rule override is made in the main checkout's
    file and copied again; a change made only in the worktree dies with it.
  - Remove in Merging step 4.
- **Features:** follow `feature-flow.md`.
- **Bugs:** follow `bug-flow.md`.
- **PRs:** one per issue, so each merges and reverts on its own. Closely related bugs that touch the same code may
  share a PR with at least one commit per issue. An umbrella issue gets one PR per piece, each from its own worktree
  and plan (`feature-flow.md` step 6 runs once per piece); the earlier PRs carry `Part of #N` and the last one
  `Closes #N`. Push a branch with `git push --set-upstream origin <type/N-description>` as soon as the local gate
  passes (the pre-push hook runs it; never bypass it with `--no-verify`), and open its PR early, so unfinished work
  stays findable. The body uses only `Closes #N` or `Part of #N`, never another closing keyword, carries every
  `Ruling:` the work made (from the ledger for plan-driven work, from the session's own calls otherwise), and ends
  with the marker.
- **PR title:** `type: subject`, at most 65 characters. The merge commit's header is the title followed by the PR
  number in parentheses, and `ci.yml` runs commitlint over every push to `main`, so a longer or unconventional title
  turns `main` red after a green merge.
- **Review:** every PR is reviewed before merge, in one of two ways.
  - Work done through superpowers:subagent-driven-development or superpowers:executing-plans is reviewed by their
    own loops, which run as those skills define them; no extra PR review follows. Before the PR opens, every in-scope
    finding their ledger deferred is fixed in the branch, given a known-gap comment, or dropped. Out-of-scope notes in
    the ledger are findings: judge each by the filing bar and move the ones that pass to `pending-findings.md`. For
    plan-driven work this sweep is the only time out-of-scope findings are written; a finding a subagent reports
    mid-plan goes in the ledger, not straight to the file, so nothing is recorded twice.
  - All other work gets superpowers:requesting-code-review before merge, with the base at
    `git merge-base origin/main HEAD`. A round is one reviewer pass plus the fixes it prompts; at most two rounds.
    Round-two findings about the change itself are handled as the next item says, with no further review; anything
    else still goes through the filing bar and "Out-of-scope findings" (a same-PR fix made after round two also
    gets no review).
  - A leftover PR from an earlier session gets a superpowers:requesting-code-review pass before it merges whichever
    way it was built, since no record of an earlier review survives a session.
- **Findings about the change itself** (a hole in the session's own fix, a gap in its own new test) are fixed in the
  branch, given a known-gap comment, or dropped. They never become an issue or a pending finding, even when they pass
  the filing bar. Anything else a review turns up, such as a bug in nearby code the change did not touch, is an
  ordinary finding: judge it by the filing bar and the out-of-scope rules.
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
is never reported, recorded or filed.

## Out-of-scope findings

A finding that passes the bar but lies outside the chunk:

- **Same PR** when it is a bug, the fix is under about 20 changed lines plus its test, and the fix lands in files or a
  shared helper the PR already changes: its own `test:` commit, then its own `fix:` commit. "The matches the change
  owns" in the red flags below means exactly this set.
- **Otherwise** append it to `pending-findings.md` the moment it is found (for plan-driven work, at the ledger sweep
  in "Review"), and carry on.

The session never opens an issue, PR comment or `TODO` for an out-of-scope finding. In this skill, "report them" in
the global "Fixing Findings" rule means `pending-findings.md` and nothing else. The known-gap comment, and the global
rule's `# TODO:` for a low-stakes ambiguity found mid-run, are separate and still allowed.

## Merging

The user's approval of the chunk is the approval for every push and merge in it, so none waits for the user. Every
command here runs inside the worktree unless it names the main checkout with `-C`:

1. Wait for CI with `gh pr checks <number> --watch --fail-fast`, then read the result from
   `gh pr checks <number> --json bucket`, since the exit code alone does not tell the cases apart (a cancelled check
   exits 0). `[]` means no checks reported yet, which happens right after the PR opens: wait a minute and run again,
   up to three times; still none is a failed `gh` call (blocked banner). A `fail` bucket is a check to fix: fix the
   cause and push again. A coverage-ratchet failure is fixed by a test where one is possible, otherwise by a new
   signed commit (never an amend) that raises the baseline and carries the `Coverage-Drop:` trailer exactly as README
   "Recording a rise" says, other trailers directly under it. A `cancel` bucket, or a failure that is plainly
   infrastructure (such as a Nix setup step failing before any check runs, or a job that hangs until its timeout),
   gets one `gh run rerun <run-id> --failed`, with the id from
   `gh run list --branch <branch> --workflow ci.yml --limit 1 --json databaseId --jq '.[0].databaseId'`
   (`vuln.yml` also runs on push, so the workflow filter matters), then this step repeats once from the watch. A
   non-zero exit with no JSON is a failed `gh` call. If CI is still red after the rerun, the session halts: tell the
   user with the blocked banner. CI is green when every bucket is `pass` or `skipping`.
2. When CI is green, `gh pr merge <number> --merge`: the repo allows merge commits only and deletes the remote
   branch itself. Do not pass `--delete-branch`; the local branch is checked out in the worktree and the delete
   would fail. Confirm with `gh pr view <number> --json state --jq .state` printing `MERGED`. A refused merge is
   read from `gh pr view <number> --json mergeStateStatus --jq .mergeStateStatus`:
   - `BEHIND`: the ruleset requires the branch to be up to date with `main`, and Renovate merges its own PRs
     unattended, so a branch that was current when CI started can fall behind. Rebase as the global GitHub rule
     says, inside the worktree: `git fetch origin`, note `git rev-parse origin/<branch>` as the old remote SHA,
     `git rebase origin/main`, confirm `git log --format='%G?' origin/main..HEAD` prints only `G` (any other letter:
     do not push, blocked banner), push with `git push --force-with-lease=<branch>:<old-remote-sha>` (the pre-push
     hook runs the full gate again, which takes minutes), and go back to step 1 for the new CI run. A rebase that
     conflicts is a change the user must see: `git rebase --abort`, blocked banner. Never `gh pr update-branch`; it
     rewrites the commits unsigned.
   - `BLOCKED` with green CI: a rule other than up-to-date refused it, such as an unresolved review thread (the
     ruleset requires every thread resolved) or an unsigned commit. Stop with the blocked banner and say which.
     Never pass `--admin`, which `gh` itself suggests, and never resolve a thread; both are the user's.
3. `git -C "<main checkout>" pull --ff-only`, so the merge is on local `main`. If the pull is refused (the user has
   committed on or dirtied local `main`), run `git -C "<main checkout>" fetch --prune origin` instead, leave `main`
   as the user left it, and say in the summary that the next session's preconditions will fail until the user
   reconciles it; the next worktree starts from `origin/main` either way.
4. `ExitWorktree` with `action: keep` (it cannot remove a worktree entered by `path`), then from the main checkout
   `git worktree remove .claude/worktrees/<N-description>` and `git branch --delete <type/N-description>`. Either
   may refuse: the delete does when local `main` does not yet hold the merge (step 3's refused pull), the remove
   when the worktree has a stray file. Leave a refused one in place and name it in the summary; never pass `--force`
   or `-D`. The next session's step 3 classifies it once `main` is current. The next branch starts from the updated
   `origin/main`.

## Ending early

If the session ends before the chunk is done, the unfinished issues stay open and step 3 of the next session picks up
their worktrees or PRs. Every finding is already in `pending-findings.md`.

## Never

- Cut a release, tag, or bump a version. Releases are the user's own step.
- Start a whole-repo audit or sweep.
- Open an issue, or close one, without the user's decision. The user decides in the walkthrough, or by approving the
  chunk, whose issues the merged PRs close.
- Force-add a gitignored file. The spec, the plan and this skill's state stay local.
- Force a delete: no `git branch -D`, no `git worktree remove --force`, no `gh pr merge --admin`.

## Red flags

| Thought | Reality |
| --- | --- |
| "The sub-skill says to ask before merging" | The chunk approval already answered; see `sub-skill-overrides.md`. |
| "The user's open PR closes an issue; I'll finish it" | Without the marker it is the user's, whatever it closes. |
| "Checks exited 0, so CI is green" | A cancelled check exits 0 too; green is every bucket `pass` or `skipping`. |
| "`gh` says to merge with `--admin`" | Bypassing a rule is the user's call. Blocked banner. |
| "The reviewer found another hole in my fix; that's a new issue" | Fix it in the branch, known-gap it, or drop it. |
