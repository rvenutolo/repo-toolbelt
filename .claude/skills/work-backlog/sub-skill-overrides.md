# Sub-skill overrides

The superpowers skills this skill calls each pause for the user at points this workflow has already settled. The user
settled them by approving the chunk (and, for a feature, the design). Each entry names the pause and what stands in
its place; everything else in those skills runs as written. The quotes are from superpowers 7.0.0; when the installed
version no longer has a quoted phrase, follow the entry's intent and say so in the session summary.

- **superpowers:brainstorming**, handing the written design to the user and the hard gate on building: replaced by
  `feature-flow.md` steps 2–5. The builder check, the Fable review and one batched question round are the approval.
- **superpowers:brainstorming**, "If this is really several independent projects … brainstorm them one at a time":
  a feature family is one project, with one brainstorm and one spec that says which issue owns which part.
- **superpowers:brainstorming**, "hand it over with a short list of the calls you made": the calls are recorded in
  the spec and listed in one line at the end of the question round; `feature-flow.md` step 5.
- **superpowers:brainstorming**, "Where it goes: … committed": the spec is written at that path but stays local,
  because `docs/superpowers/` is gitignored.
- **superpowers:brainstorming**, "Size the Work" (a small change is built from the chat with no document): a feature
  issue always gets the spec, however small; `feature-flow.md` step 1.
- **superpowers:brainstorming**, "invoke superpowers:writing-plans and no other skill" after approval: the Fable
  review, the question round and the worktree come first; `feature-flow.md` steps 4–6.
- **superpowers:writing-plans**, "Please review the plan. Which execution approach would you prefer?": the plan is
  not user-reviewed, and the execution method is chosen without asking. The plan header's
  "REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans"
  names the usual two and does not bind the choice; `feature-flow.md` step 6.
- **superpowers:using-git-worktrees**, "Would you like me to set up an isolated worktree?" and its native-tool
  step: always yes, created as "Worktrees" in `SKILL.md` says (`git worktree add --no-track -b`, then
  `EnterWorktree` by `path`). "Sandbox fallback" (working in the current directory when `git worktree add` fails):
  never; a failed `git worktree add` is a blocked-banner halt. "If NOT ignored: Add to .gitignore, commit the
  change": `.claude/worktrees/` is already ignored, and nothing is committed for it.
- **superpowers:using-git-worktrees**, project setup (`go mod download` and the like on the host): skipped; the
  devShell supplies every tool. Baseline tests as `npm test / cargo test / …`: the command is
  `./.ci/in-devshell ./run-tests`. "If tests fail: Report failures, ask whether to proceed or investigate": a red
  baseline on a fresh branch from `origin/main` is a blocked-banner halt, since `main` itself is broken.
- **superpowers:subagent-driven-development** and **superpowers:executing-plans**, "a merge, a push to a shared
  branch … stop and ask": pushing the branch and merging its PR on green CI are approved with the chunk; see
  "Merging" in `SKILL.md`.
- **superpowers:subagent-driven-development** and **superpowers:executing-plans**, deferred minors left in the
  ledger for the user: in-scope deferred findings are fixed, known-gap-commented or dropped before the PR opens;
  out-of-scope ones go through the filing bar. Their fix loops keep their own round counts; the two-round cap in
  `SKILL.md` is only for superpowers:requesting-code-review. "Never fix findings yourself in the controller
  session" still holds: the deferred fixes are one fix dispatch plus one scoped re-review (`re-review-prompt.md`),
  as the final fix wave is.
- **superpowers:subagent-driven-development** and **superpowers:executing-plans**, rulings reported to the user in
  the final message, and **superpowers:requesting-code-review**, "Declined to judge … The executor rules on each
  line": every ruling goes in the PR body ("PR title" in `SKILL.md`), for every PR, since the ledger is deleted
  with the worktree and non-plan work has no ledger at all.
- **superpowers:subagent-driven-development** (`implementer-prompt.md`, `task-reviewer-prompt.md`,
  `re-review-prompt.md`) and **superpowers:requesting-code-review** (`code-reviewer.md`): each template is used
  with the "Subagents" clause from `SKILL.md` appended (return every finding; never open an issue, leave a `TODO`
  or filter). `code-reviewer.md`'s "check it out into a separate temporary directory (e.g.
  `git worktree add /tmp/review-[SHA] [SHA]`)": the reviewer uses `git diff` and `git show` only.
- **superpowers:finishing-a-development-branch**: not invoked at all, even where another skill ends by naming it.
  "Merging" in `SKILL.md` is the integration: push, PR, merge on green, remove the worktree.
- **superpowers:requesting-code-review**, "Note Minor issues for later" and a `BASE_SHA` of `HEAD~1`: there is no
  later (fix, known-gap comment, or drop), and the base is `git merge-base origin/main HEAD`.
- **superpowers:test-driven-development**, "Exceptions (ask your human partner)" and "No exceptions without your
  human partner's permission": no exception is asked for. Configuration gets the test the repo has for it (BATS e2e
  against the image, `.ci/check-examples`, the workflow lints); a change that truly has no test is a `Ruling:` in
  the PR body. "Don't know how to test | … Ask your human partner": decide and record a `Ruling:`; the question
  banner only when the answer changes what gets built.
- **superpowers:systematic-debugging**, Phase 4.5 "Discuss with your human partner before attempting more fixes"
  (three failed fixes) and "Ask for help": a question-banner halt handled like a bug that fails the bar ("Bugs" in
  `SKILL.md`): explain what was tried, recommend, and the rest of the chunk goes on after the answer.
