# Leftovers

Runs from `SKILL.md` step 3 when the main checkout holds anything beyond `main`. Four listings feed it, all run after
`git worktree prune` (a worktree directory deleted by hand stays registered until then):

- `git worktree list`
- `gh pr list --author @me --state open --json number,body,headRefName`
- `git branch --merged main --format='%(refname:short) %(upstream:track)'`
- `git branch --no-merged main --format='%(refname:short)'`

Sort what they show into three kinds. Anything not claimed by the first two is the user's.

## This skill's unfinished work

- An open PR whose body ends with the marker and names an open issue as `Closes #N` or `Part of #N`
  (`gh issue view <number> --json state --jq .state` prints `OPEN`).
- A worktree under `.claude/worktrees/` whose branch has no PR, when the issue it belongs to is still open. The
  issue is the N after `<type>/` in the branch name (`<type>/<N>-<description>`, "Branches" in `SKILL.md`).
- A worktree under `.claude/worktrees/` with no commits beyond `origin/main` is this skill's too, but holds nothing:
  remove it (`git worktree remove`, then `git branch --delete`) and name it in the summary.

Unfinished work is finished (review, CI, merge) before anything new is chosen, under the approval it already has;
the spec and plan that record that approval are under `docs/superpowers/` in the main checkout. This comes before
the bugs-first rule.

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
- If the worktree holds a ledger (`.superpowers/sdd/*/progress.md`), run the ledger sweep from "Review" in
  `SKILL.md` before anything else: the session that died may have parked findings there and nowhere else.
- A leftover PR gets a superpowers:requesting-code-review pass before it merges, whichever way it was built.

## Done

A branch other than `main` that the `--merged` listing shows with upstream `[gone]`. If a worktree under
`.claude/worktrees/` still holds it (a session that died between the merge and the cleanup),
`git worktree remove .claude/worktrees/<N-description>` first, since the delete fails while the branch is checked
out; then `git branch --delete <name>`, naming each, and list the deleted names in the session summary.

## The user's own work

Left alone and named in the chunk proposal so the user knows it was seen: every other worktree, every other open PR
(one without the marker is the user's even when it says `Closes #N`), every branch in the `--no-merged` listing that
is not a leftover PR's branch, a merged branch whose upstream still exists or that never had one (it may be a branch
the user just made), and a worktree whose issue has since been closed.
