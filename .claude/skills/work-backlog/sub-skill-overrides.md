# Sub-skill overrides

The superpowers skills this skill calls each pause for the user at points this workflow has already settled. The user
settled them by giving approval ("Terms" in `SKILL.md`). Each entry names the pause and what stands in its place;
everything else in those skills runs as written. The quotes are from superpowers 7.0.0; when the installed version no
longer has a quoted phrase, follow the entry's intent and say so in the session summary.

The first two entries apply to every session, whichever sub-skills it ends up calling.

- **Every prompt template** (`implementer-prompt.md`, `task-reviewer-prompt.md` and `re-review-prompt.md` in
  superpowers:subagent-driven-development; `code-reviewer.md` in superpowers:requesting-code-review): each is used
  with the "Subagents" clause from `SKILL.md` appended (return every finding; never open an issue, leave a `TODO`
  or filter). `code-reviewer.md`'s "check it out into a separate temporary directory (e.g.
  `git worktree add /tmp/review-[SHA] [SHA]`)": the reviewer uses `git diff` and `git show` only, for the reason
  "Subagents" gives.
- **superpowers:finishing-a-development-branch**: not invoked at all, even where another skill ends by naming it.
  "Merging" in `SKILL.md` is the integration: push, PR, merge on green, remove the worktree. Where
  superpowers:subagent-driven-development says "residual load-bearing findings surface to your human partner when
  finishing-a-development-branch presents the options", those findings go through "Findings about the change
  itself" in `SKILL.md` at the ledger sweep instead; their rulings are already bound for the PR body.
- **superpowers:brainstorming**, handing the written design to the user and the hard gate on building: replaced by
  `feature-flow.md` steps 2–5. The builder check, the Fable review and one batched question round are the approval.
- **superpowers:brainstorming**, "If this is really several independent projects … brainstorm them one at a time":
  a feature family is one project, with one brainstorm and one spec that says which issue owns which part.
- **superpowers:brainstorming**, "hand it over with a short list of the calls you made": the calls are recorded in
  the spec and listed in one line at the end of the question round; `feature-flow.md` step 5.
- **superpowers:brainstorming**, "Where it goes: … committed": the spec is written at that path but stays local
  ("State" in `SKILL.md`).
- **superpowers:brainstorming**, "Size the Work" (a small change is built from the chat with no document) and "If it
  grows mid-task, stop, say so, and step up a size": a feature issue always gets the spec, however small
  (`feature-flow.md` step 1), so there is no size to step up; growth found during the unattended part is a `Ruling:`
  when it does not change what gets built, and a question-banner halt when it does.
- **superpowers:brainstorming**, "invoke superpowers:writing-plans and no other skill" after approval: the Fable
  review, the question round and the worktree come first; `feature-flow.md` steps 4–6.
- **superpowers:writing-plans**, "Please review the plan. Which execution approach would you prefer?" and its other
  hand-off, "Please review the plan. Does it capture what you want?": the plan is not user-reviewed, and the
  execution method is chosen without asking. The plan header's "REQUIRED SUB-SKILL: Use
  superpowers:subagent-driven-development (recommended) or superpowers:executing-plans" names the usual two and does
  not bind the choice; `feature-flow.md` step 6.
- **superpowers:writing-plans**, the scope check, "suggest breaking this into separate plans — one per subsystem": a
  family spec covers several issues on purpose, and `feature-flow.md` step 6.3 plans one issue's part at a time,
  which is already that split. Do not suggest it; plan the part named.
- **superpowers:writing-plans**, "If working in an isolated worktree, it should have been created via the
  `superpowers:using-git-worktrees` skill at execution time": the worktree already exists (`feature-flow.md` step
  6.2) and the plan is written inside it.
- **superpowers:using-git-worktrees**, "Would you like me to set up an isolated worktree?" and its native-tool
  step: always yes, created as "Worktrees" in `SKILL.md` says (`git worktree add --no-track -b`, then
  `EnterWorktree` by `path`). "Sandbox fallback" (working in the current directory when `git worktree add` fails):
  never; a failed `git worktree add` is a blocked-banner halt. "If NOT ignored: Add to .gitignore, commit the
  change": `.claude/worktrees/` is already ignored, and nothing is committed for it.
