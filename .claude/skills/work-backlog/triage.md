# Triage

Runs only when the backlog has stalled (`SKILL.md` step 5) and the user replied `triage` to the proposal ("The
proposal" in `choosing.md`; step 5 says why it is never run unasked). It never reads every open issue. Judge:

- **Bugs**, every open one, against the filing bar in `SKILL.md`.
- **Features** only when something marks one as possibly no longer wanted: its `refs` or `umbrella` row in the index
  ("The index" in `choosing.md`) points at a closed issue, or its body conflicts with a settled decision in
  `.claude/CLAUDE.md`. Judge those on whether they are still wanted: not superseded, not a duplicate, not in conflict
  with those decisions. A feature with no such mark is left alone.

Before judging a bug, re-check it on current `main`: a merge may already have fixed it. A bug that no longer
reproduces goes to the user with "close as completed" as the recommendation.

Each issue that fails its test, or no longer reproduces, goes through the same one-by-one walkthrough as a finding
(`findings-walkthrough.md` steps 2–5), with "close as not planned" or "close as completed" among the options. Nothing
closes without the user's decision. Close with `gh issue close <number> --reason "not planned"` or
`--reason completed`, with `--comment "<the user's reason>"`.
