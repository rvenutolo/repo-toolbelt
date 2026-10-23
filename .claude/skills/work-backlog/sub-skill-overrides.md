# Sub-skill overrides

The superpowers skills this skill calls each pause for the user at points this workflow has already settled. The user
settled them by approving the chunk (and, for a feature, the design). Each entry names the pause and what stands in
its place; everything else in those skills runs as written.

- **superpowers:brainstorming**, handing the written design to the user and the hard gate on building: replaced by
  `feature-flow.md` steps 2–5. The builder check, the Fable review and one batched question round are the approval.
- **superpowers:brainstorming**, "Where it goes: … committed": the spec is written at that path but stays local,
  because `docs/superpowers/` is gitignored.
- **superpowers:brainstorming**, "Size the Work" (a small change is built from the chat with no document): a feature
  issue always gets the spec, however small; `feature-flow.md` step 1.
- **superpowers:brainstorming**, "invoke superpowers:writing-plans and no other skill" after approval: the Fable
  review, the question round and the worktree come first; `feature-flow.md` steps 4–6.
- **superpowers:writing-plans**, "Please review the plan. Which execution approach?": the plan is not user-reviewed,
  and the execution method is chosen without asking.
- **superpowers:using-git-worktrees**, "Would you like me to set up an isolated worktree?" and its native-tool
  step: always yes, created as "Worktrees" in `SKILL.md` says (`git worktree add -b`, then `EnterWorktree` by
  `path`).
- **superpowers:using-git-worktrees**, project setup (`go mod download` and the like on the host): skipped; the
  devShell supplies every tool. Baseline tests as `npm test / cargo test / …`: the command is
  `./.ci/in-devshell ./run-tests`.
- **superpowers:subagent-driven-development** and **superpowers:executing-plans**, "a merge, a push to a shared
  branch … stop and ask": pushing the branch and merging its PR on green CI are approved with the chunk; see
  "Merging" in `SKILL.md`.
- **superpowers:subagent-driven-development** and **superpowers:executing-plans**, deferred minors left in the
  ledger for the user: in-scope deferred findings are fixed, known-gap-commented or dropped before the PR opens;
  out-of-scope ones go through the filing bar. Their fix loops keep their own round counts; the two-round cap in
  `SKILL.md` is only for superpowers:requesting-code-review.
- **superpowers:subagent-driven-development** and **superpowers:executing-plans**, rulings reported to the user in
  the final message: they go in the PR body ("PR title" in `SKILL.md`), since the ledger is deleted with the worktree.
- **superpowers:finishing-a-development-branch**: not invoked at all, even where another skill ends by naming it.
  "Merging" in `SKILL.md` is the integration: push, PR, merge on green, remove the worktree.
- **superpowers:requesting-code-review**, "Note Minor issues for later" and a `BASE_SHA` of `HEAD~1`: there is no
  later (fix, known-gap comment, or drop), and the base is `git merge-base origin/main HEAD`.
