# Feature flow

The user takes part in the brainstorm and approves the design there, then answers one batched round of questions.
Everything after that runs unattended unless a decision needs the user: one that changes what gets built. Claude
settles everything else and records each call in the spec.

1. **Brainstorm.** **REQUIRED SUB-SKILL:** superpowers:brainstorming, with the user. A family of features shares one
   brainstorm. Brainstorming ends when the user approves the design in the conversation; if the user opts out of its
   questions, the design is what the conversation already holds. Either way a spec is written: steps 2–5 replace the
   brainstorming skill's hand-over, its builder-check round with the user and its hard gate, and the user has
   authorized building once those steps pass.
2. **Spec.** Write it to `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`. The path is gitignored, so the spec
   stays local and is never force-added. For a family, one spec covers every issue and says which issue owns which
   part.
3. **Builder check.** Run the brainstorming skill's builder check on the spec. It looks for gaps: questions a builder
   would still need answered. Fill the ones a sensible builder could settle without changing what gets built, and
   hold the rest for step 5.
4. **Fable review.** Dispatch a subagent with `model: fable` to review the completed spec for soundness: wrong calls,
   conflicts with the settled decisions in `.claude/CLAUDE.md`, risks, and anything that slips past the filing bar.
   Apply what does not change what gets built; hold the rest for step 5.
5. **One question round.** In a single message, ask everything held from steps 3 and 4, every likely halt question
   for the unattended part (each with a recommended answer and a one-line reason, as the global "Unattended
   Execution" rule requires), any copied rule in `.claude/rules/` the planned code could not satisfy (the repo's
   `CLAUDE.md` asks the user about those), and approval for every tool and permission the rest of the flow will
   need (as the global "Long Multi-Step Tasks" rule requires). Write the answers into the spec. A spec that has
   passed steps 3–5 counts as approved; the user does not review the written document. A rule conflict that only
   surfaces during the unattended part is settled by satisfying the rule, and reported in the session summary.
6. **Per issue, in dependency order:**
   1. Create and enter a worktree, and copy the instruction files and the spec into it ("Worktrees" in `SKILL.md`).
   2. **Plan.** **REQUIRED SUB-SKILL:** superpowers:writing-plans, for this issue's part of the spec. The plan is
      written to `docs/superpowers/plans/`, which is gitignored: write it in the worktree and copy it to the main
      checkout, so it survives the worktree's removal.
   3. **Execute** in whatever way fits the plan best: superpowers:subagent-driven-development,
      superpowers:executing-plans, or another approach. Choose without asking.
   4. **PR**, reviewed and merged on green CI as `SKILL.md` describes. The next issue's worktree starts from the
      updated `main`.

Then return to `SKILL.md` step 8 for the findings walkthrough, which needs the user.
