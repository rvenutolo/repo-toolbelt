# Leftovers

Runs from `SKILL.md` step 3 when step 2 found a `current-session.md`, `.claude/backlog/parallel/` holds a file, or
the main checkout holds anything beyond `main`. Five inputs feed it: the `current-session.md` step 2 read, which
names the dead session's issues, spec, plans, halt answers and how far each issue got, and four listings, all run
after `git worktree prune` (a worktree directory deleted by hand stays registered until then):

- `git worktree list`
- `gh pr list --author @me --state open --json number,body,headRefName`
- `git branch --merged main --format='%(refname:short) %(upstream:track)'`
- `git branch --no-merged main --format='%(refname:short)'`

Sort what they show into three kinds. Anything not claimed by the first two is the user's.

## A parallel run's files

`/work-backlog-parallel` writes per-issue files under `.claude/backlog/parallel/` (its `SKILL.md` "State"), and
either skill may be the one that finds them. If `current-session.md` names a `workflow-script` and its date is
today, a parallel session may still be running in another terminal: ask with the question banner before touching
anything it names. Otherwise fold the files first, before sorting anything:

- Each `<N>-findings.md` holds findings no one has judged: judge each by "The filing bar" in `SKILL.md`, append the
  ones that pass to `pending-findings.md`, drop the rest, and delete the file by its literal path.
- Each `<N>.md` says how far issue N got. `merged`: delete the file. `parked` or anything earlier: the issue is
  unfinished work of this skill's (below), with the file's `branch:`, `pr:` and `worktrees:` telling where it is.
  A `park-note:` holds the question the run could not ask: ask it with the question banner before resuming that
  issue, recommending the note's answer, then delete the file once the issue merges or the user drops it.
- An `issue <N>` line in `current-session.md` with no `<N>.md`, no PR and no branch was approved but never built: it
  is unfinished work, resumed from the start of its flow under the approval the file records.

Worktrees that run made are this skill's too: `.claude/worktrees/merge-<N>` (on the issue branch) and
`.claude/worktrees/wf_<runId>-<k>`, whose branch is an issue branch, or its own `worktree-wf_<runId>-<k>`, or none
(detached). A `<N>.md` lists every one an issue used under `worktrees:`. One that is locked although no run is going
is unlocked with `git worktree unlock <path>` before anything else touches it.

- One on an issue branch is that issue's worktree, sorted below like any other. Before resuming in it, bring it up
  to date with `git pull --ff-only origin <branch>` (later rounds pushed from other worktrees), and when its PR
  merges, `merging.md` step 4 removes it by its own path, not the `<N-description>` one.
- A detached or `worktree-wf_*` one, or a `merge-<N>` whose rebase is in progress (`git -C <path> rebase --abort`
  first), with no commits that are not on its issue's `origin/<branch>` or on `origin/main`, holds nothing: remove
  it and delete its `worktree-wf_*` branch, naming each in the summary. A `worktree-wf_*` branch is this skill's
  even when it looks like a merged branch the user made.
- One that holds such commits, or that no `<N>.md` names: ask with the question banner whether to keep it for its
  issue or remove it.

## This skill's unfinished work

- An open PR whose body ends with the marker. When the body names an issue as `Closes #N` or `Part of #N`, that
  issue must still be open (`gh issue view <number> --json state --jq .state` prints `OPEN`). A marker PR that
  names no issue is a "fix it now" from `findings-walkthrough.md` and is this skill's as well.
- A worktree under `.claude/worktrees/` whose branch has no PR, when the issue it belongs to is still open. The
  issue is the N after `<type>/` in the branch name (`<type>/<N>-<description>`, "Branches" in `SKILL.md`). A
  `fix/finding-<slug>` branch has no issue; it is this skill's while its entry is still in `pending-findings.md`.
- A worktree under `.claude/worktrees/` with no commits beyond `origin/main` is this skill's too, but holds nothing:
  remove it (`git worktree remove`, then `git branch --delete`) and name it in the summary.
- A worktree under `.claude/worktrees/` whose issue has since been closed is this skill's but has nothing left to
  finish: ask with the question banner whether to remove it or keep it, and do as the user says.
- An approved spec recorded in `current-session.md` (`spec approved: <path>`) with issues that have no worktree, no
  PR and no merged PR, even when git and GitHub show nothing: the brainstorm and question round are done, so the
  unbuilt issues resume at `feature-flow.md` step 6 under that approval.

Unfinished work is finished (the rest of its flow, then review, CI, merge) before anything new is chosen, under the
approval it already has: the halt answers and the allowed commands are in `current-session.md`, and the spec and
plans it names are under `docs/superpowers/` in the main checkout. This comes before the bugs-first rule. If
`current-session.md` is missing (a death before it was written) the approval is gone with it: an unfinished PR or
worktree is still this skill's, but its remaining halt questions and the permission request are asked again before
work on it resumes.

Before resuming a leftover PR, read `gh pr view <number> --json isDraft,reviewDecision,comments,reviews`: a draft, a
non-empty review decision, or any comment or review means the user has touched it since. Ask with the question banner
whether to resume it or leave it, and do not touch it before the answer.

To resume:

- A leftover worktree is entered with `EnterWorktree` by `path`, as "Worktrees" in `SKILL.md` says, and the
  instruction files are copied again.
- A leftover PR whose worktree is gone gets one back with
  `git worktree add .claude/worktrees/<N-description> <type/N-description>` (no `-b`: it checks out the local
  branch, or, when only `origin/<type/N-description>` exists, creates it tracking that), then
  `git pull --ff-only origin <type/N-description>` inside it so the branch matches the remote.
- If the worktree holds a ledger (`.superpowers/sdd/*/progress.md`) with unfinished tasks, the plan is half built:
  resume the plan sub-skill from the ledger first, with the plan from `docs/superpowers/plans/` (its path is in
  `current-session.md`). Red CI on such a PR is the unbuilt part, not a check to fix. The ledger sweep ("Review" in
  `SKILL.md`) then runs at its normal place before the merge, and sweeps what the dead session parked along with
  the rest; when every task is complete, the sweep is the next step.
- A leftover PR gets a superpowers:requesting-code-review pass before it merges, whichever way it was built.

## Done

A branch other than `main` that the `--merged` listing shows with upstream `[gone]`. If a worktree under
`.claude/worktrees/` still holds it (a session that died between the merge and the cleanup),
`git worktree remove .claude/worktrees/<N-description>` first, since the delete fails while the branch is checked
out; then `git branch --delete <name>`, naming each, and list the deleted names in the session summary.

## The user's own work

Left alone and named in the chunk proposal so the user knows it was seen: every other worktree, every other open PR
(one without the marker is the user's even when it says `Closes #N`), every branch in the `--no-merged` listing that
is not a leftover PR's or leftover worktree's branch, and a merged branch whose upstream still exists or that never
had one (it may be a branch the user just made).
