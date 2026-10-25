# Leftovers

Runs from `SKILL.md` step 3 when step 2 found a `current-session.md` or the main checkout holds anything beyond
`main`. Five inputs feed it: the `current-session.md` step 2 read, which names the dead session's issues, spec, plans,
halt answers and how far each issue got, and four listings, all run after `git worktree prune` (a worktree directory
deleted by hand stays registered until then):

- `git worktree list`
- `gh pr list --author @me --state open --json number,body,headRefName`
- `git branch --merged main --format='%(refname:short) %(upstream:track)'`
- `git branch --no-merged main --format='%(refname:short)'`

Sort what they show into three kinds. Anything not claimed by the first two is the user's.

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
