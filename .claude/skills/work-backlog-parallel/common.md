# Every workflow agent

Read by every agent `run.workflow.js` starts, before its own section (`agents.md` "Build", "Review" or "Fix", or
`merge-queue.md`). `<main>` is the main checkout's absolute path, from the inputs. Paths without `<main>/` are
relative to your own working directory. Every `/work-backlog` file named here without a path (`SKILL.md` where it
says `/work-backlog`, `bug-flow.md`, `feature-flow.md`, `merging.md`, `sub-skill-overrides.md`, `pending-findings.md`'s
format) is the one under `<main>/.claude/skills/work-backlog/`, never your worktree's copy.

## What to read

Read only what your role needs; the controller already settled everything else.

Every role reads `/work-backlog` `SKILL.md` "Terms" and these bullets of its "Doing the work": "Findings about the
change itself" and "Subagents". Every role also reads the first entry of `sub-skill-overrides.md` (the prompt
templates). Then:

- **Build, bug:** the "PRs", "PR title" and "Branches" bullets of "Doing the work"; the `sub-skill-overrides.md`
  entries for systematic-debugging, test-driven-development and using-git-worktrees (its red-baseline rule);
  `bug-flow.md`.
- **Build, feature:** as for a bug, plus the "Review" bullet, the entries for writing-plans and executing-plans, and
  `feature-flow.md` step 6.
- **Review:** nothing more.
- **Fix:** the "PRs" bullet; the entries for systematic-debugging, test-driven-development and receiving-code-review.
- **Merge:** the "PRs" bullet; the test-driven-development entry; `merging.md` steps 0–2.

Do not read "The filing bar": you never judge a finding (below). A cross-reference in these files to a section of
another file names that file; one with no file name is a section of the file it appears in.

## Rules

- **Nobody is watching.** Never ask a question and never wait for an answer. Every point where a flow you follow
  says to ask the user, halt with a banner or stop for help is a park: return `state: "parked"` with a `parkNote`
  saying what happened, what was tried, the question the user would have been asked and the recommended answer.
  Decisions that do not change what gets built are `Ruling:` lines, returned in `rulings`.
- **Batch-level failures** are not yours to park. They are the blocked banners `/work-backlog` names for a failed
  `gh` call (after its one retry a minute later), commit signing failing (the "Permissions" bullet), and a red
  baseline that is `main`'s own (the using-git-worktrees entry of `sub-skill-overrides.md`; a flaky one is a finding
  there, and parks the issue only if it keeps the work from going on), plus Docker unreachable or
  `sysctl net.ipv4.ip_forward` not 1 when a step needs a container (the user's global Docker rule forbids running
  one then). Return `state: "parked"` with `batchHalt` set to the cause and a `parkNote` naming it.
- **Findings:** return every finding; never open an issue, comment on a PR, leave a `TODO` or decide for yourself
  what is worth reporting (the "Subagents" bullet). Findings about the change
  itself are handled as that bullet says. Every other finding goes in `findings`, in `pending-findings.md`'s entry
  format, unjudged: `whyReal` argues why it is real and fills the entry's "Passes the bar:" line, which in a
  `<N>-findings.md` file means not yet judged; the controller judges it. A test failure you did not cause
  (superpowers:test-driven-development's "goes in your report by name") is a finding.
- **Shared state is not yours.** Never run `git fetch` of anything but your issue's branch, `git pull`,
  `git worktree add` or `remove`, or `git branch --delete`, and never write `current-session.md`,
  `pending-findings.md`, a spec or `.claude/rules/`. In an isolated worktree, Write, Edit and git refuse paths
  outside it, but a plain shell write is not stopped: never make one. The merge agent and the review stage write
  only what their own sections name.
- **Never `ExitWorktree` or `EnterWorktree`.** Where a `/work-backlog` file says to leave the worktree to write
  ("Leaving to write"), the write is not yours: findings and rulings go in your result, and the review stage copies
  the plan.
- **After a context compaction**, re-read this file and your section, and the inputs in your prompt, before
  anything else. Rules such as "never ask", "never fetch `main`" and "never end your turn mid-command" do not
  survive a compaction otherwise.
- **PR bodies** follow the "PRs" bullet: `Closes #N` or `Part of #N` on a line by itself, and every edit before
  a push, never after. Edit by reading the body (`gh pr view <pr> --json body`) and adding new `Ruling:` lines above
  the `Backlog-Session:` marker, never by writing the body anew.
- **Long commands** (`git push`, whose pre-push hook runs the full gate, `./.ci/in-devshell ./run-tests`,
  `gh pr checks --watch`, and superpowers:executing-plans' `task-done` when it runs them) can outlast the Bash
  tool's ten-minute limit. Run such a command with `run_in_background`, appending `; echo $? > <file>` with a file
  of your own under `/tmp`, then wait with foreground calls of at most ten minutes each,
  `timeout 590 bash -c 'until [ -e <file> ]; do sleep 10; done'`, repeated until the file exists; then read the
  output and the exit status. Neither the Monitor tool nor a background notification waits for you: both reach you
  only while you keep working, and ending your turn kills your background shells. Never end your turn while a
  command runs: your final message is your result, and a result sent early is a wrong one. "Wait a minute" is the
  same loop on a file that never appears, with `timeout 60`.
- **A git ref-lock error** (`cannot lock ref`, `index.lock` exists) comes from another agent's command on the shared
  git directory: wait a minute and retry once.
- **Your worktree is the worktree.** superpowers:using-git-worktrees, loaded by executing-plans, finds this one
  already linked; it creates none, runs no project setup and enters nothing, and its baseline step is skipped when
  `agents.md` "Build" already ran it.
- **Output** is the structured object the prompt's schema asks for, with every field present: a field your role
  does not fill is an empty string, an empty list or `false`. `pr` is the number, digits only; `plan` is
  repo-relative (`docs/superpowers/plans/…`); `worktree` is the absolute `pwd` you worked in.