- **superpowers:using-git-worktrees**, project setup (`go mod download` and the like on the host): skipped; the
  devShell supplies every tool. Baseline tests as `npm test / cargo test / …`: the command is
  `./.ci/in-devshell ./run-tests`, run in the background ("Long commands" in `SKILL.md`). "If tests fail: Report
  failures, ask whether to proceed or investigate": a red baseline on a fresh branch from `origin/main` is a
  blocked-banner halt, since `main` itself is broken.
- **superpowers:subagent-driven-development** and **superpowers:executing-plans**, "Four things stop you, and only
  these: an irreversible or destructive operation; a security-sensitive action; a side effect outside this worktree
  that norms say you ask about first (a merge, a push to a shared branch, a publish); and a plan so broken that every
  path forward is a guess": pushing the branch and merging its PR on green CI are approved ("Merging" in `SKILL.md`).
  An irreversible, destructive or security-sensitive step the plan did not name is a blocked-banner halt. A plan so
  broken that every path is a guess is a question-banner halt, handled like a bug that fails the bar (`bug-flow.md`
  step 3): explain, recommend, and the rest of the chunk goes on after the answer.
- **superpowers:subagent-driven-development**, deferred minors and parked findings left in the ledger for the user:
  the ledger sweep ("Review" in `SKILL.md`) handles them before the PR merges. The fix loops keep their own round
  counts; the two-round cap in `SKILL.md` is only for superpowers:requesting-code-review. "Never fix findings
  yourself in the controller session" still holds: the sweep's fixes are one fix dispatch plus one scoped re-review
  (`re-review-prompt.md`), as the final fix wave is.
- **superpowers:executing-plans**, "Final: minor (deferred)" lines left in the ledger for the user: the ledger sweep
  handles them before the PR merges. This skill has no fix dispatch and no re-review ("Fix the Critical and
  Important findings yourself … in ONE pass", "Do not dispatch a re-review"), so the controller makes the sweep's
  fixes itself, in one pass, as that skill makes its own.
- **superpowers:subagent-driven-development** and **superpowers:executing-plans**, "delete this plan's workspace
  (`rm -rf <workspace>`)" at their finish: the ledger sweep runs first, because the skills collect only `Ruling:`
  lines before the delete and the sweep needs the deferred, parked and out-of-scope lines too. Delete the workspace
  after the sweep, before the PR merges.
- **superpowers:subagent-driven-development** and **superpowers:executing-plans**, rulings reported to the user in
  the final message, and **superpowers:requesting-code-review**, "Declined to judge … The executor rules on each
  line": every ruling goes in the PR body ("PRs" in `SKILL.md`), for every PR, since the ledger is deleted with the
  worktree and non-plan work has no ledger at all.
- **superpowers:subagent-driven-development**, an implementer's question ("Ask them now", "It's always OK to pause
  and clarify"): the controller answers it. A question whose answer changes what gets built is a question-banner halt
  to the user; any other is a `Ruling:`.
- **superpowers:requesting-code-review**, "Note Minor issues for later" and a `BASE_SHA` of `HEAD~1`: there is no
  later ("Findings about the change itself" in `SKILL.md`), and the base is `git merge-base origin/main HEAD`.
- **superpowers:test-driven-development**, "Exceptions (ask your human partner)" and "No exceptions without your
  human partner's permission": no exception is asked for. Configuration gets the test the repo has for it (BATS e2e
  against the image, `.ci/check-examples`, the workflow lints); a change that truly has no test is a `Ruling:` in
  the PR body. "Don't know how to test | … Ask your human partner": decide and record a `Ruling:`; the question
  banner only when the answer changes what gets built.
- **superpowers:systematic-debugging**, Phase 4.5 "Discuss with your human partner before attempting more fixes"
  (three failed fixes) and "Ask for help": a question-banner halt handled like a bug that fails the bar
  (`bug-flow.md` step 3): explain what was tried, recommend, and the rest of the chunk goes on after the answer.
- **superpowers:systematic-debugging**, a cause found to be "truly environmental, timing-dependent, or external"
  with "Implement appropriate handling (retry, timeout, error message); Add monitoring/logging": no handling is
  added on that finding alone. The bug is re-judged by the filing bar; one that fails it goes to the user as
  `bug-flow.md` step 3 says, and one that passes gets the fix its real trigger calls for.
