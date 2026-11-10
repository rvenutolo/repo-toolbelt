# Halt questions that recur

The proposal (bugs) or the question round (features, `feature-flow.md` step 5) asks every likely halt question in
advance ("Terms" in `SKILL.md`). These kinds come up again and again; check each issue in the chunk against them, and
ask the ones that apply with the default below as the recommendation unless the issue gives a reason for another.
Each default is the answer that never needs undoing.

- **No possible local test.** A change only CI or a config file exercises (a workflow, a Renovate setting). Default:
  record a `Ruling:` that the workflow lints and the PR's own CI run are the test, since a test written to exercise a
  YAML key tests the parser, not the change.
- **"Land together".** The issue asks for its change to ship with another issue's. Default: one PR each, ordered by
  the dependency ("Combined changes" in `choosing.md`).
- **Title broader than body.** The title names a problem neither the body nor a comment describes. Default: try to
  reproduce it; fix it in the same PR if it reproduces, otherwise a `Ruling:` that it did not.
- **Prerequisite outside the repo.** The change needs a GitHub App setting, a secret, another repo's change or a
  release. Default: leave the issue out and name what the user must do (the proposal's "cannot do at all" item).
- **An untracked doc goes stale.** The change makes `.claude/CLAUDE.md` wrong (a lint, formatter, command or layout it
  describes). Default: update it in the main checkout in the same issue's work ("Worktrees" in `SKILL.md`).
- **Reference implementation elsewhere.** The issue points at code in another repo to copy or port. Default: port
  the behavior, not the code, to this repo's conventions, and record where it differs as a `Ruling:`.
- **Umbrella scope.** The issue asks for sub-issues to be filed and worked. Default: one PR per piece, no sub-issues
  opened, unless the user opts in.
- **A copied rule the planned code cannot satisfy.** Default: satisfy the rule ("Copied rules" in `SKILL.md`); ask
  only when satisfying it changes what gets built.
- **A fix makes nearby code dead.** The change leaves code the issue does not mention with nothing left to do (a
  pass-through, a fallback, a branch no input reaches). Default: leave it and record it as an out-of-scope finding, so
  the PR stays the change the issue asked for and the user decides the removal in the walkthrough.
- **A lint change that fails consumers.** A changed lint would fail a repo that uses this one. Default: name the
  consumers in the PR body and the summary, so they are fixed before the release reaches them through Renovate.
