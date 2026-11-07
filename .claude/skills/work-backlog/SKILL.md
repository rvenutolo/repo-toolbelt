---
name: work-backlog
description: >-
  Works open issues one PR at a time: proposes a chunk, builds each in a worktree, merges on green CI after your OK.
  `plan` rehearses; `short` or `long` sizes the chunk; issue numbers name it. For many independent issues,
  /work-backlog-parallel is faster.
argument-hint: "[plan] [short|long] [<issue>...]"
disable-model-invocation: true
---

# Working the backlog

One run of this skill is one backlog session. The aim is a backlog that shrinks: the session closes issues and never
refills the queue with work nobody needs. Only the user's decision opens an issue.

The user's global instructions and this repo's `.claude/CLAUDE.md` and `.claude/rules/` still apply. Where this skill
settles something one of them would stop to ask the user about, this skill's answer wins, since nobody is there to
answer during the unattended part; the main cases are `sub-skill-overrides.md` (its first two entries apply to every
session), "Copied rules" under "Doing the work" and "Out-of-scope findings".

`resume.md` says what to do after a compaction, how a halt is recorded, and what a session that ends early leaves;
`state.md` gives the state files' formats. Read `resume.md` first after a compaction.

## How the session runs

The user takes part at fixed points and the rest runs unattended. A decision needs the user only when it changes
what gets built, or when it is a merge under shakedown; everything else the session settles itself and records as a
`Ruling:` ("Terms"). The user's points:

