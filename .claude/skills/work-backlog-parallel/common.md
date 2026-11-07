# Every workflow agent

Read by every agent `run.workflow.js` starts, before its own section (`agents.md` "Build", "Review" or "Fix", or
`merge-queue.md`). `<main>` is the main checkout's absolute path, from the inputs. Paths without `<main>/` are
relative to your own working directory. Every `/work-backlog` file named here without a path (`SKILL.md` where it
says `/work-backlog`, `bug-flow.md`, `feature-flow.md`, `merging.md`, `sub-skill-overrides.md`, and
`pending-findings.md`'s entry format, which is in `state.md`) is the one under `<main>/.claude/skills/work-backlog/`,
never your worktree's copy.

## What to read

Read only what your role needs; the controller already settled everything else.

Every role reads `/work-backlog` `SKILL.md` "Terms", its "Never" section, and these bullets of its "Doing the work":
"Findings about the change itself", "Subagents" and "Copied rules". Every role also reads the first two entries of
`sub-skill-overrides.md` (the prompt templates, and the skill that is never invoked, which executing-plans' "Finish"
would otherwise call). Then:

- **Build, bug:** the "PRs", "PR title", "Branches" and "Worktrees" bullets of "Doing the work" and the "Comments"
  bullet of "Choosing the chunk"; the
  `sub-skill-overrides.md` entries for systematic-debugging, test-driven-development and using-git-worktrees (its
  red-baseline rule); `bug-flow.md`; and "The filing bar", used only to judge your own issue (`bug-flow.md` step 3).
- **Build, feature:** as for a bug, plus the "Review" bullet, the entries for writing-plans and executing-plans, and
  `feature-flow.md` step 6.
- **Review:** the "Comments" bullet of "Choosing the chunk".
- **Fix:** the "PRs" bullet; the entries for systematic-debugging, test-driven-development and receiving-code-review.
- **Merge:** the "PRs" bullet; the test-driven-development entry; `merging.md` steps 0–2.

Apart from that one use, do not read "The filing bar": you never judge an out-of-scope finding (below). A name
called a bullet, or "Leaving to write", is in `/work-backlog` `SKILL.md` "Doing the work", except the rules of this
file's own "Rules" list ("Long commands", "PR bodies" and the rest), which are here; a cross-reference to a
section of another file names that file; any other unqualified name is a section of the file it appears in. Wherever
a `/work-backlog` file says to run something as "Long commands" says, to run it in the background, or to "wait a
minute", this file's "Long commands" rule applies instead: a background notification reaches you only while you keep
working, and nothing that arrives after your final message changes it.

## Rules

- **Nobody is watching.** Never ask a question and never wait for an answer. Every point where a flow you follow
  says to ask the user, halt with a banner or stop for help is a park: return `state: "parked"` with a `parkNote`
  saying what happened, what was tried, the question the user would have been asked and the recommended answer.
  Decisions that do not change what gets built are `Ruling:` lines, returned in `rulings`. A conflict with a copied
  rule is not a park: the "Copied rules" bullet settles it.
- **Batch-level failures** are not yours to park. They are these of the blocked banners `/work-backlog` `SKILL.md`
  "How the session runs" lists: a failed `gh` call (after its one retry a minute later), commit signing failing
  (`permissions.md` "Signing"), Docker unreachable or `sysctl net.ipv4.ip_forward` not 1 right before the one step
  that needs a container (`/work-backlog` `SKILL.md` "How the session runs" names it; the check runs nowhere else);
  plus a red baseline that is `main`'s own (the using-git-worktrees entry of `sub-skill-overrides.md`; a
  flaky one is a finding there, and parks the issue only if it keeps the work from going on). Each would stop every
  other agent too: the red baseline because every isolated worktree starts from the same `origin/main`. Return
  `state: "parked"` with `batchHalt` set to the cause and a `parkNote` naming it.
- **Findings:** the "Subagents" bullet governs, and findings about the change itself are handled as their bullet
  says. Every other finding goes in `findings`, unjudged; the controller judges it. The fields map onto
  `pending-findings.md`'s entry format one to one: `symptom` is the entry's heading line, `file` is `File:`,
  `trigger` is `Trigger:`, `whyReal` is `Passes the bar:` (your case that it is real, which in a `<N>-findings.md`
  file means not yet judged) and `foundWhile` is `Found while:`. A test failure you did not cause is a finding, as
  the test-driven-development entry of `sub-skill-overrides.md` says.
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
- **Summary items:** every item that `/work-backlog` `SKILL.md` "End-of-run report" lists under `Session notes` or
  `Consumer-impacting changes` and that only you can see (read those two bullets) goes in `rulings` as a line
  starting `Summary:`, so the controller's summary names it; the controller never sees the code, so it cannot
  notice these itself. A consumer-impacting change is also a `Ruling:` in the PR body, as `halt-questions.md`'s
  default for that case asks; the `Summary:` line is its copy for the controller. `Summary:` lines are not rulings
  on the change: never put one in the PR body.
