---
name: work-backlog
description: >-
  Works this repo's open GitHub issues one PR at a time: proposes a chunk of related bugs or features, builds each in
  its own worktree, and merges each on green CI once the user approves the chunk (the first, shakedown runs ask
  before each merge). Use when starting a session to work through the issue backlog, or when asked to pick up the
  next backlog issue, bug or feature.
  Arguments: `plan` for a read-only proposal that changes nothing, and issue numbers to name the chunk. For a long
  run of independent issues, /work-backlog-parallel finishes sooner.
argument-hint: "[plan] [<issue>...]"
disable-model-invocation: true
---

# Working the backlog

One run of this skill is one backlog session. The aim is a backlog that shrinks: the session closes issues and never
refills the queue with work nobody needs. Only the user's decision opens an issue.

The user's global instructions and this repo's `.claude/CLAUDE.md` and `.claude/rules/` still apply, with two
exceptions this skill makes on purpose: `sub-skill-overrides.md` lists the points where it replaces a pause of a
superpowers skill it calls, and "Copied rules" (under "Doing the work") settles a copied-rule conflict found during
unattended work by satisfying the rule instead of asking, as `.claude/CLAUDE.md` otherwise requires. Read
`sub-skill-overrides.md` before the first sub-skill invocation of the session; its first two entries (the subagent
clause and the skill that is never invoked) apply to every session, plan-driven or not.

`resume.md` says what to do after a compaction, how a halt is recorded, and what a session that ends early leaves;
`state.md` gives the state files' formats. Read `resume.md` first after a compaction.

## How the session runs

The user takes part at fixed points and the rest runs unattended. A decision needs the user only when it changes
what gets built; everything else the session settles itself and records as a `Ruling:` ("Terms"). Every place that
says a decision is "settled as 'How the session runs' says" means this test. The user's points:

- **Bug chunk:** the chunk OK (which also answers the halt questions the proposal carries), the permission request,
  then the findings walkthrough at the end.
- **Feature chunk:** the chunk OK, the brainstorm, one batched question round, then the findings walkthrough.
- **Either:** a question-banner halt when a decision changes what gets built, and a blocked-banner halt when
  something outside the session's control stops it (a failed `gh` call, red CI after a rerun, a refused merge,
  commit signing that fails, a long command that hangs past its 30 minutes ("Long commands"), Docker unreachable or
  `sysctl net.ipv4.ip_forward` not 1 when a step needs a container, since the user's global Docker rule forbids
  running one then). Both are recorded as "Halting" in `resume.md` says.

## Arguments

The invocation's arguments, in any order:

