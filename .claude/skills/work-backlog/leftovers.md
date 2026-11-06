# Leftovers

Runs from `SKILL.md` step 3 when step 2 found a `current-session.md`, `.claude/backlog/parallel/` holds a file, or
the main checkout holds anything beyond `main`. Its inputs: the `current-session.md` step 2 read, which names the
dead session's issues, spec, plans, halt answers and how far each issue got; the files under
`.claude/backlog/parallel/`; and four listings, all run after `git worktree prune` (a worktree directory deleted by
hand stays registered until then):

- `git worktree list`
- `gh pr list --author @me --state open --json number,body,headRefName`
- `git branch --merged main --format='%(refname:short) %(upstream:track)'`
- `git branch --no-merged main --format='%(refname:short)'`

Sort what they show by the sections below. Anything no section above "The user's own work" claims is the user's.
`.claude/backlog/config` and `.claude/backlog/reports/` are the user's settings and kept reports, not leftovers:
nothing here reads, moves or deletes them.

## A parallel run's files

`/work-backlog-parallel` writes per-issue files under `.claude/backlog/parallel/` (its `SKILL.md` "State"), and
either skill may be the one that finds them.

**A run may still be going.** A long parallel run crosses midnight, so the date proves nothing. Read
`current-session.md`'s `workflow-*` lines first:

- `workflow-done`: the run finished and its result is recorded; only its "Ending" is left. Run
  `/work-backlog-parallel`'s "Ending" step 1 instead of asking (redo it if its output is missing), then write the
  items its "Ending" step 3 would name into `.claude/backlog/parallel/summary.md` before its step 4 deletes the
  files they come from, then run its step 4 and carry on with this session. Its walkthrough is this session's step
  4, which comes after every leftover is finished, and this session's step 9 reads `summary.md` into its own summary
  and then deletes it, so the user gets one summary.
- `workflow-result` with no `workflow-done`: the run finished, and its result waits in
  `.claude/backlog/parallel/result.json`. Record from it as the parallel skill's "The run" step 5 does, delete the
  file, and treat the run as `workflow-done`.
- `workflow-run` (written with `workflow-script` and `workflow-args` at launch) and neither of those: ask with the
  question banner, before touching anything the run names, whether to resume it, take its issues over here, or leave
  them because the run is still going. Recommend
  "leave" when a `wf_*` worktree holds a rebase in progress or is locked by a live process ("Locked worktrees"
  below). Taking over strips the `workflow-*` and `cap:` lines from `current-session.md` first, so the parallel skill
  never resumes a run whose issues moved on. Whether the run can still be resumed, and how, is decided as
  `/work-backlog-parallel`'s "The run" step 4 says; recommend the resume when that step finds it possible. Leaving
  keeps `current-session.md` as the run's file: this session never adds its own chunk to a file that still holds
  `workflow-*` lines (`SKILL.md` step 6).
- `cap:` and `issue` lines but no `workflow-*` line: a parallel batch that was approved and never launched. Ask with
  the question banner whether to launch it with `/work-backlog-parallel` (recommended: the approval was given for a
  parallel run) or take its issues over here; taking over strips the `cap:` line first, so a later parallel session
  does not launch issues that moved on.

Then fold the files, before sorting anything:

- Each `<N>-findings.md` holds findings no one has judged: judge each by "The filing bar" in `SKILL.md`, append the
  ones that pass to `pending-findings.md`, drop the rest, and delete the file by its literal path.
- Each `<N>.md` says how far issue N got, with its `branch:`, `pr:`, `plan:` and `worktrees:` telling where it is.
  `plan:` is repo-relative: the plan is at that path in the main checkout.
  - `merged`: correct the spec for each divergence `ruling:` in it (the code that merged wins) and apply each
    `ruling:` that names a `.claude/CLAUDE.md` edit to the main checkout's copy, sort the worktrees it lists
    (below), then delete the file. The parallel skill's "Ending" step 1 does only the findings fold above, this spec
    correction and the `.claude/CLAUDE.md` edits; the worktree sort and the delete wait for its step 4, because
    its summary reads the file's `worktrees:` line.
  - `skipped`: confirm the issue or PR is closed (`gh issue view`, `gh pr view`), then sort its `worktrees:` before
    deleting the file: they are not unfinished work, whatever the issue's state, since the user closed what they
    were for. Ask with the question banner whether to remove each worktree and its branch, recommending removal.
    A budget skip is the exception: its note says the issue was never started (a budget, the deadline, or a
    prerequisite waiting for its merge OK), its issue is still open and no worktree or PR was made. There is nothing
    to confirm or sort: delete the file, and the issue stays in the backlog for the next chunk.
  - `ready-to-merge`: an open PR the run built and reviewed, waiting for the shakedown's merge OK
    (`/work-backlog-parallel` "Shakedown and budgets"). Ask and merge as `/work-backlog-parallel` "Ending" step 1
    says, in the worktree it names, with no requesting-code-review pass. Then handle it as `merged`. On no, keep the
    file and leave the PR open for the user.
  - `parked` or anything earlier: the issue is unfinished work of this skill's (below). A `park-note:` holds the
    question the run could not ask: ask it with the question banner before resuming that issue, recommending the
    note's answer. A note the script wrote (a dependency, a batch halt, a dead agent) carries no question; ask
    whether to resume the issue now that its cause is settled. Once the issue merges, apply its `.claude/CLAUDE.md`
    rulings as for `merged`, then delete the file; delete it too if the user drops the issue.
