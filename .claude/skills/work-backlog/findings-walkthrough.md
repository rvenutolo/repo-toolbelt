# Findings walkthrough

Go through `pending-findings.md` one finding at a time, and never skip one. Never present them as a list: the user
decides each on its own, and a list invites a blanket answer that files or drops findings nobody weighed. There is
no cap on how many there are.

For each finding:

1. **Re-check it on current `main`.** A merge since it was found may have fixed it. If it no longer reproduces, say so
   and recommend dropping it.
2. **Explain it** for a reader who does not know the context: what the code does, what goes wrong, the realistic input
   that triggers it, and who would hit it.
3. **Offer options**, typically:
   - open an issue, labelled `bug` (so it leads the next session) or `enhancement`
   - fix it now, in this session
   - leave a one-line known-gap comment beside the code
   - drop it
4. **Recommend one**, with a one-line reason.
5. **Wait for the user's decision.** Ask with the question banner, one finding per question.
6. **Carry it out**: open the issue, make the fix, or add the comment. The user's "fix it now" is the approval for
   that fix's branch, push and merge: it follows `bug-flow.md` (worktree, test, fix, review) and "Merging" in
   `SKILL.md`, after the permission request ("Permissions" in `SKILL.md`) if this session has not made it yet. A fix
   made now has no issue: its branch is `fix/finding-<slug>` ("Branches" in `SKILL.md`), its PR body describes the
   finding instead of using a closing keyword, and still ends with the marker. Write `current-session.md` as soon as
   the worktree exists ("State" in `SKILL.md`); step 6 of the session adds the chunk to it later. A known-gap comment
   also goes through a PR of its own, since `main` is protected and nothing lands on it without one.
7. **Remove the entry** from `pending-findings.md` only after the decision is carried out.