- **`plan`:** plan only, so a run can be rehearsed. It changes nothing: step 1 runs `git fetch --prune origin` and
  `gh auth status` only (no switch, no pull, a dirty tree is fine); step 2 takes the open-issue count for the
  proposal's trend line but appends no log line and creates no state file; step 3 runs `git worktree prune
  --dry-run` in place of the prune and lists what it finds without acting on it; step 4 names the pending findings
  without walking them; step 5 compares the count step 2 took, standing in for today's line, and says whether triage
  would run instead of running it. Step 6 then prints the proposal, its halt questions and the permission request
  (`permissions.md`) as it would send them; for a feature chunk, whose questions and request belong to
  `feature-flow.md` step 5, it prints the ones it can already see and names the spec it would write. `plan`'s one
  message begins with the done banner; the proposal is printed without its question banner, since nothing is
  awaited. The printed proposal is not approvable, because the skill cannot re-invoke itself: to run it, invoke
  `/work-backlog` again, with the issue numbers to keep this chunk. Scratch files go to `.claude/tmp/` only; nothing
  is written under `.claude/backlog/`, so a rehearsal leaves nothing the next real run's step 3 would sort as a
  leftover.
- **Issue numbers:** the chunk is exactly these issues, so step 6 skips choosing and classifies only them
  (`backlog-index <N>...`, "Choosing the chunk"). Each must still be open and doable: a named number whose `state`
  column is not `OPEN` (a closed issue, or a pull request's number) is dropped with a note in the proposal. The
  bugs-first rule and the size limit give way, since
  the user chose, but shakedown does not: while it is on, the chunk is the first named bug, or the first named issue
  when none is a bug, and the proposal says so ("Shakedown"). Named issues of both kinds make the bugs this chunk and
  the features the runner-up, said in the proposal, because a chunk follows one flow.
- **Anything else** (including `short` and `long`, which are `/work-backlog-parallel`'s batch sizes): ask with the
  question banner, listing the valid forms, before step 1.

## Terms

- **Chunk:** the set of issues the user approves for this session ("Choosing the chunk").
- **Approval:** what authorizes the unattended part, every push and merge included, except that while shakedown is
  on each merge also waits for its own OK (`merging.md` step 2). For a bug chunk it is the chunk OK. For a feature
  chunk it is the chunk OK plus the question round of `feature-flow.md` step 5; nothing is pushed or merged before
  that round is answered.
- **Controller:** this session, as opposed to the subagents it dispatches.
- **Banners:** the `❓❓❓❓❓` (question) and `🚧🚧🚧🚧🚧` (blocked) lines the global "Attention Banners" rule
  requires as the first line of a message that waits for the user, and the `✅✅✅✅✅` (done) line, on the summary
  and on a `plan` run's proposal. A question-banner halt resumes after the answer; a blocked-banner halt ends the
  unattended part until the user has acted.
- **Halt question:** a question the unattended part would otherwise stop to ask; the global "Unattended Execution /
  Pre-Answer Halt Questions" rule calls them halt points. The chunk proposal (bugs) or the question round (features)
  asks every likely one in advance, each with a recommended answer and a one-line reason.
- **Permission request:** the message that lists the allow rules the unattended part needs and asks the user to add
  them before it starts (`permissions.md`).
- **Filing bar:** the two-part test a finding must pass to be recorded ("The filing bar").
- **Known-gap comment:** a one-line comment beside code stating a gap in the present tense. It cites no issue or PR
  number because the repo's own issue-numbers lint fails CI on one, and because the docs rules bar history from
  comments.
- **Ledger:** the progress file a plan sub-skill keeps at `.superpowers/sdd/<plan-basename>/progress.md` in the
  worktree (both superpowers:subagent-driven-development and superpowers:executing-plans use it). It holds task
  completions, every `Ruling:`, and the findings those skills park or defer. The sub-skill deletes it at its own
  finish.
- **Ledger sweep:** the controller's pass over the ledger before a plan-driven PR merges ("Review").
- **`Ruling:`:** one line recording a decision the session made where a sub-skill would have asked the user. Every
  ruling goes in the PR body ("PRs"). Calls made before approval (`feature-flow.md` steps 3 and 4) are recorded in
  the spec instead; they are design decisions, not rulings.
- **The marker:** the line `Backlog-Session: YYYY-MM-DD`, with the session's `session-date:` (`state.md`), that ends
  every PR body this skill writes. The user never writes it, so it is what tells this skill's PRs from the user's.

## State

Local files under `.claude/backlog/` in the main checkout; `state.md` gives their formats. Everything this skill
writes outside a PR is local and untracked: `.claude/` apart from `.claude/skills/`, `docs/superpowers/` and
`.superpowers/` are ignored by the user's global git excludes, not this repo's `.gitignore`. So the state files,
specs, plans, ledgers, worktrees and rule copies never appear in `git status` and are never force-added. Step 2
creates the directory and the first two files if any is missing.

- `session-log.md`: one line per session with its open-issue count; the stall check (step 5) and the proposal's
  trend line read it.
- `pending-findings.md`: out-of-scope findings waiting for the user's decision (`findings-walkthrough.md`).
- `current-session.md`: the resume record: the approved chunk, its answers, and how far each issue got. Step 6
  writes it (or `findings-walkthrough.md` step 6, earlier), step 3 reconciles it, step 9 deletes it, and `resume.md`
  reads it. One a parallel run owns is sorted by `leftovers.md`.
- `parallel/`: per-issue progress and findings files `/work-backlog-parallel` writes (its `SKILL.md` "State"). This
  skill writes them only when `leftovers.md` records a finished run's result, and folds any it finds.
- `config`: the user's settings for both backlog skills ("Shakedown", "Budgets"). Written only on the user's word;
  never a leftover.
- `reports/`: one file per finished session, named by its `session-date:` ("End-of-run report"). Kept, never a
  leftover.

Every path in this skill is relative to the main checkout. Work happens in worktrees under `.claude/worktrees/`, so
from inside one, the main checkout is `$(dirname "$(git rev-parse --path-format=absolute --git-common-dir)")`. The
state files are written there and nowhere else, and only from the main checkout ("Leaving to write"); reading them
from inside a worktree is fine.

## The session

1. **Preconditions.** `git status --porcelain` prints nothing, `gh auth status --hostname github.com` succeeds, and
   after `git switch main` (the user's branch stays as it was, checked out nowhere), `git fetch --prune origin` and
   `git pull --ff-only` succeed; the prune is what later marks a merged branch's upstream `[gone]`. If any of these
   fails, stop and tell the user (blocked banner) before doing anything else. The same holds for every later `gh`
   call this skill makes: if one fails, retry it once after a minute (a long run meets transient network errors);
   if it fails again, stop with the blocked banner, and never log a count or choose work from a partial or guessed
   result.
2. **Log.** Create `.claude/backlog/`, `session-log.md` and `pending-findings.md` if any is missing (never
   `current-session.md`: its existence means unfinished work, which step 3 reads it for). Append `YYYY-MM-DD open=N`
   to `session-log.md`, N from `gh issue list --state open --limit 1000 --json number --jq length`, unless today's
   line exists without `done`: that is this session restarted, so keep it and add none. Any other line, whatever its
   date, is another session's.
3. **Unfinished work.** Normally there is none: step 2 found no `current-session.md`, `.claude/backlog/parallel/` is
   empty or absent, and after `git worktree prune`, `git worktree list` shows only the main checkout,
   `gh pr list --author @me --state open` is empty, and `git branch` shows `main` alone. Then go to step 4.
   Otherwise follow `leftovers.md`, which sorts what is there into this skill's unfinished work (finished before
   anything new is chosen), merged branches to delete, and the user's own work, which is left alone.
4. **Leftover findings.** If `pending-findings.md` has entries, walk through them with `findings-walkthrough.md`.
   This runs after step 3 so that a "fix it now" decision does not start new work while a leftover is unfinished.
5. **Stall check.** Compare today's line with the third line above it ending in `done` (three finished sessions
   earlier, however many calendar days apart; lines without `done` are skipped). If that line exists and today's count
   is not lower than its count, the backlog has not shrunk over three sessions: run `triage.md` before choosing work.
6. **Choose the chunk.** Read `.claude/backlog/config` ("Shakedown", "Budgets"): an unreadable value is the default,
   said in the proposal. Then choose the chunk (below), or take the issues the arguments name ("Arguments"), propose
   it, and wait for the user's OK; with `plan`, print the proposal and end there instead. The user may swap or trim
   it.
   A swapped-in issue gets the proposal's per-issue items (comments, earlier pieces, kind, halt questions) in a
   follow-up message; like the permission follow-up (`permissions.md`), it is not a second question round. Then write
   `current-session.md`, or add the chunk to the one that already exists (a step 4 fix started it, or `leftovers.md`
   left a parallel run's parked issues in it); never overwrite one. Either way its `session-date:` line names the
   date of the line this session's step 2 appended, and the halt answers and allowed commands go in with the chunk;
   for a bug chunk the `started:` line goes in now too ("Budgets"), since the OK begins the unattended part. A file
   `leftovers.md` says a running parallel run owns gets no chunk: the session chooses none, says why, and goes to
   step 8, because that run's merge queue is the only thing that may pull `main` or touch worktrees while it runs,
   and a second merger would collide with it.
7. **Do the work** (below), one PR per issue, merging each on green CI.
8. **New findings.** Walk through every entry in `pending-findings.md` with `findings-walkthrough.md`.
9. **Summary.** Write the end-of-run report ("End-of-run report" gives its sections and what goes in each) and send
   it as one message. Then append `done` to the `session-log.md` line step 2 appended or kept (its date, not today's:
   a run crosses midnight, and step 6 copies that date into `session-date:`). Delete `current-session.md`, unless
   `leftovers.md` says a running parallel run owns it (leave it as it is) or left parked issues in it (trim it to
   them, as `/work-backlog-parallel`'s "Ending" step 4 says).

## Choosing the chunk

Run `<main>/.claude/skills/work-backlog/backlog-index` (with the issue numbers when the arguments name them), with
`<main>` the main checkout's absolute path, since its allow rule (`permissions.md`) names that path and a rule matches
the command's literal prefix. It writes every
open issue, body included, to `.claude/tmp/issues.json` and prints one tab-separated row per issue: `number`,
`state`, `kind`, `labels`, `title`, `repro`, `refs`, `umbrella`, `bytes`. The bodies run to well over what the Bash
tool shows, and a classification read from a cut-off listing misses issues, so classify from the rows and read a full
body (`jq --raw-output '.[] | select(.number == <N>) | .body' .claude/tmp/issues.json`) only for a row whose `kind`
is `?`, a non-bug row with `repro` `yes`, and every issue the proposal takes. A body the Bash tool would cut off
(`bytes` past about 16000) is written to `.claude/tmp/issue-<N>.md` instead and read with the Read tool, which
pages. `refs` and `umbrella` show the dependencies and umbrella issues "Order" and the proposal need. A non-zero
exit is a failed `gh` call (step 1).

An issue is a bug when it has the `bug` label, its title starts `fix:`, or its body reports something that is wrong
now with a way to reproduce it; otherwise it is a feature, whatever other labels it carries. The body counts because
few issues carry the label, and a three-line fix sent down the feature flow costs a brainstorm, a spec and a Fable
review. A check that fails on `main` today, or a config whose present value is wrong, is a reproduction; a `ci:` or
`chore:` title is a judgment call the proposal names. The `kind` column decides only the first two (`bug`, or
`feature` from a `feat:` title or the `enhancement` label); `?` is neither. The proposal names every issue whose kind
was a judgment call, so
the user can correct it with the chunk OK: a bug found from its body alone, and a `?` row. An issue whose kind
follows from its label or title is not named. A kind the OK corrects re-sorts the chunk: the issue follows the
corrected flow, and a chunk now holding both kinds splits as "Arguments" says, bugs this chunk and features the
runner-up. If any open issue is a bug the session can do, the chunk is bugs; otherwise it is features. A bug the
session cannot do at all (below) does not count, or it would block every later chunk until the user acts.

- **Bugs:** related bugs together (same code, same root cause). With none related, a batch of several small bugs.
- **Features:** one feature, or several that are closely related: they share code, one is a direct prerequisite of
  another, or they form a family that shares a runner, helper or template (such as the lints that wrap an external
  tool, or the shared workflows). One brainstorm covers a family.
- **Order:** by implementation sequence as the global "Prioritizing Work Items" rule defines it: what unblocks other
  items, then what speeds up the feedback loop, then what retires risk. Not by product value, and not by size. The
  first factor counts any open issue the item unblocks, in the chunk or not, and a prerequisite internal to the chunk
  that no issue names (a shared helper the pieces need) counts under it too.
- **Size:** two to four PRs; one while shakedown is on ("Shakedown"). Each PR waits on its own CI run and merges before
  the next starts, and the findings walkthrough still follows, so more than four rarely finishes in a session. When a
  family is bigger, propose the part that fits; the rest stays open for the next session, and the spec covers only the
  proposed part, with a shared helper designed so the rest can reuse it.
- **Combined changes:** an issue body or comment that asks for its change to land together with another's still
  gets one PR per issue ("PRs" below): order the two by the dependency between them, and say so in the proposal.
- **Comments:** for the issues in the proposed chunk, also read `gh issue view <number> --comments`. A decision the
  user left in a comment changes the proposal, and the list query returns bodies only.
- **Earlier pieces:** for each issue in the proposed chunk, also run
  `gh pr list --state merged --limit 1000 --search '"Part of #<number>" in:body' --json number,title,body` (the
  quotes keep the phrase whole; unquoted, the search matches the three words anywhere in a body), then confirm the
  body holds the whole line `Part of #<number>`, as `leftovers.md` matches it. A merged piece means an earlier
  session built part of this family or umbrella from a spec under `docs/superpowers/specs/` that names the issue:
  name that spec in the proposal, and `feature-flow.md` step 1 extends it instead of starting over.