- An `issue <N>` line in `current-session.md` with no `<N>.md`, no PR and no branch was approved but never built: it
  is unfinished work, resumed from the start of its flow under the approval the file records, after every issue its
  `after` list names. A `piece <name>` line after it limits the resume to that piece of an umbrella issue, whose PR
  says `Part of #N` unless the piece is the spec's last.
- `run-args.json` and `result.json`: delete `result.json` once it is recorded (above), and `run-args.json` once
  `current-session.md` holds no `workflow-*` line.
- `summary.md`: a dead session's undelivered summary; keep it, and step 9 gives its items and deletes it.

Worktrees that run made are this skill's too: `.claude/worktrees/merge-<N>` (on the issue branch) and
`.claude/worktrees/<runId>-<k>` (the run id itself begins `wf_`), whose branch is an issue branch, or its own
`worktree-<runId>-<k>`, or none (detached). A `<N>.md` lists every one an issue used under `worktrees:`. Before
sorting one: a locked one whose
lock PID is dead ("Locked worktrees" below) is unlocked with `git worktree unlock <path>` (once the user has said no
run is going), and one with a rebase in progress gets `git -C <path> rebase --abort`, which puts it back on its
branch. Nothing else removes them for this skill: Claude Code's own retention sweep removes only clean `wf_*`
worktrees whose commits are all pushed, and only after the `cleanupPeriodDays` setting's cutoff.

- One on an issue branch is that issue's worktree, sorted below like any other. Before resuming in it, when
  `origin/<branch>` exists, bring it up to date with `git pull --ff-only origin <branch>` (later rounds pushed from
  other worktrees); a refused fast-forward means work in two places: ask with the question banner. When the
  issue's `park-note:` says the worktree holds an unpushed, rebased branch, that is expected, not lost work: the
  next merge rebases again, so recommend dropping the rebase by moving the branch back to `origin/<branch>`
  (`git -C <path> switch --detach origin/<branch>`, then, once the user confirms,
  `git branch --force <branch> origin/<branch>` and `git -C <path> switch <branch>`), and offer pushing it instead
  only when the note says a resolution review passed it. When its PR merges, `merging.md` step 4 removes it by its
  own path, not the `<N-description>` one.
- A detached or `worktree-wf_*` one with no commits that are not on its issue's `origin/<branch>` or on
  `origin/main` holds nothing: remove it and delete its `worktree-<runId>-<k>` branch, naming each in the summary.
- One that holds such commits, or that no `<N>.md` names: ask with the question banner whether to keep it for its
  issue or remove it.
- A `worktree-wf_*` branch is this skill's even with no worktree left and even when it looks like a merged branch
  the user made: delete it once `origin/main` contains it.

### Locked worktrees

Claude Code locks a worktree it made for an agent and unlocks it when that agent finishes, so a lock means a running
agent or a session that died. `git worktree list --porcelain` shows the lock reason with its PID, and
`kill -0 <pid>` tells whether that process is alive: alive means a run is going, dead means the lock is stale.

## This skill's unfinished work

- An open PR whose body ends with the marker. When the body names an issue in a line that is exactly `Closes #N` or
  `Part of #N` (a whole-line match, "PRs" in `SKILL.md`), that issue must still be open
  (`gh issue view <number> --json state --jq .state` prints `OPEN`). A marker PR that names no issue is a "fix it
  now" from `findings-walkthrough.md` and is this skill's as well.
