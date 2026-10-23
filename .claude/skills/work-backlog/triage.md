# Triage

Runs only when the backlog has stalled (`SKILL.md` step 5). Judge every open issue by its kind:

- **Bugs** against the filing bar in `SKILL.md`.
- **Features** only on whether they are still wanted: not superseded, not a duplicate, and not in conflict with the
  settled decisions in `.claude/CLAUDE.md`.

Each issue that fails its test goes through the same one-by-one walkthrough as a finding (`findings-walkthrough.md`
steps 2–5), with "close as not planned" among the options. Nothing closes without the user's decision. Close with
`gh issue close <number> --reason "not planned" --comment "<the user's reason>"`.
