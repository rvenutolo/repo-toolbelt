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
  the change itself"), `<main>/.claude/skills/work-backlog/sub-skill-overrides.md` (every entry applies to every
  role) and `<main>/.claude/skills/work-backlog-parallel/SKILL.md` ("Named exceptions"). They govern wherever this
  file is silent. Every `/work-backlog` file this file names without a path (`bug-flow.md`, `feature-flow.md`,
  `merging.md`, `pending-findings.md`'s format) is the one under `<main>/.claude/skills/work-backlog/`, not your
  worktree's copy.
- **Nobody is watching.** Never ask a question and never wait for an answer. Every point where a flow you follow
  says to ask the user, halt with a banner or stop for help is a park: return `state: "parked"` with a
  `parkNote` saying what happened, what was tried, the question the user would have been asked and the recommended
  answer. Decisions that do not change what gets built are `Ruling:` lines, returned in `rulings`.
- **Batch-level failures** are not yours to park: `gh auth status --hostname github.com` failing, any `gh` call
  failing twice a minute apart, Docker unreachable or `sysctl net.ipv4.ip_forward` not 1 when a step needs a
  container (the user's global Docker rule forbids running one then), and a red baseline on a fresh worktree
  (`main` itself is broken). Return `state: "parked"` with `batchHalt` set to the cause.
- **Findings:** return every finding; never open an issue, comment on a PR, leave a `TODO` or decide for yourself
  what is worth reporting (the "Subagents" clause). Findings about the change itself are handled as `/work-backlog`
  says; every other finding goes in `findings`, in `pending-findings.md`'s entry format, unjudged. A test failure
  you did not cause (superpowers:test-driven-development's "goes in your report by name") is a finding.
- **Shared state is not yours.** Never run `git fetch` of anything but your issue's branch, `git pull`,
  `git worktree add` or `remove`, or `git branch --delete`, and never write `current-session.md`,
  `pending-findings.md`, a spec or `.claude/rules/`. Builders cannot write outside their worktree anyway.
- **Long commands** (`git push`, whose pre-push hook runs the full gate, `./.ci/in-devshell ./run-tests`, and
  superpowers:executing-plans' `task-done` when it runs them) can outlast the Bash tool's ten-minute limit. Run
  them with `run_in_background`, then wait for them with the Monitor tool until the command exits, and read its
  output. Never end your turn while one is running: your final message is your result, and a result sent early is
  a wrong one.
- **Your worktree is the worktree.** superpowers:using-git-worktrees, loaded by executing-plans, finds this one
  already linked; it creates none, runs no project setup and enters nothing (`sub-skill-overrides.md` entries for
  it apply except the creation command, which the workflow did).
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
   - With a branch found: `git fetch origin <branch>` (when the remote has it), then `git switch --detach` to
     whichever of `<branch>` and `FETCH_HEAD` contains the other (`git merge-base --is-ancestor`); if neither does,
     park, since work exists in two places. A branch checked out in a dead builder's worktree cannot be checked out
     here, hence detached. Find that worktree with `git worktree list`. Copy its plan
     (`docs/superpowers/plans/`) and its ledger (`.superpowers/sdd/<plan-basename>/progress.md`) to the same relative
     paths here, since executing-plans looks for the ledger under the current worktree only and would otherwise
     start over. Work detached from here on and push with `git push origin HEAD:refs/heads/<branch>`. Resume the
     plan from the ledger; work never committed in the dead worktree is lost and is redone.
2. **Fresh start** (nothing found): `gh issue view <n> --json state --jq .state`; `CLOSED` returns
   `state: "skipped"`. Rename the branch with `git branch -m <type>/<n>-<description>` ("Branches"). Copy
   `<main>/.claude/CLAUDE.md`, `<main>/.claude/rules/` and the spec to the same relative paths here
   (`mkdir --parents .claude docs/superpowers/specs docs/superpowers/plans` first), and read the copied rules that
   apply to the files you will change.
3. **Build.**
   - **Bug:** `bug-flow.md` steps 2–4. "Not a bug" (step 3) parks, with "close as completed" or "close as not
     planned" as the recommended answer; skip step 3's cleanup (no worktree remove, branch delete or rebase: the
     merge queue and `leftovers.md` own those).
   - **Feature:** `feature-flow.md` step 6.3, planning only this issue's part of the spec, then step 6.4 with
     superpowers:executing-plans (the only executor available: superpowers:subagent-driven-development needs a
     subagent tool this agent lacks). Skip executing-plans' "Final Review"
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

Inputs: `n`, `round` (1 or 2), `branch`, `pr`, `headWorktree` (where the latest commit was made), `plan` (the build
agent's plan path), `worktrees` (every worktree path the issue's agents used so far), `build` (the latest build or
fix agent's output, verbatim), `main`.

1. **Record.** Write `<main>/.claude/backlog/parallel/<n>.md` ("State" in `SKILL.md`) from the inputs: `state:
   ready`, `branch:`, `pr:`, `plan:` (from the `plan` input, never from `build`, whose `plan` a fix agent leaves
   empty), `worktrees:`, and every ruling already in the file plus `build.rulings`. Append each entry of
   `build.findings` to `<main>/.claude/backlog/parallel/<n>-findings.md`. In round 1, when `plan` is set, copy it to
   `<main>/docs/superpowers/plans/`.
2. **Review** the branch as superpowers:requesting-code-review's `code-reviewer.md` describes, with the
   "Subagents" clause: base `git -C <headWorktree> merge-base origin/main HEAD`, head `HEAD` of `headWorktree`,
   inspected with `git -C <headWorktree> diff` and `git -C <headWorktree> show` only. The requirements are the
   issue (`gh issue view <n>`) and, for a feature, the spec and plan. Do not edit any file in any worktree.
3. **Sort** each finding: about the change itself (a hole in this PR's code or tests) goes in `changeFindings`;
   anything else is appended to `<n>-findings.md` and listed in `otherFindings`. Each "Declined to judge" line
   `code-reviewer.md` produces goes in `changeFindings` with its summary starting `Declined to judge:`, so the fix
   agent rules on it. The pre-push gate already ran the tests; the review does not rerun them.
4. **Return** `changeFindings`, `otherFindings`, and `batchHalt` (empty unless a batch-level failure stopped the
   review; then the findings are whatever was gathered).

**A conflict resolution** (inputs `resolution: true`, `mergeWorktree`, `conflictedFiles`, `branch`, `pr`, `n`,
`main`, from `merge-queue.md`): skip step 1. Review only how the conflicts in `conflictedFiles` were resolved:
`git -C <mergeWorktree> diff origin/main HEAD -- <files>` against what each side's PR meant
(`gh pr view <pr>` and the `main` commits that touched those files). A wrong or lossy resolution is a
`changeFindings` entry; a finding outside the resolution follows step 3 and is not this review's business beyond
that.

## Fix

Inputs: `n`, `round`, `branch`, `pr`, `changeFindings`, `date`, `main`.

1. **Reconcile and check out:** `git fetch origin <branch>`, then `git switch --detach FETCH_HEAD`. If a commit on
   the branch already says it addresses this round's findings (a resumed fix), continue from what remains.
2. **Handle each finding** as "Findings about the change itself" says: fixed (test first, then fix, as separate
   commits where a test applies), given a known-gap comment, or dropped with a ruling. A `Declined to judge:` entry
   always gets a ruling: the behavior stands, or it is now a finding and handled as one. Three failed attempts at one
   finding park the issue (`sub-skill-overrides.md`, the systematic-debugging entry).
3. **Push** with `git push origin HEAD:refs/heads/<branch>` (the pre-push gate runs), and add any new rulings to the
   PR body with `gh pr edit <pr> --body`.
4. **Return** `state: "ready"`, `worktree`, `rulings`, `findings`.