- A worktree under `.claude/worktrees/` whose branch has no PR, when the issue it belongs to is still open. The
  issue is the N after `<type>/` in the branch name (`<type>/<N>-<description>`, "Branches" in `SKILL.md`). A
  `fix/finding-<slug>` branch has no issue; it is this skill's while its entry is still in `pending-findings.md`.
  The PR listing above shows open PRs only, so check `gh pr list --state all --head <branch> --json state` first: a
  `CLOSED` PR means the user rejected that work, so it is not unfinished. Ask with the question banner whether to
  remove the worktree and its branch, recommending removal, and never open a new PR for it unasked. A `MERGED` one
  belongs to "Done".
- A worktree under `.claude/worktrees/` with no commits beyond `origin/main` is this skill's too, but holds nothing:
  remove it (`git worktree remove`, then `git branch --delete`) and name it in the summary.
- A worktree under `.claude/worktrees/` whose issue has since been closed is this skill's but has nothing left to
  finish: ask with the question banner whether to remove it or keep it, and do as the user says.
- An approved spec recorded in `current-session.md` (`spec approved: <path>`) with issues that have no worktree, no
  PR and no merged PR, even when git and GitHub show nothing: the brainstorm and question round are done, so the
  unbuilt issues resume at `feature-flow.md` step 6 under that approval.

Unfinished work is finished (the rest of its flow, then review, CI, merge) before anything new is chosen, under the
approval it already has: the halt answers and the allowed commands are in `current-session.md`, and the spec and
plans it names are under `docs/superpowers/` in the main checkout. This comes before the bugs-first rule. The one
exception is a parked issue the user chose to leave for later (the answer to its `park-note:` question): it stays
recorded and parked, and a new chunk may be chosen around it. If `current-session.md` is missing (a death before it
was written) the approval is gone with it: an unfinished PR or worktree is still this skill's, but its remaining halt
questions and the permission request are asked again before work on it resumes.

Before resuming a leftover PR, run `merging.md` step 0's checks on it from the main checkout, before any work on
it. Their outcomes are step 0's, with two differences, because no worktree has been entered yet and step 3 starts by
leaving one: a `MERGED` PR is recorded in `current-session.md` and its branch and worktree are handled under "Done"
below; a `CLOSED` PR is handled as the second bullet of "This skill's unfinished work" says for a closed PR (ask
whether to remove its worktree and branch, recommending removal).

To resume:

- A leftover worktree is entered with `EnterWorktree` by `path`, as "Worktrees" in `SKILL.md` says, and the
  instruction files are copied again.
- A leftover PR whose worktree is gone gets one back with
  `git worktree add .claude/worktrees/<N-description> <type/N-description>` (no `-b`: it checks out the local
  branch, or, when only `origin/<type/N-description>` exists, creates it tracking that), then
  `git pull --ff-only origin <type/N-description>` inside it so the branch matches the remote.
- If the worktree holds a ledger (`.superpowers/sdd/*/progress.md`) with unfinished tasks, the plan is half built:
  resume the plan sub-skill from the ledger first, with the plan from `docs/superpowers/plans/` (its path is in
  `current-session.md`, or in the issue's `<N>.md` `plan:` line for a parallel run). Red CI on such a PR is the
  unbuilt part, not a check to fix. The ledger sweep ("Review" in `SKILL.md`) then runs at its normal place before
  the merge, and sweeps what the dead session parked along with the rest; when every task is complete, the sweep is
  the next step.
- A leftover PR gets a superpowers:requesting-code-review pass before it merges, whichever way it was built, unless
  its `<N>.md` says `ready-to-merge` (above).

## Done

A branch other than `main` that the `--merged` listing shows with upstream `[gone]`, or a leftover PR's branch once
`gh pr view` prints `MERGED`. If a worktree under `.claude/worktrees/` still holds it (a session that died between
the merge and the cleanup), remove that worktree first (the path `git worktree list` shows holding the branch:
`<N-description>`, or a parallel run's `<runId>-<k>` or `merge-<N>`), since the delete fails while the branch is
checked out; then `git branch --delete <name>`, naming each, and list the deleted names in the session summary. A
delete refused because local `main` lacks the merge waits for a later session, as `merging.md` step 4 says.

## The user's own work

Left alone and named in the chunk proposal so the user knows it was seen: every other worktree, every other open PR
(one without the marker is the user's even when it says `Closes #N`), every branch in the `--no-merged` listing that
is not a leftover PR's or leftover worktree's branch, and a merged branch whose upstream still exists or that never
had one (it may be a branch the user just made).
