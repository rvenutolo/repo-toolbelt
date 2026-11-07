# Permissions

The unattended part runs commands outside the user's permission allowlist, and a prompt mid-run stalls it until the
user comes back (the global "Long Multi-Step Tasks" rule). Saying "allowed" in chat does not stop the harness from
prompting; only an allow rule or a session run with permissions bypassed does. So the permission request ("Terms" in
`SKILL.md`) names the allow rules the run needs and the allowlists lack, and asks the user to add them to the main
checkout's `.claude/settings.local.json` (a local file, ignored like the rest of "State" in `SKILL.md`) or to restart
with permissions bypassed. A session already running with permissions bypassed omits the request; the signing check
below still runs. A session whose system prompt carries the paragraph beginning "While bypass permissions mode is
active" runs with permissions bypassed; one without it treats itself as not bypassed and makes the request, which
costs one answer, where the wrong guess costs a stalled run. `defaultMode` in the settings is only a hint, since a
session can be started in another mode.

## When to ask

- In step 1 of "The session", before the preflight, when a "Run while the user is there" rule is missing
  (`write the rules` / `continue`), so the first run's listings do not each prompt on their own.
- With the chunk proposal (bugs) or in the question round (features, `feature-flow.md` step 5). A "fix it now" from
  `findings-walkthrough.md` asks first if this session has not asked yet.
- Again right before the unattended part starts (the chunk OK for bugs, the answered question round for features, the
  launch for `/work-backlog-parallel`): re-read the allowlists, since the user may have changed them in between, and
  run the signing check. Also confirm the main checkout is on `main` (`git symbolic-ref --short HEAD`) and that, after
  `git fetch origin main`, `git merge-base --is-ancestor HEAD origin/main` succeeds: a sitting is long, and a commit the
  user made on local `main` meanwhile would refuse the pull that follows every merge, so ask them to move it off first.