The proposal is one message, beginning with the question banner, that names:

- the issues, why they belong together, and why they come first by the three ordering factors;
- the runner-up chunk and why it lost, so the user can swap with one word, or "none" when no other chunk could be
  formed (every doable issue of the chosen kind is already in this one);
- the user's own work step 3 saw and left alone, or "none";
- the backlog's trend, as `Backlog: <prev> → <now> open since <date>`, from the `open=` and date of the last
  `session-log.md` line before this session's and the count step 2 took (not `backlog-index`'s row count, which
  covers only the named issues when the arguments name some), or `Backlog: <now> open, first session` when this
  session's line is the first; the stall check sees only three sessions apart, and this shows growth between any two;
- the time budget and whether shakedown is on ("Budgets", "Shakedown"), so the user can change either with the OK,
  and, while shakedown is on, that the session will ask for the merge OK when CI is green, about fifteen minutes
  after the PR opens, so the user knows to stay or come back;
- when the chunk holds several independent issues and a long unattended run, that `/work-backlog-parallel` would
  finish it sooner;
- for a bug chunk, every likely halt question for the unattended part ("Terms"; `halt-questions.md` lists the kinds
  that recur, each with its default answer) and the permission request (`permissions.md`). The OK on the chunk
  answers the questions; the permission request needs its own answer. A feature chunk asks both in
  `feature-flow.md` step 5 instead;
