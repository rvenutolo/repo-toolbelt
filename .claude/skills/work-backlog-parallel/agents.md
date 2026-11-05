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
  that errors twice a minute apart, Docker unreachable or `sysctl net.ipv4.ip_forward` not 1 when a step needs a
  container (the user's global Docker rule forbids running one then), commit signing failing (the key has left
  `ssh-agent`; every later commit would fail too), the main checkout's `HEAD` not an ancestor of `origin/main` (the
  user committed on or switched local `main`, and a builder branching from it would carry that work into its PR),
  and a red baseline that stays red on a rerun while the latest `ci.yml` run on `main`
  (`gh run list --branch main --workflow ci.yml --limit 1 --json conclusion`) is red too (`main` itself is broken).
  A baseline red only here, under four concurrent gates, is a flaky test: a finding, and the issue parks if it keeps
  the work from going on. Return `state: "parked"` with `batchHalt` set to the cause and a `parkNote` naming it.
- **Findings:** return every finding; never open an issue, comment on a PR, leave a `TODO` or decide for yourself
  what is worth reporting (the "Subagents" clause). Findings about the change itself are handled as `/work-backlog`
  says; every other finding goes in `findings`, in `pending-findings.md`'s entry format, unjudged. A test failure
  you did not cause (superpowers:test-driven-development's "goes in your report by name") is a finding.
- **Shared state is not yours.** Never run `git fetch` of anything but your issue's branch, `git pull`,
  `git worktree add` or `remove`, or `git branch --delete`, and never write `current-session.md`,
  `pending-findings.md`, a spec or `.claude/rules/`. Builders cannot write outside their worktree anyway.
- **Never `ExitWorktree` or `EnterWorktree`.** Where a `/work-backlog` file says to leave the worktree to write
  ("Leaving to write"), the write is not yours: findings and rulings go in your result, and the review stage copies
  the plan.
- **After a context compaction**, re-read this file's "Every agent" and your section, and the inputs in your prompt,
  before anything else. Rules such as "never ask", "never fetch `main`" and "never end your turn mid-command" do not
  survive a compaction otherwise.
- **PR bodies change before a push, never after.** CI re-runs on an `edited` event and cancels the run in progress,
  so a body edit right after a push throws away the CI run that push started. Edit the body first, then push.
- **Long commands** (`git push`, whose pre-push hook runs the full gate, `./.ci/in-devshell ./run-tests`, and
  superpowers:executing-plans' `task-done` when it runs them) can outlast the Bash tool's ten-minute limit. Run
  them with `run_in_background`, then wait for them with the Monitor tool until the command exits, and read its
  output. Never end your turn while one is running: your final message is your result, and a result sent early is
  a wrong one. A flow that says "wait a minute and run again" waits the same way: `sleep 60` with
  `run_in_background`, then the Monitor tool, never a foreground `sleep`.
- **Your worktree is the worktree.** superpowers:using-git-worktrees, loaded by executing-plans, finds this one
  already linked; it creates none, runs no project setup and enters nothing (`sub-skill-overrides.md` entries for
  it apply except the creation command, which the workflow did).
- **Output** is the structured object the prompt's schema asks for; `worktree` is always the absolute `pwd` you
  worked in.

## Build

Inputs: `n`, `kind` (`bug` or `feature`), `spec` (repo-relative path, read at `<main>/<spec>`; empty for a bug),
`date`, `main`.

1. **Reconcile.** An earlier agent may have started this issue (the workflow resumes interrupted agents from their
   original prompt, and a resumed run re-runs most agents live).
   - `gh pr list --state all --search "#<n> in:body" --json number,state,headRefName,body`, keeping PRs whose body
     ends with `Backlog-Session: <date>` and has a line that is exactly `Closes #<n>` or `Part of #<n>` (a substring
     match would take a longer number's PR for this one). A `MERGED` one: return `state: "merged"`. An `OPEN` one is a finished
     build, since a PR opens only after step 4's push: return `state: "ready"` at once with `branch` its
     `headRefName`, `pr` its number, `rulings` the body's `Ruling:` lines, no `findings`, and `plan` from the
     `plan:` line of `<main>/.claude/backlog/parallel/<n>.md` (empty when there is none). Do not build again: a
     bug whose fix is already on the branch would no longer reproduce and park as "not a bug".
   - Otherwise a local branch from a dead builder: one matching `*/<n>-*` that `git worktree list --porcelain` shows
     checked out in a `.claude/worktrees/wf_*` or `.claude/worktrees/merge-<n>` worktree. A matching branch anywhere
     else is the user's: leave it alone, and park if its name is the one this issue would take.
   - With a branch found: `git fetch origin <branch>` (when the remote has it), then `git switch --detach` to
     whichever of `<branch>` and `FETCH_HEAD` contains the other (`git merge-base --is-ancestor`); if neither does,
     park, since work exists in two places. A branch checked out in a dead builder's worktree cannot be checked out
     here, hence detached. Find that worktree with `git worktree list`. Copy its plan
     (`docs/superpowers/plans/`) and its ledger (`.superpowers/sdd/<plan-basename>/progress.md`) to the same relative
     paths here, since executing-plans looks for the ledger under the current worktree only and would otherwise
     start over. Work detached from here on and push with `git push origin HEAD:refs/heads/<branch>`. Resume the
     plan from the ledger; work never committed in the dead worktree is lost and is redone. With no ledger, the
     branch's commits beyond `origin/main` say how far it got: a feature whose plan tasks are all committed, or a bug
     with its `fix:` commit, goes straight to step 4.
2. **Fresh start** (nothing found): `gh issue view <n> --json state --jq .state`; `CLOSED` returns
   `state: "skipped"`. `git merge-base --is-ancestor HEAD origin/main` must succeed: the worktree starts from the
   main checkout's `HEAD`, and when that is not on `origin/main` it holds the user's own commits, a batch-level
   failure ("Every agent"). Rename the branch with `git branch -m <type>/<n>-<description>` ("Branches"). Copy
   `<main>/.claude/CLAUDE.md`, `<main>/.claude/rules/` and `<main>/<spec>` to the same relative paths here
   (`mkdir --parents .claude docs/superpowers/specs docs/superpowers/plans` first), and read the copied rules that
   apply to the files you will change. Then run the baseline, `./.ci/in-devshell ./run-tests`, before changing
   anything: red here is judged as "Every agent" says for a red baseline, never fixed as part of this issue.
3. **Build.**
   - **Bug:** `bug-flow.md` steps 2–4. "Not a bug" (step 3) parks, with "close as completed" or "close as not
     planned" as the recommended answer; skip step 3's cleanup (no worktree remove, branch delete or rebase: the
     merge queue and `leftovers.md` own those).
   - **Feature:** `feature-flow.md` step 6.3, planning only this issue's part of the spec, then step 6.4 with
     superpowers:executing-plans (the only executor available: superpowers:subagent-driven-development needs a
     subagent tool this agent lacks). Skip executing-plans' "Final Review"
     entirely and treat the final review as clean: the review stage replaces it, so write no `Final review:` ledger
     line and send no message about it. Then run "Finish" except its workspace deletion, which waits for step 4 (a
     death before the push would otherwise leave a finished branch with no ledger to resume from). Before that, run
     the ledger sweep ("Review" in `/work-backlog`): in-scope deferred findings are fixed, known-gap-commented or
     dropped, out-of-scope notes go in `findings`, and every `Ruling:` goes in `rulings`. Where the merged code of
     an earlier piece contradicts the spec, the code wins and the divergence is a ruling.
4. **Push and open the PR.** `git push --set-upstream origin <branch>` (or the detached form from step 1). A
   pre-push gate failure is fixed and pushed again, as a failing test would be. Then `gh pr create` per "PRs" and
   "PR title" in `/work-backlog`: `Closes #<n>` or `Part of #<n>`, the rulings, ending with
   `Backlog-Session: <date>`. Once the PR exists, delete the plan's workspace (`.superpowers/sdd/<plan-basename>/`).
5. **Return** `state: "ready"`, `branch`, `pr`, `plan` (the plan's repo-relative path, `docs/superpowers/plans/…`,
   or empty), `worktree`, `rulings`, `findings`.

## Review

Inputs: `n`, `round` (1 or 2), `branch`, `pr`, `headWorktree` (where the latest commit was made), `plan` (the build
agent's plan path), `worktrees` (every worktree path the issue's agents used so far), `build` (the latest build or
fix agent's output, verbatim), `main`.

1. **Record.** Write `<main>/.claude/backlog/parallel/<n>.md` ("State" in `SKILL.md`) from the inputs: `state:
   ready`, `branch:`, `pr:`, `plan:`, `worktrees:`, and every ruling already in the file plus `build.rulings` not
   already there. Append each entry of `build.findings` not already in
   `<main>/.claude/backlog/parallel/<n>-findings.md` to it (a re-run review must not add them twice). The plan comes
   from the `plan` input, never from `build`, whose `plan` a fix agent leaves empty: when it is set and
   `<main>/<plan>` does not exist yet, copy `<headWorktree>/<plan>` there, then write `plan: <main>/<plan>`, a path
   that outlives the worktree.
2. **Review** the pushed branch as superpowers:requesting-code-review's `code-reviewer.md` describes, with the
   "Subagents" clause. In the main checkout, `git fetch origin <branch>` (your issue's branch, which "Shared state"
   allows), then base `git merge-base origin/main origin/<branch>`, head `origin/<branch>`, inspected with
   `git diff` and `git show` only. The pushed branch, not `headWorktree`, is what merges, and a writer's worktree
   may already be gone. The requirements are the issue (`gh issue view <n>`) and, for a feature, the spec and plan.
   Edit no file in any issue worktree.
3. **Sort** each finding: about the change itself (a hole in this PR's code or tests) goes in `changeFindings`;
   anything else is appended to `<n>-findings.md` and listed in `otherFindings`. Each "Declined to judge" line
   `code-reviewer.md` produces goes in `changeFindings` with its summary starting `Declined to judge:`, so the fix
   agent rules on it. The pre-push gate already ran the tests; the review does not rerun them.
4. **Return** `changeFindings`, `otherFindings`, `recordedWriterFindings` (true once step 1 has appended
   `build.findings`) and `batchHalt` (empty unless a batch-level failure stopped the review). When a halt stops the
   review, `otherFindings` holds only the out-of-scope findings not yet appended to `<n>-findings.md`; the
   controller writes those, and `build.findings` too when `recordedWriterFindings` is false.

**A conflict resolution** (inputs `resolution: true`, `mergeWorktree`, `conflictedFiles`, `branch`, `pr`, `n`,
`main`, from `merge-queue.md`): skip step 1 and return `recordedWriterFindings: true`. Review only how the
conflicts in `conflictedFiles` were resolved:
`git -C <mergeWorktree> diff origin/main HEAD -- <files>` against what each side's PR meant
(`gh pr view <pr>` and the `main` commits that touched those files). A wrong or lossy resolution is a
`changeFindings` entry; a finding outside the resolution follows step 3 and is not this review's business beyond
that.

## Fix

Inputs: `n`, `round`, `branch`, `pr`, `spec` (repo-relative, empty for a bug), `changeFindings`, `date`, `main`.

1. **Reconcile and check out:** `git fetch origin <branch>`, then `git switch --detach FETCH_HEAD`. If a commit on
   the branch already says it addresses this round's findings (a resumed fix), continue from what remains. Copy
   `<main>/.claude/CLAUDE.md`, `<main>/.claude/rules/` and, for a feature, `<main>/<spec>` in, as Build step 2
   does.
2. **Handle each finding** as "Findings about the change itself" says: fixed (test first, then fix, as separate
   commits where a test applies), given a known-gap comment, or dropped with a ruling. A `Declined to judge:` entry
   always gets a ruling: the behavior stands, or it is now a finding and handled as one. Three failed attempts at one
   finding park the issue (`sub-skill-overrides.md`, the systematic-debugging entry).
3. **Push:** first add any new rulings to the PR body with `gh pr edit <pr> --body` ("PR bodies change before a
   push"), then `git push origin HEAD:refs/heads/<branch>` (the pre-push gate runs).
4. **Return** `state: "ready"`, `worktree`, `rulings`, `findings`.
