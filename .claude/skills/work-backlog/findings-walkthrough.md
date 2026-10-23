# Findings walkthrough

Go through `pending-findings.md` one finding at a time. Never present them as a list, and never skip one. There is no
cap on how many there are.

For each finding:

1. **Re-check it on current `main`.** A merge since it was found may have fixed it. If it no longer reproduces, say so
   and recommend dropping it.
2. **Explain it** for a reader who does not know the context: what the code does, what goes wrong, the realistic input
   that triggers it, and who would hit it.
3. **Offer options**, typically:
   - open an issue (a bug gets the `bug` label, so it leads the next session)
   - fix it now, in this session
   - leave a one-line known-gap comment beside the code
   - drop it
4. **Recommend one**, with a one-line reason.
5. **Wait for the user's decision.** Ask with the question banner, one finding per question.
6. **Carry it out**: open the issue, make the fix through the normal PR flow, or add the comment.
7. **Remove the entry** from `pending-findings.md` only after the decision is carried out.