- an issue whose own text asks the session to do something this skill forbids (open further issues, change another
  repo, cut a release), with the question it raises, so the user settles it with the chunk. An issue that describes
  a product which does such things, such as a workflow that opens issues, raises no question;
- an issue the session cannot do at all, such as one that needs a setting only the user can change in a GitHub App's
  settings or in another repo: leave it out of the chunk and name it, with what the user would have to do;
- an umbrella issue in the chunk, one that asks for sub-issues to be filed and then worked: it is worked as one PR
  per piece ("PRs" below) unless the user opts in the proposal to have the sub-issues opened instead.

## Doing the work

The loop for one issue:

1. Check the issue is still open: `gh issue view <number> --json state --jq .state`. `CLOSED` means the user decided
   against it since the chunk was approved: skip it, leave its part of any spec unbuilt, name it in the summary, and
   go on to the next. Then check the time budget ("Budgets"); a spent one starts no further issue.
2. Create a worktree and enter it, copy the instruction files in, and run the baseline tests ("Worktrees").
3. Build the change (`feature-flow.md` or `bug-flow.md`).
4. Push as soon as the local gate passes, and open the PR ("PRs").
5. Review ("Review").
6. Merge on green CI, leave the worktree, pull `main`, remove the worktree ("Merging", `merging.md`).