- **PR bodies** follow the "PRs" bullet, its 100-character line limit and its body-edit procedure included (read the
  body, add the new `Ruling:` lines above the marker, never write the body anew).
- **The superpowers plugin directory** is what `<main>/.claude/skills/work-backlog/permission-diff --superpowers-dir`
  prints: `code-reviewer.md` is `<it>/skills/requesting-code-review/code-reviewer.md`, and the plan scripts are under
  `<it>/skills/<skill>/scripts/`. The config directory is not `~/.claude`, so a guessed path misses.
- **Long commands** (`git push`, whose pre-push hook runs the full gate, `./.ci/in-devshell ./run-tests`,
  `gh pr checks --watch`, and superpowers:executing-plans' `task-done` when it runs them) can outlast the Bash
  tool's limits. Each one writes its exit status to a file in the worktree it runs in, by absolute path:
  `<wt>/.claude/tmp/<n>-<role>-<step>-<k>.status`, where `<wt>` is that worktree (your `pwd`; for the merge agent,
  its rebase worktree), `<role>` is `build`, `fix` or `merge`, `<step>` names the command (`push`, `tests`, `ci`),
  and `<k>` is the lowest number with no such file in `ls <wt>/.claude/tmp/`. A worktree outlives one agent (a
  resumed build, three merge calls), so a name an earlier agent used would return its old status at once.
  `.claude/tmp/` is ignored by git.
  1. `mkdir --parents <wt>/.claude/tmp`, then run the command with `run_in_background`, as
     `cd <wt> && timeout 30m <command>; echo $? > <status path>` (devshell commands as `cd <wt> && timeout 30m
     ./.ci/in-devshell …; echo $? > <status path>`). The `cd` goes in front of every long command, since an earlier
     call's `cd` may have left the shell elsewhere and a push from the wrong tree runs the wrong hook. The absolute
     path is what makes the write and the wait name the same file whatever directory each call runs in. The status
     is the whole list's exit: a `1` with no output from the command means the `cd` failed, not the command.
  2. Wait with foreground calls of `<main>/.claude/skills/work-backlog-parallel/wait-for <status path> 580`, each
     passing the Bash tool `timeout: 600000`: the tool's default limit is shorter, and a call past it moves to the
     background, where its end never reaches you. A call the tool still cuts off counts as exit 124 (not written
     yet). Repeat on 124, four calls at most: `timeout 30m` writes a status within 30 minutes, so a fourth 124, or a
     status of 124, means the command hung. That is a park, or a batch-level failure when its cause is one (the
     "Batch-level failures" rule above), with the command and its last output in the note.
  3. Read the output and the exit status.

  Use that helper, by that path, for every wait: the allow rule that pre-approves waits names it, and a
  `timeout … bash -c` loop or a write under `/tmp` would stop at a permission prompt nobody answers. Neither the
  Monitor tool nor a background notification waits for you: both reach you only while you keep working. Your final
  message is your result and nothing that arrives after it can change it; a background shell still running when you
  finish is stopped a few minutes later, its exit unseen. Never end your turn while a command runs. "Wait a minute"
  is `wait-for` on a status path nothing writes, with `60`.

  `git commit` is a long command of another kind: run it as `/work-backlog` "Long commands" says (in the
  foreground, with the Bash tool `timeout: 600000`); it needs no status file.
- **A git ref-lock error** (`cannot lock ref`, `index.lock` exists) comes from another agent's command on the shared
  git directory: wait a minute and retry once.
- **Your worktree is the worktree.** superpowers:using-git-worktrees, loaded by executing-plans, finds this one
  already linked; it creates none, runs no project setup and enters nothing, and its baseline step is skipped when
  `agents.md` "Build" already ran it.
- **Output** is the structured object the prompt's schema asks for, with every field present: a field your role
  does not fill is an empty string, an empty list or `false`. `pr` is the number, digits only; `plan` is
  repo-relative (`docs/superpowers/plans/…`); `worktree` is the absolute `pwd` you worked in, or the worktree
  `agents.md` names where it says so. Return `branch` and `pr` whenever they exist, whatever the state: a park after
  the PR opened must still name it, or the user is sent to "read its PR" with no number.
