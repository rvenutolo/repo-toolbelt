# Triage

Runs only when the backlog has stalled (`SKILL.md` step 5). Judge every open issue by its kind:

- **Bugs** against the filing bar in `SKILL.md`.
- **Features** only on whether they are still wanted: not superseded, not a duplicate, and not in conflict with the
  settled decisions in `.claude/CLAUDE.md`.

Before judging a bug, re-check it on current `main`: a merge may already have fixed it. A bug that no longer
reproduces goes to the user with "close as completed" as the recommendation.

Each issue that fails its test, or no longer reproduces, goes through the same one-by-one walkthrough as a finding
(`findings-walkthrough.md` steps 2–5), with "close as not planned" or "close as completed" among the options. Nothing
closes without the user's decision. Close with `gh issue close <number> --reason "not planned"` or
`--reason completed`, with `--comment "<the user's reason>"`.