Strictly one issue at a time: the next worktree is created only after the previous PR has merged, because it starts
from the merged `origin/main` and, in a family, builds on the earlier piece's code. The bullets below are the rules
those steps refer to.

- **Branches:** `<type>/<N>-<description>`, where N is the issue number, so a branch or worktree can be tied to its
  issue without a PR. An umbrella's pieces use the umbrella's number with their own descriptions. The worktree
  directory is `.claude/worktrees/<N-description>`. A fix with no issue (a "fix it now" from
  `findings-walkthrough.md`) has no N: its branch is `fix/finding-<slug>` and its worktree
  `.claude/worktrees/finding-<slug>`, with the slug taken from the finding's symptom line, so `leftovers.md` can tell
  it apart from the user's branches.
- **Permissions:** the permission request, the rule strings it names, when it is asked and the signing check are in
  `permissions.md`. Read it when writing the chunk proposal (bugs) or the question round (features), and again right
  before the unattended part starts.
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
    throwaway: a change to the spec, the plan, a rule or rule override, or `.claude/CLAUDE.md` is made in the main
    checkout's file ("Leaving to write") and copied again; a change made only in the worktree dies with it. A change
    that makes `.claude/CLAUDE.md` wrong (a lint, formatter, command or layout it describes) updates it this way, in
    the same issue's work, as the global docs rule asks of every doc.
  - Then run the baseline tests, `./.ci/in-devshell ./run-tests` ("Long commands"), for bugs and features alike: a
    failure that was already on `main` would otherwise be blamed on the change. Judge a red baseline as the
    using-git-worktrees entry of `sub-skill-overrides.md` says.
  - Resume a leftover worktree or PR as `leftovers.md` says. Leave and remove in `merging.md` steps 3 and 4.