The request tells the user how to add the rules and names the three replies the proposal's `Reply` line offers ("The
proposal" in `choosing.md`): `rules added` means the user added them with `/permissions` or by editing
`.claude/settings.local.json`, and the session re-reads the files before it starts; `write the rules` has the session
append every missing rule string to the main checkout's `.claude/settings.local.json` under `permissions.allow`
(creating the file if absent, keys sorted alphabetically at every level, as the user's settings rule asks), re-run
`permission-diff` and go on, since Claude Code re-reads the settings files while a session runs, so the rules apply at
once and the user types nothing; `skip the permissions` is a refusal, and an answer: go on, and say the run will stop
at its first prompt until the user returns. A deferral ("do whatever you think") answers no permission question: the
request still needs its own answer, and a follow-up message carrying only the request is allowed and is not a second
question round. Never drop a command to avoid its rule.

## Rule strings

Check the allowlists rather than assuming, and list only the rules they lack:
`<main>/.claude/skills/work-backlog/permission-diff <rule>...`, with every candidate below written out (or one per
line on stdin with `-`), prints each rule that no allowlist (`$CLAUDE_CONFIG_DIR/settings.json` and the main
checkout's `.claude/settings.json` and `.claude/settings.local.json`; Claude Code reads no user-level local file)
holds verbatim, and prints `defaultMode` on stderr. Its `--superpowers-dir` prints the installed superpowers plugin
directory, the `<plugin dir>` below. It compares strings only, but a rule printed with a `covered by: <rule>` line
under it is already allowed by that broader rule (`Bash(git *)` covers `Bash(git rebase *)`) and is left out of the
request. Name rule strings, not commands: a rule matches a command by its literal prefix, so `Bash(git rebase *)`
does not cover `git -C <path> rebase`, and a leading variable assignment (`GIT_EDITOR=true git …`) matches no `git`
rule. Claude Code strips a leading `timeout`, `nice` or `nohup` before matching, so a rule names the wrapped command,
never the wrapper. A redirect is checked as a file write: a target outside the working directories prompts unless an
`Edit` allow rule covers it; a `Bash` rule never does.

The request lists the candidates of the flows this session will run. Each candidate below is tagged `all` (every
flow, a bug chunk and a "fix it now" included), `features` or `leftovers` (step 3 found unfinished work), with what
runs it; a bug chunk with no leftovers asks for no plan-script rule and no recursive delete.

**Run while the user is there.** These are first used in steps 1 to 6, before or with the request, so a missing rule
costs a prompt, not a stall; `git pull --ff-only` and `git worktree list` run unattended later too (`merging.md` step
3, `merge-queue.md` section 6), so they still belong in the request:

- `Bash(git switch main)`, `Bash(git pull --ff-only*)`, `Bash(git branch --merged *)`, `Bash(git branch --no-merged *)`,
  `Bash(git worktree prune*)` (the `*` lets `plan`'s `--dry-run` match) and `Bash(git worktree list *)` (all): step 1,
  step 3's listings and `leftovers.md`; the pull also runs in the main checkout after every merge and, resuming a
  parallel run's worktree, as `origin <branch>` there.
- `Bash(<main>/.claude/skills/work-backlog/backlog-preflight*)`,
  `Bash(<main>/.claude/skills/work-backlog/backlog-status*)`,
  `Bash(<main>/.claude/skills/work-backlog/backlog-index*)` and
  `Bash(<main>/.claude/skills/work-backlog/permission-diff*)` (all), with `<main>` written out so the rule matches
  from any directory: step 1, `leftovers.md` and the proposal run them; with them allowed, a later session's step 1
  and proposal, `plan` included, run without a prompt.
- `Bash(git config --get *)`, `Bash(ssh-keygen -Y sign *)` and `Bash(printf *)` (all): the signing check ("Signing"),
  run by `backlog-preflight` in step 1 and again by the session right before the unattended part.

**The unattended part.**

- `Bash(cp *)`, `Bash(mkdir --parents *)` (all): the instruction-file copy on every worktree entry ("Worktrees" in
  `SKILL.md`).
- `Bash(./.ci/in-devshell *)` (all): the gate, lint checks, a bug's reproduction and the baseline regeneration's
  `go run` in `merging.md`.
- `Bash(./run-tests *)` (all): the baseline tests and the tests of the baseline regeneration in `merging.md`;
  `run-tests` enters the devShell itself, so it runs without the `./.ci/in-devshell` prefix.
- `Bash(./.ci/test-image *)` (all): the one step that needs a container ("How the session runs" in `SKILL.md`);
  `.ci/test-image` enters the devShell itself, so it runs without the `./.ci/in-devshell` prefix.
- `Bash(git worktree add *)`, `Bash(git worktree remove *)`, `Bash(git branch --delete *)` (all): "Worktrees" in
  `SKILL.md`, `merging.md` step 4 and `leftovers.md`.
- `Bash(sysctl net.ipv4.ip_forward)` (all): the Docker check before `./.ci/test-image`, the one step that needs a
  container ("How the session runs" in `SKILL.md`).
- `Bash(git add *)`, `Bash(git commit *)` (all): the `test:` and `fix:` commits (`bug-flow.md` step 4), a plan's
  commits, a baseline regeneration and a merge-time fix (`merging.md`).
- `Bash(git fetch *)`, `Bash(git rev-parse *)`, `Bash(git log *)`, `Bash(git merge-base *)`, `Bash(git diff *)`,
  `Bash(git show *)`, `Bash(git status *)`, `Bash(git symbolic-ref *)` (all): the `BEHIND` rebase and its signature
  check and baseline test in `merging.md` step 2, the review base ("Review" in `SKILL.md`), the reviewer's inspection
  of the branch, step 1's preconditions and the `main` check in "When to ask".
- `Bash(git -c rerere.enabled=false rebase *)` (with `--abort`; `merging.md` step 2 says why every rebase carries the
  `-c`) and `Bash(git push *)` (all): `--set-upstream` for the first push, and `--force-with-lease` after a rebase in
  `merging.md` step 2.
- `Bash(gh *)` (all), or each of `gh pr create|edit|checks|merge|list|view`, `gh issue view|list|close`,
  `gh run list|rerun`, `gh api repos/*` and `gh auth status`: the PR, its CI watch and merge, the issue checks, the
  walkthrough's closes, step 1's preconditions, and the ruleset read for a refused merge (`merging.md` step 2).
- `Bash(git worktree unlock *)`, `Bash(git -C * rebase --abort)`, `Bash(git -C * switch *)`,
  `Bash(git branch --force *)`, `Bash(kill -0 *)` (leftovers): `leftovers.md` runs these
  on a leftover worktree. Never the bare `Bash(git -C *)`: it matches every git subcommand after `-C`, `reset --hard`
  and `branch -D` included.
- `Bash(sleep *)` (all): the background waits in "Long commands" in `SKILL.md`.
- `Bash(date --utc *)` (all): the time budget's `started:` line, its deadline and its check before each issue
  ("Budgets" in `SKILL.md`), and the parallel run's `deadline`.
- `Bash(jq *)` (all): reading an issue body from the index ("The index" in `choosing.md`) and a PR body before an
  edit.
- `Bash(rm <main>/.claude/backlog/current-session.md)` and `Bash(rm <main>/.claude/backlog/parallel/*)` (all), with
  `<main>` written out: step 9's delete of `current-session.md`, and the deletes of the parallel run's files
  (`leftovers.md` "A parallel run's files", `/work-backlog-parallel` "Ending"), which run after the work is done and
  would otherwise prompt with nobody there.
- `Bash(echo *)` and `Bash(ls *)` (all): a background command's exit status written to a file and the check for an
  existing status file (`/work-backlog-parallel` `common.md` "Long commands"), and the listing of a directory before
  a delete.
- The plan sub-skills' helper scripts (features), each run by its absolute path from the installed superpowers
  plugin, without a `bash` prefix (`sub-skill-overrides.md`), so one rule per script matches. `<plugin dir>` is what
  `permission-diff --superpowers-dir` prints, written out:
  `Bash(<plugin dir>/skills/executing-plans/scripts/task-start *)`,
  `Bash(<plugin dir>/skills/executing-plans/scripts/task-done *)`,
  `Bash(<plugin dir>/skills/subagent-driven-development/scripts/sdd-workspace *)`,
  `Bash(<plugin dir>/skills/subagent-driven-development/scripts/task-brief *)` and
  `Bash(<plugin dir>/skills/subagent-driven-development/scripts/review-package *)`. superpowers:executing-plans runs
  `review-package` and `sdd-workspace` from the subagent-driven-development directory too.
- `Bash(rm --recursive --force <main>/.claude/worktrees/*/.superpowers/sdd/*)` (features), with `<main>` written out:
  the plan workspace delete after the ledger sweep. `sdd-workspace` prints the workspace as an absolute path, and the
  delete names that path in long options (`sub-skill-overrides.md`), so a relative `rm -rf` rule never matches it.
- `Edit` rules (all) for every file the run writes, omitted when `permission-diff` prints `defaultMode: acceptEdits`
  or `bypassPermissions` (the script counts `bypassPermissions` only from `$CLAUDE_CONFIG_DIR/settings.json`, since a
  project or local file cannot set it; the system-prompt tell above is the other way to know): a wrong omission costs
  one prompt at the first write, the `current-session.md` write right after the OK, while the user is still there.
  `Edit(/<main>/.claude/worktrees/**)` for the code, and
  `Edit(/<main>/.claude/backlog/**)`, `Edit(/<main>/.claude/tmp/**)`, `Edit(/<main>/docs/superpowers/**)`,
  `Edit(/<main>/.claude/rules/**)` and `Edit(/<main>/.claude/CLAUDE.md)` for the state files, the issue listing,
  specs, plans, rule copies and `.claude/CLAUDE.md` corrections. `<main>` is the main checkout's absolute path, so
  each rule starts with `//`, which Claude Code reads as an absolute path. An `Edit` rule covers the Write tool too.
- Any tool the spec's work, or a bug's fix, needs beyond the devShell's.

## Workflow agents add

`/work-backlog-parallel`'s agents run unattended in their own worktrees, and one that meets a prompt holds its slot
and stalls every issue that depends on it, so its request adds the commands they run (its `planning.md` "The
permission request"):

- `Bash(cd *)`: the merge agent runs `cd <worktree> && ./.ci/in-devshell …`.
- `Bash(git switch --detach *)`, `Bash(git branch --move *)` and `Bash(git branch --list *)`: a build agent's branch
  rename and a fix agent's detached checkout (`agents.md`), and the merge queue's branch listing.
- The merge agent runs its git in the rebase worktree as `git -C <rebase worktree> …`, which no rule above matches.
  Each form is its own rule, for the reason the `git -C` bullet above gives, and only the forms `merge-queue.md` and
  `agents.md` "Review" use are named:
  - `Bash(git -C * -c rerere.enabled=false rebase *)`
  - `Bash(git -C * rebase --abort)`
  - `Bash(git -C * switch *)`
  - `Bash(git -C * merge --ff-only *)`
  - `Bash(git -C * checkout --ours *)`
  - `Bash(git -C * add *)`
  - `Bash(git -C * commit *)`
  - `Bash(git -C * log *)`
  - `Bash(git -C * status *)`
  - `Bash(git -C * merge-base *)`
  - `Bash(git -C * diff *)`
  - `Bash(git -C * range-diff *)`
  - `Bash(git -C * blame *)`
  - `Bash(git -C * rev-parse *)`
  - `Bash(git -C * push --force-with-lease=*)`
- `Bash(<main>/.claude/skills/work-backlog-parallel/wait-for *)`: the waits in its `common.md` "Long commands". The
  status files those waits poll live under `.claude/tmp/` in the worktree the command runs in, named by absolute
  path; every agent's worktree, the merge agent's rebase worktree included, lies under `.claude/worktrees/`, which
  that `Edit` rule covers.
- `Edit(/<main>/.claude/backlog/parallel/**)` is covered by the `.claude/backlog/` rule above; the review stage
  writes there.
- `Bash(git worktree unlock *)`, `Bash(kill -0 *)`, `Bash(date --utc *)` and `Bash(sysctl net.ipv4.ip_forward)` (the
  merge queue's cleanup, a build agent's deadline check, the Docker check) from "Rule strings", whatever flows the
  session runs.

## Signing

Every commit and rebase signs with the key `git config --get user.signingkey` names, so a key that is missing or
expires mid-run fails every later commit. Check it the way git uses it, with a test signature: `backlog-preflight`
runs it in step 1 (its `signing` and `agent-key` lines, while the user is there to allow the rule), and the session
runs it again right before the unattended part ("When to ask"), since a key can be unloaded during a long sitting:

```bash
printf x | ssh-keygen -Y sign -n git -f "$(git config --get user.signingkey)" >/dev/null
```

A non-zero exit means the agent does not hold the key: say so in the request. `ssh-add -L` cannot show a key's
lifetime, so `backlog-preflight` prints `agent-key` as a `WARN` asking the user to confirm the key was loaded with
no lifetime that could expire during the run; as a `WARN` it rides in the proposal's `Preflight:` lines, so it is
asked even when the permission request is left out. Commit signing that fails mid-run is a blocked-banner halt.
