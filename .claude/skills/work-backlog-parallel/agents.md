# Workflow agents

Instructions for the agents `run.workflow.js` starts for each issue. Each agent's prompt names its section here and
passes its inputs as JSON; the agent reads this file from the main checkout (`<main>` below) and follows "Every
agent" plus its own section. Paths without `<main>/` are relative to the agent's own working directory.

The roles, and where each runs:

| Role | Runs in | Writes |
| --- | --- | --- |
| Build | its own isolated worktree (`isolation: 'worktree'`) | that worktree, its branch, the PR |
| Review | the main checkout, not isolated | `<main>/.claude/backlog/parallel/`, `<main>/docs/superpowers/plans/` |
| Fix | its own isolated worktree | that worktree, the branch (by push), the PR body |

The merge agent is `merge-queue.md`.

## Every agent

- **Read first:** `<main>/.claude/skills/work-backlog/SKILL.md` ("Terms", "The filing bar", "PRs", "Findings about
  the change itself", "Long commands") and `<main>/.claude/skills/work-backlog-parallel/SKILL.md` ("Named
  exceptions"). They govern wherever this file is silent.
- **Nobody is watching.** Never ask a question and never wait for an answer. Every point where a flow you follow
  says to ask the user, halt with a banner or stop for help is a park: return `state: "parked"` with a
  `parkNote` saying what happened, what was tried, the question the user would have been asked and the recommended
  answer. Decisions that do not change what gets built are `Ruling:` lines, returned in `rulings`.
- **Batch-level failures** are not yours to park: `gh auth status --hostname github.com` failing, any `gh` call
  failing twice a minute apart, Docker unreachable when a step needs it. Return `state: "parked"` with
  `batchHalt` set to the cause.
- **Findings:** return every finding; never open an issue, comment on a PR, leave a `TODO` or decide for yourself
  what is worth reporting (the "Subagents" clause). Findings about the change itself are handled as `/work-backlog`
  says; every other finding goes in `findings`, in `pending-findings.md`'s entry format, unjudged.
- **Shared state is not yours.** Never run `git fetch` of anything but your issue's branch, `git pull`,
  `git worktree add` or `remove`, or `git branch --delete`, and never write `current-session.md`,
  `pending-findings.md`, a spec or `.claude/rules/`. Builders cannot write outside their worktree anyway.
- **Long commands** (`git push`, whose pre-push hook runs the full gate, and `./.ci/in-devshell ./run-tests`) run
  with `run_in_background`; read the output when the command exits.
- **Output** is the structured object the prompt's schema asks for; `worktree` is always the absolute `pwd` you
  worked in.

## Build

Inputs: `n`, `kind` (`bug` or `feature`), `spec` (path in the main checkout, empty for a bug), `date`, `main`.

1. **Reconcile.** An earlier agent may have started this issue (the workflow resumes interrupted agents from their
   original prompt).
   - `gh pr list --state all --search "#<n> in:body" --json number,state,headRefName,body`, keeping PRs whose body
     ends with `Backlog-Session: <date>` and names `Closes #<n>` or `Part of #<n>`. A `MERGED` one: return
     `state: "merged"`. An `OPEN` one: its `headRefName` is the branch.
   - Otherwise `git branch --list '*/<n>-*'`: a local branch from a dead builder is the branch.
   - With a branch found: `git switch --detach <branch>` (a branch checked out in a dead builder's worktree cannot be
     checked out here), find that worktree with `git worktree list`, and read its ledger
     (`.superpowers/sdd/*/progress.md`) and plan with the Read tool. Work detached from here on and push with
     `git push origin HEAD:refs/heads/<branch>`. Resume the plan from the ledger; work never committed in the dead
     worktree is lost and is redone.
2. **Fresh start** (nothing found): `gh issue view <n> --json state --jq .state`; `CLOSED` returns
   `state: "skipped"`. Rename the branch with `git branch -m <type>/<n>-<description>` ("Branches"). Copy
   `<main>/.claude/CLAUDE.md`, `<main>/.claude/rules/` and the spec to the same relative paths here
   (`mkdir --parents .claude docs/superpowers/specs docs/superpowers/plans` first), and read the copied rules that
   apply to the files you will change.
3. **Build.**
   - **Bug:** `bug-flow.md` steps 2–4. "Not a bug" (step 3) parks, with "close as completed" or "close as not
     planned" as the recommended answer.
   - **Feature:** `feature-flow.md` step 6.3, planning only this issue's part of the spec, then step 6.4 with
     superpowers:executing-plans (the only executor available: superpowers:subagent-driven-development needs a
     subagent tool this agent lacks). `sub-skill-overrides.md` applies. Skip executing-plans' "Final Review"
     entirely; the review stage replaces it. Before its "Finish" deletes the workspace, run the ledger sweep
     ("Review" in `/work-backlog`): in-scope deferred findings are fixed, known-gap-commented or dropped,
     out-of-scope notes go in `findings`, and every `Ruling:` goes in `rulings`. Where the merged code of an earlier
     piece contradicts the spec, the code wins and the divergence is a ruling.
4. **Push and open the PR.** `git push --set-upstream origin <branch>` (or the detached form from step 1). A
   pre-push gate failure is fixed and pushed again, as a failing test would be. Then `gh pr create` per "PRs" and
   "PR title" in `/work-backlog`: `Closes #<n>` or `Part of #<n>`, the rulings, ending with
   `Backlog-Session: <date>`. An open PR from step 1 gets `gh pr edit <pr> --body` with the rulings instead.
5. **Return** `state: "ready"`, `branch`, `pr`, `plan` (the plan's path in this worktree, or empty), `worktree`,
   `rulings`, `findings`.

## Review

Inputs: `n`, `round` (1 or 2), `branch`, `pr`, `headWorktree` (where the latest commit was made), `plan`, `build`
(the build or fix agent's output, verbatim), `main`.

1. **Record.** Write `<main>/.claude/backlog/parallel/<n>.md` ("State" in `SKILL.md`) from `build`: `state: ready`,
   `branch:`, `pr:`, `plan:`, every worktree path seen so far under `worktrees:`, every ruling. Append each entry of
   `build.findings` to `<main>/.claude/backlog/parallel/<n>-findings.md`. In round 1, when `plan` is set, copy it to
   `<main>/docs/superpowers/plans/`.
2. **Review** the branch as superpowers:requesting-code-review's `code-reviewer.md` describes, with the
   "Subagents" clause: base `git -C <headWorktree> merge-base origin/main HEAD`, head `HEAD` of `headWorktree`,
   inspected with `git -C <headWorktree> diff` and `git -C <headWorktree> show` only. The requirements are the
   issue (`gh issue view <n>`) and, for a feature, the spec and plan. Do not edit any file in any worktree.
3. **Sort** each finding: about the change itself (a hole in this PR's code or tests) goes in `changeFindings`;
   anything else is appended to `<n>-findings.md` and listed in `otherFindings`.
4. **Return** `changeFindings`, `otherFindings`.

## Fix

Inputs: `n`, `round`, `branch`, `pr`, `changeFindings`, `date`, `main`.

1. **Reconcile and check out:** `git fetch origin <branch>`, then `git switch --detach FETCH_HEAD`. If a commit on
   the branch already says it addresses this round's findings (a resumed fix), continue from what remains.
2. **Handle each finding** as "Findings about the change itself" says: fixed (test first, then fix, as separate
   commits where a test applies), given a known-gap comment, or dropped with a ruling. Three failed attempts at one
   finding park the issue (`sub-skill-overrides.md`, the systematic-debugging entry).
3. **Push** with `git push origin HEAD:refs/heads/<branch>` (the pre-push gate runs), and add any new rulings to the
   PR body with `gh pr edit <pr> --body`.
4. **Return** `state: "ready"`, `worktree`, `rulings`, `findings`.
