# Feature flow

The user takes part in the brainstorm and approves the design there. Everything after that runs unattended unless a
decision needs the user: one that changes what gets built. Claude settles everything else and records each call in the
spec.

1. **Brainstorm.** **REQUIRED SUB-SKILL:** superpowers:brainstorming, with the user. It may decide that no written
   design is needed; the remaining steps still apply, scaled down. A family of features shares one brainstorm.
   Brainstorming ends when the user approves the design in the conversation. Steps 2–5 below replace its written
   design, its builder-check round with the user, its hand-over and its hard gate; the user has authorized building
   once those steps pass.
2. **Halt questions.** Before going unattended, ask every likely halt question in one batched message, each with a
   recommended answer and a one-line reason, as the global "Unattended Execution" rule requires. In the same message,
   list every tool and permission the rest of the flow will need and ask the user to approve them all at once, as the
   global "Long Multi-Step Tasks" rule requires. Write the answers into the spec.
3. **Spec.** Write it to `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`.
4. **Builder check.** Run the brainstorming skill's builder check on the spec. It looks for gaps: questions a builder
   would still need answered. Fill them.
5. **Fable review.** Dispatch a subagent with `model: fable` to review the completed spec for soundness: wrong calls,
   conflicts with the settled decisions in `.claude/CLAUDE.md`, risks, and anything that slips past the filing bar.
   Apply its findings.

   In steps 4 and 5, a finding that changes what gets built goes to the user; settle the rest. A spec that has passed
   both reviews counts as approved; the user does not review the written document.
6. **Plan.** **REQUIRED SUB-SKILL:** superpowers:writing-plans.
7. **Execute** in whatever way fits the plan best: superpowers:subagent-driven-development,
   superpowers:executing-plans, or another approach. Choose without asking.
8. **PRs.** One per issue, reviewed and merged on green CI as `SKILL.md` describes.

Then return to `SKILL.md` step 8 for the findings walkthrough, which needs the user.
