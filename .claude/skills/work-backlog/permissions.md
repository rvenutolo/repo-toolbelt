# Permissions

The unattended part runs commands outside the user's permission allowlist, and a prompt mid-run stalls it until the
user comes back (the global "Long Multi-Step Tasks" rule). Saying "allowed" in chat does not stop the harness from
prompting; only an allow rule or a session run with permissions bypassed does. So the permission request ("Terms" in
`SKILL.md`) names the allow rules the run needs and the allowlists lack, and asks the user to add them to the main
checkout's `.claude/settings.local.json` (a local file, ignored like the rest of "State" in `SKILL.md`) or to restart
with permissions bypassed. A session already running with permissions bypassed omits the request; the signing check
below still runs.

## When to ask

- With the chunk proposal (bugs) or in the question round (features, `feature-flow.md` step 5). A "fix it now" from
  `findings-walkthrough.md` asks first if this session has not asked yet.
- Again right before the unattended part starts (the chunk OK for bugs, the answered question round for features, the
  launch for `/work-backlog-parallel`): re-read the allowlists, since the user may have changed them in between, and
  run the signing check.

A deferral ("do whatever you think") answers no permission question: the request still needs its own answer, and a
follow-up message carrying only the request is allowed and is not a second question round.

## Rule strings

Check the allowlists in `$CLAUDE_CONFIG_DIR/settings.json` and the main checkout's `.claude/settings*.json` rather
than assuming, and list only the rules they lack. Name rule strings, not commands: a rule matches a command by its
literal prefix, so `Bash(git rebase *)` does not cover `git -C <path> rebase`, and a leading variable assignment
(`GIT_EDITOR=true git …`) matches no `git` rule. Claude Code strips a leading `timeout`, `nice` or `nohup` before
matching, so a rule names the wrapped command, never the wrapper. A redirect is checked as a file write: a target
outside the working directories always prompts, whatever the `Bash` rules say.

The candidates, with what runs them:

- `Bash(cp *)`, `Bash(mkdir --parents *)`: the instruction-file copy on every worktree entry ("Worktrees").
- `Bash(./.ci/in-devshell *)`: the baseline tests, the gate, lint checks, a bug's reproduction and the baseline
  regeneration in `merging.md`.
- `Bash(git worktree add *)`, `Bash(git worktree remove *)`, `Bash(git worktree prune)`,
  `Bash(git worktree unlock *)`, `Bash(git branch --delete *)`.
- `Bash(git pull --ff-only*)`: in the main checkout and, resuming a parallel run's worktree, `origin <branch>` there.
- `Bash(git rebase *)` (with `--abort`) and `Bash(git push *)` (`--set-upstream`, and `--force-with-lease` after a
  rebase in `merging.md` step 2).
- `Bash(git -C *)`: `leftovers.md` runs `git -C <path> rebase --abort` and `switch`.
- `Bash(sleep *)`: the background waits in "Long commands".
- The plan sub-skills' helper scripts, run by absolute path from the installed superpowers plugin
  (`<plugin dir>` is `$CLAUDE_CONFIG_DIR/plugins/cache/claude-plugins-official/superpowers/<version>`):
  `Bash(<plugin dir>/skills/executing-plans/scripts/task-start *)`,
  `Bash(<plugin dir>/skills/executing-plans/scripts/task-done *)` and
  `Bash(<plugin dir>/skills/subagent-driven-development/scripts/sdd-workspace *)`, with the version written out.
- `Bash(rm -rf .superpowers/sdd/*)`: the plan workspace delete after the ledger sweep.
- `Edit` rules for every file the run writes, unless the session runs with accept-edits or bypassed permissions:
  `Edit(/<main>/.claude/worktrees/**)` for the code, and `Edit(/<main>/.claude/backlog/**)`,
  `Edit(/<main>/docs/superpowers/**)` and `Edit(/<main>/.claude/rules/**)` for the state files, specs, plans and rule
  copies. `<main>` is the main checkout's absolute path, so each rule starts with `//`, which Claude Code reads as an
  absolute path. An `Edit` rule covers the Write tool too.
- Any tool the spec's work, or a bug's fix, needs beyond the devShell's.

## Workflow agents add

`/work-backlog-parallel`'s agents run unattended in their own worktrees, and one that meets a prompt holds its slot
and stalls every issue that depends on it, so its request adds the commands they run (its `planning.md` "The
permission request"):

- `Bash(cd *)`: the merge agent runs `cd <worktree> && ./.ci/in-devshell …`.
- `Bash(git add *)`, `Bash(git commit *)`, `Bash(git fetch *)`, `Bash(git switch *)`, `Bash(git branch -m *)`,
  `Bash(git merge --ff-only *)`, `Bash(git checkout --ours *)`, `Bash(git merge-base *)`, `Bash(git rev-parse *)`,
  `Bash(git log *)`, `Bash(git diff *)`, `Bash(git show *)`, `Bash(git status *)`, `Bash(git branch --list *)`,
  `Bash(git worktree list *)`.
- `Bash(gh *)`, or each of `gh pr create|edit|checks|merge|list|view`, `gh issue view`, `gh run list|rerun`,
  `gh api repos/*` (the merge queue's conflict check reads which PR a `main` commit came from) and `gh auth status`.
- `Bash(<main>/.claude/skills/work-backlog-parallel/wait-for *)`: the waits in its `common.md` "Long commands". The
  status files those waits poll live in the agent's own worktree under `.claude/tmp/`, which the `.claude/worktrees/`
  `Edit` rule covers.
- `Edit(/<main>/.claude/backlog/parallel/**)` is covered by the `.claude/backlog/` rule above; the review stage
  writes there.
- `Bash(kill -0 *)`, `Bash(sysctl net.ipv4.ip_forward)`.

## Signing

Every commit and rebase signs with the key `git config user.signingkey` names, so a key that is missing or expires
mid-run fails every later commit. Check it the way git uses it, with a test signature:

```bash
printf x | ssh-keygen -Y sign -n git -f "$(git config user.signingkey)" >/dev/null
```

A non-zero exit means the agent does not hold the key: say so in the request. `ssh-add -L` cannot show a key's
lifetime, so the request also asks the user to confirm the key was loaded with no lifetime that could expire during
the run. Commit signing that fails mid-run is a blocked-banner halt.