- **Bug chunk:** the chunk OK (also answering the proposal's halt questions), the permission request, each merge
  while shakedown is on, then the findings walkthrough at the end.
- **Feature chunk:** the chunk OK, the brainstorm, one batched question round, each merge while shakedown is on,
  then the findings walkthrough.
- **Either:** a question-banner halt when a decision changes what gets built (a not-a-bug finding, `bug-flow.md`
  step 3, is one), and a blocked-banner halt when something outside the session's control stops it: a failed `gh`
  call, what `merging.md` and "Long commands" name, commit signing that fails, or Docker unreachable or
  `sysctl net.ipv4.ip_forward` not 1 right before `./.ci/test-image`, the one step that needs a container (the
  user's global Docker rule forbids running one then; the halt on it happens there and nowhere else, and the
  preflight's `ip-forward` line is only a WARN). Both are recorded as `resume.md` "Halting" says.

## Arguments

The invocation's arguments, in any order:

- **`plan`:** a read-only rehearsal that changes nothing; `plan.md` says what each step does instead.
- **Issue numbers:** the chunk is exactly these issues, so step 6 skips choosing and classifies only them
  (`backlog-index <N>...`, `choosing.md`). Each must still be open and doable: a named number whose `state` column is
  not `OPEN` (a closed issue, or a pull request's number) is dropped with a note in the proposal. The bugs-first rule
  and the size limit give way, since the user chose, but shakedown still trims the chunk ("Shakedown"). Named issues
  of both kinds make the bugs this chunk and the features the runner-up, said in the proposal, because a chunk
  follows one flow.
- **`short` or `long`:** the chunk size, at most two PRs or at most four ("Size" in `choosing.md`); without one the
  size rule stands. Shakedown ignores both, since its chunk is one issue. `/work-backlog-parallel` reads the same two
  words as its batch sizes, so a habit formed there costs no question here.
- **Anything else:** ask with the question banner, listing the valid forms, before step 1.

## Terms

- **Chunk:** the set of issues the user approves for this session ("Choosing the chunk").
- **Failed `gh` call:** a `gh` command that still errors after one retry a minute later (a `sleep 60` in the
  background, "Long commands"). A long run meets transient network errors, so every `gh` call this skill makes gets
  that retry; a failed one is a blocked-banner halt, never a count logged or work chosen from a partial or guessed
  result.
- **Approval:** what authorizes the unattended part, every push and merge included, except that while shakedown is
  on each merge also waits for its own OK (`merging.md` step 2). For a bug chunk, the chunk OK; for a feature chunk,
  the chunk OK plus the question round of `feature-flow.md` step 5, before which nothing is pushed or merged.
- **Controller:** this session, as opposed to the subagents it dispatches.
- **Banners:** the `❓❓❓❓❓` (question) and `🚧🚧🚧🚧🚧` (blocked) lines the global "Attention Banners" rule
  requires as the first line of a message that waits for the user, and the `✅✅✅✅✅` (done) line, on the summary
  and on a `plan` run's proposal. A question-banner halt resumes after the answer; a blocked-banner halt ends the
  unattended part until the user has acted. The shakedown lift offer ("Shakedown") waits for an answer, so it is a
  question-banner follow-up after the summary, not part of it.
- **Halt question:** a question the unattended part would otherwise stop to ask; the global "Unattended Execution /
  Pre-Answer Halt Questions" rule calls them halt points. The chunk proposal (bugs) or the question round (features)
  asks every likely one in advance, each with a recommended answer and a one-line reason.
- **Permission request:** the message listing the allow rules the unattended part needs, asked before it starts
  (`permissions.md`).
- **Filing bar:** the two-part test a finding must pass to be recorded ("The filing bar").
- **Known-gap comment:** a one-line comment beside code stating a gap in the present tense, citing no issue or PR
  number: the repo's own issue-numbers lint fails CI on one, and the docs rules bar history from comments.
- **Ledger:** the progress file a plan sub-skill keeps at `.superpowers/sdd/<plan-basename>/progress.md` in the
  worktree (both superpowers:subagent-driven-development and superpowers:executing-plans use it). It holds task
  completions, every `Ruling:`, and the findings those skills park or defer; it is deleted after the ledger sweep
  (`sub-skill-overrides.md`).
- **Ledger sweep:** the controller's pass over the ledger before a plan-driven PR merges ("Review").
- **`Ruling:`:** one line recording a decision the session made where a sub-skill would have asked the user. Every
  ruling goes in the PR body ("PRs"). Calls made before approval (`feature-flow.md` steps 3 and 4) are recorded in
  the spec instead; they are design decisions, not rulings.
- **The marker:** the line `Backlog-Session: YYYY-MM-DD`, with the session's `session-date:` (`state.md`), the last
  line of every PR body this skill writes; the user never writes it, so it tells this skill's PRs from the user's. A
  PR is this skill's when its body has a line that is exactly `Backlog-Session: <date>`, wherever it stands, since a
  tool may append a line after it.

## State

Local files under `.claude/backlog/` in the main checkout; `state.md` gives their formats. Everything this skill
writes outside a PR is local and untracked: `.claude/` apart from `.claude/skills/`, `docs/superpowers/` and
`.superpowers/` are ignored by the user's global git excludes, not this repo's `.gitignore`. So the state files,
specs, plans, ledgers, worktrees and rule copies never appear in `git status` and are never force-added. Step 2
creates the directory and the first two files if any is missing.

- `session-log.md`: one line per session with its open-issue count, read by the stall check (step 5) and the trend line.
- `pending-findings.md`: out-of-scope findings waiting for the user's decision (`findings-walkthrough.md`).
- `current-session.md`: the resume record: the approved chunk, its answers, and how far each issue got. Step 6
  writes it (or `findings-walkthrough.md` step 6, earlier), step 3 reconciles it, step 9 deletes it, and `resume.md`
  reads it. One a parallel run owns is sorted by `leftovers.md`.
- `parallel/`: per-issue progress and findings files `/work-backlog-parallel` writes (its `SKILL.md` "State"). This
  skill writes them only when `leftovers.md` records a finished run's result, and folds any it finds.
- `config`: the user's settings for both skills ("Shakedown", "Budgets"), written only on the user's word; never a
  leftover.
- `reports/`: one file per finished session, named by its `session-date:` (`report.md`); kept, never a leftover.

Every path in this skill is relative to the main checkout. Work happens in worktrees under `.claude/worktrees/`, so
from inside one, the main checkout is `$(dirname "$(git rev-parse --path-format=absolute --git-common-dir)")`. The
state files are written there and nowhere else, and only from the main checkout ("Leaving to write"); reading them
from inside a worktree is fine.

Every line appended to `current-session.md` after its first starts with the UTC time, and one `at:` line names the file
and step the session is in, rewritten at each step (`state.md`), so a dead session's record says when it stopped and
where. `<main>/.claude/skills/work-backlog/backlog-status` reads these files, the parallel run's files, the session's
open PRs, the worktrees and the local branches merged and not merged into `main`, and prints a session's state; it is
read-only and runs from any terminal, so the user can watch an unattended run without touching the session, and
`leftovers.md` opens with it.

## The session

1. **Preconditions.** First run `permission-diff` over the "Run while the user is there" rules (`permissions.md`):
   when any is missing, ask once with the question banner, `write the rules` (recommended) or `continue`, so a first
   run meets one question instead of a prompt per command before the proposal. Then `git status --porcelain` prints
   nothing; then, after `git switch main` (the user's branch stays as it was, checked out nowhere), run
   `<main>/.claude/skills/work-backlog/backlog-preflight`: one `PASS`, `WARN` or `FAIL` line per check (auth, clean
   tree, `main` current with `origin/main`, the signing key, the superpowers version the overrides quote, the config
   file, the notification hook, and more), so every prerequisite a run needs is learned here, not from a failed step
   hours later. A `FAIL` line is a blocked banner quoting it; `WARN` lines ride in the proposal (`choosing.md` "The
   proposal"). Then `git fetch --prune origin` and `git pull --ff-only` must succeed; the prune is what later marks a
   merged branch's upstream `[gone]`. If any of these fails, stop with the blocked banner before doing anything else.
   Every later `gh` call is retried as "Failed `gh` call" ("Terms") says.
2. **Log.** Create `.claude/backlog/`, `session-log.md` and `pending-findings.md` if any is missing (never
   `current-session.md`: its existence means unfinished work; step 3 reads it). Append `YYYY-MM-DD open=N` to
   `session-log.md`, N from `gh issue list --state open --limit 1000 --json number --jq length`, unless today's line
   exists without `done`: that is this session restarted, so keep it and add none. A line whose date is the
   `session-date:` of a carried-over `current-session.md` holding `workflow-*` lines belongs to that run, even
   today: append this session's own line. Any other line, whatever its date, is another session's.
3. **Unfinished work.** Normally there is none: step 2 found no `current-session.md`, `.claude/backlog/parallel/` is
   empty or absent, and after `git worktree prune`, `git worktree list` shows only the main checkout,
   `gh pr list --author @me --state open` is empty, and `git branch` shows `main` alone. Then go to step 4.
   Otherwise follow `leftovers.md`, which sorts what is there into this skill's unfinished work (finished before
   anything new is chosen), merged branches to delete, and the user's own work, which is left alone.
4. **Leftover findings.** If `pending-findings.md` has entries, walk through them with `findings-walkthrough.md`.
   This runs after step 3 so that a "fix it now" decision does not start new work while a leftover is unfinished.
5. **Stall check.** Compare today's line with the third line above it ending in `done` (three finished sessions
   earlier, however many calendar days apart; lines without `done` are skipped). If that line exists and today's count
   is not lower than its count, the backlog has not shrunk over three sessions: the proposal's trend line says so and
   offers `triage` as a reply, and `triage.md` runs only on that reply. A backlog of deliberately filed families
   stalls by design while the user files faster than a session closes, so an unasked triage would cost a long sitting
   for nothing.
6. **Choose the chunk.** Read `.claude/backlog/config` ("Shakedown", "Budgets"): an unreadable value is the default,
   and the preflight's `WARN config` line says so in the proposal. Choose the chunk, or take the issues the arguments
   name ("Arguments"), propose it as `choosing.md` says, and wait for the user's reply from its `Reply` line (`triage`
   runs `triage.md`, then the proposal is sent again); with `plan`, print the proposal and end there instead. Then write
   `current-session.md` with its `session-date:` (the date of the line step 2 appended), the `flow:` and `issue` lines
   (`state.md`), the halt answers, the allowed commands and, for a bug chunk, the `started:` line ("Budgets"), since the
   OK begins the unattended part; a file that already exists is added to as `leftovers.md` says, and one a running
   parallel run owns means this session chooses no chunk and goes to step 8 (`leftovers.md` "A parallel run's files").
7. **Do the work** (below), one PR per issue, merging each on green CI.
8. **New findings.** Walk through every entry in `pending-findings.md` with `findings-walkthrough.md`.
9. **Summary.** Write the end-of-run report (`report.md`) and send it as one message. Then append `done` to the
   `session-log.md` line step 2 appended or kept (its date, not today's: a run crosses midnight, and step 6 copies that
   date into `session-date:`). Delete `current-session.md`, unless `leftovers.md` says a running parallel run owns it
   (leave it as it is), or it holds parked issues, a PR recorded `pr open, not merged` by a shakedown "no", or a `#<N>
   left to user` line: trim it to those issues and lines, as `/work-backlog-parallel`'s "Ending" step 4 says, so the
   next session's step 3 still knows the first "no" was given.

## Choosing the chunk

`choosing.md` holds the detail; in short: run `backlog-index`, classify from its rows (bug by label, `fix:` title or a
body that reproduces something wrong now; otherwise a feature), bugs first when any doable bug is open, related
issues together in dependency order, two to three PRs (one under shakedown; `short` or `long` sets it), then one
proposal message in a fixed layout with the runner-up, the trend, the halt questions, the permission request and the
reply forms. The user's OK on it is the approval ("Terms").

## Doing the work

The loop for one issue:

1. Check the issue is still open: `gh issue view <number> --json state --jq .state`. `CLOSED` means the user decided
   against it since the chunk was approved: skip it, leave its part of any spec unbuilt, name it in the summary, and
   go on to the next. Then check the time budget ("Budgets"); a spent one starts no further issue.
2. Create a worktree and enter it, copy the instruction files in, and run the baseline tests ("Worktrees").
3. Build the change (`feature-flow.md` or `bug-flow.md`). Read `sub-skill-overrides.md` before the session's first
   sub-skill invocation: it names the pauses this skill has already settled.
4. Push as soon as the local gate passes, and open the PR ("PRs").
5. Review ("Review").
6. Merge on green CI, leave the worktree, pull `main`, remove the worktree ("Merging", `merging.md`).

Strictly one issue at a time: the next worktree is created only after the previous PR has merged, because it starts
from the merged `origin/main` and, in a family, builds on the earlier piece's code. The bullets below are the rules
those steps refer to.

- **Branches:** `<type>/<N>-<description>`, where N is the issue number, so a branch or worktree can be tied to its
  issue without a PR. An umbrella's pieces use the umbrella's number with their own descriptions. The worktree
  directory is `.claude/worktrees/<N-description>`. A change with no issue (a "fix it now" or a known-gap comment
  from `findings-walkthrough.md`) has no N: its branch is `fix/finding-<slug>` or `docs/finding-<slug>` and its
  worktree `.claude/worktrees/finding-<slug>`, with the slug taken from the finding's symptom line, so `leftovers.md`
  can tell it apart from the user's branches.
- **Permissions:** `permissions.md`: the request, its rule strings, when it is asked, the signing check.
- **Worktrees:** every branch is a worktree.
  - Create from the main checkout with
    `git worktree add --no-track -b <type/N-description> .claude/worktrees/<N-description> origin/main`. Without
    `--no-track` the branch's upstream would be `origin/main`, a plain `git push` would refuse, and a later
    `git pull --ff-only` would pull `main` into the branch. A failed `git worktree add` is a blocked-banner halt.
  - Enter with `EnterWorktree` and the directory's absolute `path`. Never `EnterWorktree` with a `name`: it would
    name the branch `worktree-<name>` with `/` turned into `+`, which breaks the branch convention. Both tools'
    descriptions ask for a direction from the user or the project before use; this skill is that direction, so the
    calls are never proactive.
  - Copy, right after entering and before any subagent is dispatched: `.claude/CLAUDE.md`, `.claude/rules/` and,
    for a feature chunk, the chunk's spec, at the same paths (`mkdir --parents docs/superpowers/specs
    docs/superpowers/plans` first). They are local files ("State") and so absent in a new worktree. The copies are
    throwaway: a change to the spec, the plan, a rule or rule override, or `.claude/CLAUDE.md` is made in the main
    checkout's file ("Leaving to write") and copied again; a change made only in the worktree dies with it. A change
    that makes `.claude/CLAUDE.md` wrong (a lint, formatter, command or layout it describes) updates it this way, in
    the same issue's work, as the global docs rule asks of every doc.
  - Then run the baseline tests, `./run-tests` ("Long commands"), for bugs and features alike: a
    failure that was already on `main` would otherwise be blamed on the change. Judge a red baseline as the
    using-git-worktrees entry of `sub-skill-overrides.md` says.
  - Resume a leftover worktree or PR as `leftovers.md` says. Leave and remove in `merging.md` steps 3 and 4.
- **Leaving to write:** a session that entered a worktree with `EnterWorktree` is isolated in it. Claude Code
  refuses the Write and Edit tools on a main-checkout path and `git -C` on the main checkout; reading is allowed.
  So every write to a main-checkout file (the state files, a plan copy, a spec, rule or `.claude/CLAUDE.md`
  correction) is made from the main checkout:
  1. Never while an Agent call is outstanding; a background Bash command keeps its own directory and may run on.
  2. `ExitWorktree` with `action: keep`, then make the write.
  3. `EnterWorktree` with the worktree's absolute `path` again, and confirm `git rev-parse --show-toplevel` prints
     that path before the next command.

  Writes that fall together share one round trip. Never write a main-checkout file from inside a worktree with Bash
  (`cp`, a redirect): it may get through only because the guard checks the file tools and `git`, not Bash paths, and
  a stricter guard would stop an unattended run partway.
- **Long commands:** the pre-push gate (`git push`), the baseline tests, the baseline regeneration and the CI watch
  each run for minutes, past the Bash tool's two-minute default and sometimes its ten-minute foreground maximum. Run
  them with `run_in_background` and read the output when the command exits (a foreground timeout is not a failure).
  Each runs as `timeout 30m <command>` (the global rule for commands that could hang; permission matching strips the
  `timeout`, so the command's own allow rule still applies). Exit 124 means it hung: a blocked-banner halt naming the
  command and its last output, never a silent rerun. A step that says to wait a minute runs `sleep 60` with
  `run_in_background` and goes on when its completion notification arrives; never a foreground `sleep`, which the
  harness blocks. `git commit` is a long command of another kind: both commit hooks run through the devShell (the
  pre-commit hook also builds the runner, the commit-msg hook runs commitlint), minutes the first time, so run it in the
  foreground with the Bash tool `timeout: 600000`, with no `timeout 30m` and no background.
- **Copied rules:** a conflict with a copied rule in `.claude/rules/` that only surfaces during the unattended part
  is settled by satisfying the rule, and named in the summary. `.claude/CLAUDE.md` asks the user about such a
  conflict, but nobody is there to answer, and satisfying the rule is the choice that never needs undoing. A conflict
  known before the unattended part is asked instead, with the chunk proposal (bugs) or in the question round
  (features, `feature-flow.md` step 5).
- **PRs:** one per issue, so each merges and reverts on its own; related bugs never share one, so dropping one bug
  never means rewriting a pushed branch another bug's PR needs. An umbrella issue gets one PR per piece, each from
  its own worktree and plan (`feature-flow.md` step 6 runs once per piece); the earlier PRs carry `Part of #N` and
  the last one `Closes #N`. Push a branch with `git push --set-upstream origin <type/N-description>` as soon as the
  local gate passes (the pre-push hook runs it; never bypass it with `--no-verify`), and open its PR early, so
  unfinished work stays findable.
  - The body uses only `Closes #N` or `Part of #N`, never another closing keyword, each on a line by itself, because
    both skills tie a PR to its issue by matching that whole line (a substring match would take a PR for a longer
    issue number as this one's). It ends with the marker: an attribution line (such as "Generated with Claude Code")
    goes above the `Closes #N` or `Part of #N` line, never after the marker. Every line of the body is at most 100
    characters while `.commitlintrc.yml` leaves commitlint's footer rule at its inherited 100: the merge commit's
    message is the PR body, and that rule reads every line after the first `Word:` line as footer. Wrap a long ruling
    onto continuation lines indented two spaces.
  - The body carries every `Ruling:` the work made (from the ledger for plan-driven work, from the session's own
    calls otherwise), since nothing else survives the worktree. A ruling made after the PR opened is added with
    `gh pr edit <number> --body` (read the body first and add the rulings above the marker, so nothing already there
    is lost): at the ledger sweep for plan-driven work, before the sub-skill deletes the ledger, and before the merge
    for other work.
  - A body edit goes before a push, never after: CI re-runs on the `edited` event and cancels the run in progress,
    so an edit right after a push throws away the CI run that push started. With nothing left to push, edit before
    the CI watch starts.
- **PR title:** `type: subject`, at most 72 characters minus the merge header's suffix (a space, then `(#N)`: 4 plus
  the PR number's digits), so 65 while PR numbers have three digits. The merge header is the title plus that suffix,
  and `ci.yml` runs commitlint over every push to `main`, so a longer or unconventional title turns `main` red after
  a green merge.
- **Review:** every PR is reviewed before merge, in one of two ways.
  - Work done through superpowers:subagent-driven-development or superpowers:executing-plans is reviewed by their
    own loops, which run as those skills define them; no extra PR review follows. Before the PR merges, and before
    the sub-skill deletes its workspace, the controller runs the ledger sweep over every in-scope finding the ledger
    deferred or parked, handled as "Findings about the change itself" says, and every out-of-scope note, judged by
    the filing bar; the ones that pass move to `pending-findings.md` (one round trip out of the worktree, "Leaving
    to write"), and the PR body gets the rulings the ledger holds ("PRs"). For plan-driven work this sweep is the
    only time out-of-scope findings are written; a finding a subagent reports mid-plan goes in the ledger, not
    straight to the file, so nothing is recorded twice. `sub-skill-overrides.md` says how each sub-skill makes the
    fixes.
  - All other work gets superpowers:requesting-code-review before merge, with the base at
    `git merge-base origin/main HEAD`. A round is one reviewer pass plus the fixes it prompts; at most two rounds,
    because a small change that still draws findings after two passes is a wrong change, not an under-reviewed one,
    and the cap bounds what an unattended run spends. Round-two findings about the change itself are handled as the
    next item says, with no further review; anything else still goes through the filing bar and "Out-of-scope
    findings" (a same-PR fix made after round two also gets no review). A merge-time fix is the exception: one scoped
    pass, as `merging.md` "Merge-time fixes" says. Record `#<N> review round <k>` in `current-session.md` as each
    round starts (`state.md`), with the round's other writes where it has any, so a compaction does not reset the
    cap to round one.
  - A leftover PR is reviewed as `leftovers.md` says.
- **Findings about the change itself** (a hole in the session's own fix, a gap in its own new test, a finding a plan
  sub-skill parked or deferred) are fixed in the branch, given a known-gap comment, or dropped. They never become an
  issue or a pending finding. The filing bar does not apply to them, nor to design findings (`feature-flow.md` step
  4): both are judged on whether the change is sound, so a hole in the session's own fix may be fixed even when only
  an unlikely input opens it. Anything else a review turns up, such as a bug in nearby code the change did not touch,
  is an ordinary finding: judge it by the filing bar and the out-of-scope rules.
- **Subagents:** every subagent prompt that implements or reviews tells the subagent to return every finding to the
  controller, including one a `Ruling:` in the PR body already rules on, and never to open an issue, comment on a
  PR, leave a `TODO` or decide for itself what is worth reporting. The filing bar is the controller's to apply,
  after the report; a reviewer told what not to flag reviews with a blind spot (`sub-skill-overrides.md`). A
  reviewer inspects the branch with `git diff` and `git show`, never by adding a worktree of its own, which the next
  session's step 3 would report as the user's work. The prompt also says: `git commit`, `./run-tests` and every
  `./.ci/in-devshell` command run in the foreground with the Bash tool `timeout: 600000`, never backgrounded and never
  retried while one may still be running. The user's two-minute default timeout would otherwise background them, and a
  subagent that retries a commit meets the first one's `index.lock`, or reports a task done before its tests finish.

## The filing bar

A finding counts only if both hold:

1. **It is real.** It actually happened, or a realistic input triggers it: input the user, a consuming repo or a bot
   would actually produce. An input built to break the tool does not count, even with a working reproduction.
2. **It is wrong now.** A hole that only a hypothetical future edit could open does not count.

A finding that fails the bar is never fixed: a guard or test for an input nobody writes is the speculative work the
spec review strikes. It gets a known-gap comment when the code is in a file the PR already changes, or is dropped. It
is never reported, recorded or filed. (A finding that passes the bar may also end as a known-gap comment, but only
by the user's choice in `findings-walkthrough.md`.) The bar judges findings about other code, not the change itself
("Findings about the change itself").

## Out-of-scope findings

A finding that passes the bar but lies outside the chunk:

- **Same PR** when it is a bug, the fix is under about 20 changed lines plus its test, and the fix lands in files or a
  shared helper the PR already changes: its own `test:` commit, then its own `fix:` commit. The limits keep the PR one
  reviewable change whose revert takes nothing unrelated; judge a case near them by that.
- **Otherwise** append it to `pending-findings.md` in the main checkout ("Leaving to write" from a worktree) the
  moment it is found (at the ledger sweep for plan-driven work, "Review"), and carry on.

The session never opens an issue, PR comment or `TODO` for an out-of-scope finding. In this skill, "report them" in
the global "Fixing Findings" rule means `pending-findings.md` and nothing else. A known-gap comment for a finding
that fails the bar, and the global rule's `# TODO:` for a low-stakes ambiguity found mid-run, are separate and still
allowed.

## Merging

`merging.md` gives the procedure, its traps and its exceptions: watching CI, a check that fails, a coverage-ratchet
failure, an infrastructure failure worth one rerun, a branch that fell behind `main` or conflicts with it, a merge
the ruleset refuses, and a pull or delete that is refused. Approval ("Terms") covers every push and merge in it.

A blocked-banner halt in `merging.md` stops the chunk at that issue. Leave the worktree and record the halt in
`current-session.md` (`resume.md` "Halting"), then tell the user what stopped it and what the session would do
next; after the user's reply, go on with the issues that do not depend on the halted one, or stop. Steps 8 and 9
then run from the main checkout, except that a halt caused by a failed `gh` call skips step 8: the walkthrough needs
`gh` to open issues, and the findings wait in `pending-findings.md` for the next session. The summary says so. A
session that ends before the chunk is done leaves what `resume.md` "Ending early" describes.

## Shakedown

Until the user has watched one run go through end to end, an unattended merge is a step taken on trust, so both
backlog skills start in shakedown: `shakedown: on` is the default while `.claude/backlog/config` does not say `off`
(step 6 reads the file). `/work-backlog-parallel` reads its own key, `shakedown-parallel:`, with the same default,
because a run that merged serially has not exercised its merge queue; lifting one never lifts the other. While it is
on, a chunk is one issue: form and order the chunk as usual, then take its first issue as the chunk and name the rest
as the runner-up, ahead of the runner-up chunk (`choosing.md` "The proposal"); with named issues, the first named
bug, or the first named issue when none is a bug. Its merge waits for the user's OK: `merging.md` step 2 asks with the
question banner once CI is green, recorded as `resume.md` "Halting" says. After the summary, a question-banner follow-up
offers to lift it ("Banners"); on the user's yes, write the skill's own key off (`shakedown: off` here) to the config
(create the file, or replace the key's line). The user may also turn it off or on with the chunk OK. Turned off with the
OK: write `shakedown: off` then, and send a full-size proposal (its halt questions included) as a follow-up before
starting (`permissions.md` "When to ask" says why a follow-up is allowed). Turned on: the chunk is its first issue.

## Budgets

An unattended run that goes badly should stop costing time on its own, so the config bounds it: `max-hours:` (default 8)
and `max-parked:` (default 2). The time runs from the `started:` line the session writes to `current-session.md` as
the unattended part begins, `started: <date --utc +%FT%TZ>`, replacing one a carried-over file holds: step 6 writes
it with a bug chunk's OK, `feature-flow.md` step 5 with the answered question round, and `findings-walkthrough.md`
step 6 when a "fix it now" makes a worktree. The deadline is `started:` plus `max-hours`,
`date --utc --date "<started> +<max-hours> hours" +%FT%TZ`; before each issue the loop compares `date --utc +%FT%TZ`
with it as strings, which orders correctly in that form. Once the deadline has passed, no further issue starts: the
loop's step 1 stops there, the issues not started stay open for the next session, and the session goes to step 8
after the current issue merges or halts. The proposal states `max-hours` and shakedown; `max-parked` is
`/work-backlog-parallel`'s alone, whose halts park an issue and go on, and here every halt already waits for the user.

## Never

- Cut a release, tag, or bump a version. Releases are the user's own step.
- Start a whole-repo audit or sweep: it refills the backlog the session exists to shrink.
- Open an issue, or close one, without the user's decision. The user decides in the walkthrough, or by approving the
  chunk, whose issues the merged PRs close.
- Force-add a local file ("State").
- Force a delete: no `git branch -D`, no `git worktree remove --force`, no `gh pr merge --admin`.
- Text-merge `.coverage-baseline.txt`: a rebase that touches it regenerates it (`merging.md` step 2, "Baseline after
  a rebase").
- Edit `CHANGELOG.md`; the release tooling writes it.

## Red flags

| Thought | Reality |
| --- | --- |
| "The sub-skill says to ask before merging" | Answered by approval, unless shakedown is on; `sub-skill-overrides.md`. |
| "The chunk is approved, so I can push this feature" | Not before the question round; see "Approval" in Terms. |
| "The user's open PR closes an issue; I'll finish it" | Without the marker it is the user's, whatever it closes. |
| "CI is watching; I'll start the next worktree meanwhile" | One issue at a time; the next starts from the merge. |
| "Checks exited 0, so CI is green" | A cancelled check exits 0 too; green is every bucket `pass` or `skipping`. |
| "`gh` says to merge with `--admin`" | Bypassing a rule is the user's call. Blocked banner. |
| "The reviewer found another hole in my fix; that's a new issue" | Fix it in the branch, known-gap it, or drop it. |
| "The push timed out or was killed; rerun it" | A tool timeout is no failure; exit 124 is a hang. "Long commands". |
| "`gh pr checks` errored right after the PR opened" | "No checks reported" means none has started; wait, run again. |
| "The leftover ledger has open tasks; sweep and merge" | Open tasks mean the plan resumes first, then the sweep. |
| "The user said the commands are allowed, so no prompt" | Only an allow rule or bypass mode does; `permissions.md`. |
| "A body edit after the push is harmless" | It cancels the CI run the push started; edit before the push ("PRs"). |
| "`cp` into the main checkout works from here" | The guard misses Bash; leave first ("Leaving to write"). |