- **Leaving to write:** a session that entered a worktree with `EnterWorktree` is isolated in it. Claude Code
  refuses the Write and Edit tools on a main-checkout path and `git -C` on the main checkout; reading is allowed.
  So every write to a main-checkout file (the state files, a plan copy, a spec, rule or `.claude/CLAUDE.md`
  correction) is made from the main checkout:
  1. Never while an Agent call is outstanding; a background Bash command keeps its own directory and may run on.
  2. `ExitWorktree` with `action: keep`.
  3. Make the write.
  4. `EnterWorktree` with the worktree's absolute `path` again, and confirm `git rev-parse --show-toplevel` prints
     that path before the next command.

  Writes that fall together share one round trip. Never write a main-checkout file from inside a worktree with Bash
  (`cp`, a redirect): it gets through only because the guard checks the file tools and `git`, not Bash paths, and a
  stricter guard would stop an unattended run partway.
- **Long commands:** the pre-push gate (`git push`), the baseline tests, the baseline regeneration and the CI watch
  each run for minutes, past the Bash tool's two-minute default and sometimes its ten-minute foreground maximum. Run
  them with `run_in_background` and read the output when the command exits; a foreground timeout is not a failure of
  the command. Each runs as `timeout 30m <command>` (the global rule for commands that could hang; permission
  matching strips the `timeout`, so the command's own allow rule still applies). Exit 124 means it hung: a
  blocked-banner halt naming the command and its last output, never a silent rerun. A step that says to wait a
  minute runs `sleep 60` with `run_in_background` and goes on when its completion notification arrives; never a
  foreground `sleep`, which the harness blocks.
- **Features:** follow `feature-flow.md`.
- **Bugs:** follow `bug-flow.md`.
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
    issue number as this one's). It ends with the marker. Every line of the body is at most 100 characters: the
    merge commit's message is the PR body, and commitlint's footer rule, which the repo's config does not relax,
    reads every line after the first `Word:` line as footer. Wrap a long ruling onto continuation lines indented two
    spaces.
  - The body carries every `Ruling:` the work made (from the ledger for plan-driven work, from the session's own
    calls otherwise), since nothing else survives the worktree. This bullet is the one place that rule lives. A
    ruling made after the PR opened is added with `gh pr edit <number> --body` (read the body first and add the
    rulings above the marker, so nothing already there is lost): at the ledger sweep for plan-driven work, before
    the sub-skill deletes the ledger, and before the merge for other work.
  - A body edit goes before a push, never after: CI re-runs on the `edited` event and cancels the run in progress,
    so an edit right after a push throws away the CI run that push started. With nothing left to push, edit before
    the CI watch starts.
- **PR title:** `type: subject`, at most 65 characters. The merge commit's header is the title followed by the PR
  number in parentheses, and `ci.yml` runs commitlint over every push to `main`, so a longer or unconventional title
  turns `main` red after a green merge.
- **Review:** every PR is reviewed before merge, in one of two ways.
  - Work done through superpowers:subagent-driven-development or superpowers:executing-plans is reviewed by their
    own loops, which run as those skills define them; no extra PR review follows. Before the PR merges, and before
    the sub-skill deletes its workspace, the controller runs the ledger sweep: every in-scope finding the ledger
    deferred or parked is handled as "Findings about the change itself" says, every out-of-scope note is judged by
    the filing bar, the ones that pass moving to `pending-findings.md` (one round trip out of the worktree, "Leaving
    to write"), and the PR body is updated with the rulings the ledger holds ("PRs"). For plan-driven work this sweep
    is the only time out-of-scope findings are written; a finding a subagent reports mid-plan goes in the ledger,
    not straight to the file, so nothing is recorded twice. `sub-skill-overrides.md` says how each plan sub-skill
    makes the sweep's fixes.
  - All other work gets superpowers:requesting-code-review before merge, with the base at
    `git merge-base origin/main HEAD`. A round is one reviewer pass plus the fixes it prompts; at most two rounds,
    because a small change that still draws findings after two passes is a wrong change, not an under-reviewed one,
    and the cap bounds what an unattended run spends. Round-two findings about the change itself are handled as the
    next item says, with no further review; anything else still goes through the filing bar and "Out-of-scope
    findings" (a same-PR fix made after round two also gets no review).
  - A leftover PR from an earlier session gets a superpowers:requesting-code-review pass before it merges whichever
    way it was built, since no record of an earlier review survives a session. The one record that does is a
    parallel run's `ready-to-merge` file, whose PR merges with no further review (`leftovers.md`).
- **Findings about the change itself** (a hole in the session's own fix, a gap in its own new test, a finding a plan
  sub-skill parked or deferred) are fixed in the branch, given a known-gap comment, or dropped. They never become an
  issue or a pending finding. The filing bar does not apply to them, nor to design findings (`feature-flow.md` step
  4): both are judged on whether the change is sound, so a hole in the session's own fix may be fixed even when only
  an unlikely input opens it. This item is the one place that exemption lives. Anything else a review turns up, such
  as a bug in nearby code the change did not touch, is an ordinary finding: judge it by the filing bar and the
  out-of-scope rules. Every other place in this skill that says "fixed, known-gap-commented or dropped" means this
  item.
- **Subagents:** every subagent prompt that implements or reviews tells the subagent to return every finding to the
  controller and never to open an issue, comment on a PR, leave a `TODO` or decide for itself what is worth
  reporting. The filing bar is the controller's to apply, after the report; a reviewer told what not to flag reviews
  with a blind spot. The prompt templates the sub-skills ship carry no such clause, so it is appended to each
  (`sub-skill-overrides.md`). A reviewer inspects the branch with `git diff` and `git show`, never by adding a
  worktree of its own, which the next session's step 3 would report as the user's work.

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
  shared helper the PR already changes: its own `test:` commit, then its own `fix:` commit.
- **Otherwise** append it to `pending-findings.md` in the main checkout's `.claude/backlog/` (from inside a worktree,
  as "Leaving to write" says) the moment it is found (for plan-driven work, at the ledger sweep in "Review"), and
  carry on.

The session never opens an issue, PR comment or `TODO` for an out-of-scope finding. In this skill, "report them" in
the global "Fixing Findings" rule means `pending-findings.md` and nothing else. The known-gap comment, and the global
rule's `# TODO:` for a low-stakes ambiguity found mid-run, are separate and still allowed.

## Merging

Approval ("Terms") covers every push and merge in the chunk, so none waits for the user, except a merge under
shakedown ("Shakedown"). `merging.md` gives the
procedure, its traps and its exceptions: watching CI, a check that fails, a coverage-ratchet failure, an
infrastructure failure worth one rerun, a branch that fell behind `main` or conflicts with it, a merge the ruleset
refuses, and a pull or delete that is refused.

A blocked-banner halt in `merging.md` stops the chunk at that issue. Leave the worktree and record the halt in
`current-session.md` ("Halting" in `resume.md`), then tell the user what stopped it and what the session would do
next; after the
user's reply, go on with the issues that do not depend on the halted one, or stop, as the user says. Steps 8 and 9
then run from the main checkout, except that a halt caused by a failed `gh` call skips step 8: the walkthrough needs
`gh` to open issues, and the findings wait in `pending-findings.md` for the next session. The summary says so. A
session that ends before the chunk is done leaves what "Ending early" in `resume.md` describes.

## Shakedown

Until the user has watched one run go through end to end, an unattended merge is a step taken on trust, so both
backlog skills start in shakedown: `shakedown: on` is the default while `.claude/backlog/config` does not say `off`
(step 6 reads the file). While it is on, a chunk is one issue: form and order the chunk as usual, then take its
first issue as the chunk and name the rest as the runner-up; with named issues, the first named bug, or the first
named issue when none is a bug. Its merge waits for the user's OK: `merging.md` step 2 asks with the question banner
once CI is green, recorded as "Halting" in `resume.md` says. The summary offers to lift it; on the user's yes, write
`shakedown: off` to the config (create the file, or replace the key's line). The user may also turn it off or on with
the chunk OK. Turned off with the OK: write `shakedown: off` then, and send a full-size proposal (its halt questions
included) as a follow-up before starting; like the permission follow-up (`permissions.md`), it is not a second
round. Turned on: the chunk is its first issue.

## Budgets

An unattended run that goes badly should stop costing time on its own, so the config bounds it: `max-hours:` (default 8)
and `max-parked:` (default 2). The time runs from the `started:` line the session writes to `current-session.md` as
the unattended part begins, `started: <date --utc +%FT%TZ>`, replacing one a carried-over file holds: step 6 writes
it with a bug chunk's OK, `feature-flow.md` step 5 with the answered question round, and `findings-walkthrough.md`
step 6 when a "fix it now" makes a worktree. Once `max-hours` has passed, no further issue starts: the loop's step 1
stops there, the issues not started stay open for the next session, and the session goes to step 8 after the current
issue merges or halts. The proposal states `max-hours` and shakedown; `max-parked` is `/work-backlog-parallel`'s
alone, whose halts park an issue and go on, and here every halt already waits for the user.

## End-of-run report

Step 9 writes its summary to `.claude/backlog/reports/<session-date>.md`, named by the `session-date:` line's date
(`-2`, `-3` and so on when that date already has one), before sending it, so it survives a closed terminal and two
runs can be compared. The message sent is the same text. `plan` writes no report. Sections, each `none` when empty:

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
  shakedown is on, the offer to lift it ("Shakedown").
- `Session notes`: the branches and worktrees step 3 deleted; a CI rerun and why; a copied-rule conflict settled by
  satisfying the rule ("Copied rules"); a sub-skill pause whose quoted phrase the installed superpowers version no
  longer has (`sub-skill-overrides.md`); a piece skipped because the user closed its issue; an issue the brainstorm
  added to or dropped from the chunk; and every item in `.claude/backlog/parallel/summary.md`, which `leftovers.md`
  writes when step 3 finishes a parallel run's "Ending" (delete that file once the summary is sent).

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
| "The push timed out, so it failed" | The gate takes minutes; run it in the background and read the result. |
| "`timeout 30m` killed it; I'll run it again" | Exit 124 means it hung. Blocked banner; "Long commands". |
| "`gh pr checks` errored right after the PR opened" | "No checks reported" means none has started; wait, run again. |
| "The leftover ledger has open tasks; sweep and merge" | Open tasks mean the plan resumes first, then the sweep. |
| "The user said the commands are allowed, so no prompt" | Only an allow rule or bypass mode does; `permissions.md`. |
