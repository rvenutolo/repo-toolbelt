# Feature flow

The user takes part in the brainstorm and approves the design there, then answers one batched round of questions.
Everything after that runs unattended under "How the session runs" in `SKILL.md`. A call made before approval (steps
3 and 4) is recorded in the spec; a call made during the unattended part is a `Ruling:` ("Terms" in `SKILL.md`).

Steps 1 to 5 run in the main checkout on `main`; no worktree exists yet. The worktree is made per issue in step 6.

1. **Brainstorm.** **REQUIRED SUB-SKILL:** superpowers:brainstorming, with the user. Open it with the current state:
   the chunk's issue bodies and comments, and the settled decisions in `.claude/CLAUDE.md`, so its questions start
   from what is already decided instead of re-asking it. When the chunk proposal named an earlier session's spec
   (merged `Part of #N` pieces, "Earlier pieces" in `SKILL.md`), open with that spec and extend it: its merged parts
   are settled, and the code that landed is their design. A family of features shares one brainstorm. Brainstorming
   ends when the user approves the design in the conversation; if the user opts out of its questions, the design is
   what the conversation already holds. If the brainstorm drops an issue from the chunk or adds one (an open issue
   that is not the user's own work), the spec's issue list is the chunk from then on; name the change in the summary.
   Either way a spec is written, and the builder check's questions wait for step 5 (`sub-skill-overrides.md` names
   the brainstorming pauses these steps replace).
2. **Spec.** Write it to `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`, a local path ("State" in
   `SKILL.md`), and record the path in `current-session.md`. For a family, one spec covers every issue and says
   which issue owns which part.
3. **Builder check.** Run the brainstorming skill's builder check on the spec. It looks for gaps: questions a builder
   would still need answered. Fill the ones a sensible builder could settle without changing what gets built, and
   hold the rest for step 5.
4. **Fable review.** Dispatch a subagent with `model: fable` to review the completed spec for soundness: wrong calls,
   conflicts with the settled decisions in `.claude/CLAUDE.md`, risks, and anything in the spec that would build
   speculative work, such as a check, guard or test for an input nobody writes. This is the one check on the design
   before hours of unattended building, so it gets the most capable model: a wrong call it misses is built, merged
   and found by the user. The reviewer returns every finding and filters none, as "Subagents" in `SKILL.md` says;
   design findings are judged on soundness ("Findings about the change itself" in `SKILL.md`). Apply what does not
   change what gets built; hold the rest for step 5.
5. **One question round.** A single message, beginning with the question banner, that asks:
   - everything held from steps 3 and 4;
   - every likely halt question for the unattended part ("Terms" in `SKILL.md`; `halt-questions.md` lists the kinds
     that recur), each with a recommended answer and a one-line reason;
   - any copied rule in `.claude/rules/` the planned code could not satisfy (the repo's `CLAUDE.md` asks the user
     about those);
   - the permission request (`permissions.md`), with any tool the spec's work needs beyond the devShell's added to
     its list.

   The message ends with a one-line list of the calls made in steps 3 and 4, which the user may scan but need not
   answer. Write the answers into the spec. If the user defers ("do whatever you think"), every recommended answer
   is the answer; write each into the spec as a call. A deferral answers no permission request (`permissions.md`
   "When to ask"). A spec that has passed steps 3–5 counts as approved; the user does not review the written
   document. Then append to `current-session.md`: `spec approved: <path>`, the commands the user allowed, the halt
   answers, and the `started:` line ("Budgets" in `SKILL.md`), since the unattended part begins here, so a session
   resumed from that file knows the brainstorm and this round are done.

   A rule fix or override the user chose in this round is written in the main checkout's `.claude/rules/` and copied
   into every worktree the flow creates afterwards. A rule conflict that only surfaces later is settled as "Copied
   rules" in `SKILL.md` says.
6. **Per issue, in dependency order** (for an umbrella issue, per piece, each piece with its own worktree, plan and
   PR):
   1. Check the issue is still open, as the loop in "Doing the work" in `SKILL.md` says.
   2. Create and enter a worktree, copy the instruction files and the spec into it, and run the baseline tests
      ("Worktrees" in `SKILL.md`).
   3. **Plan.** **REQUIRED SUB-SKILL:** superpowers:writing-plans, for this issue's part of the spec. From the second
      piece on, read the merged code of the earlier pieces first: the spec was written before any of it existed, and
      where it describes a helper or interface differently from what landed, the code wins and the spec is corrected
      in the main checkout and copied again. The plan is written to `docs/superpowers/plans/`, a local path: write it
      in the worktree, then, in one round trip out of it ("Leaving to write" in `SKILL.md`, which a spec correction
      shares), copy it to the main checkout so it survives the worktree's removal and record its path against the
      issue in `current-session.md`.
   4. **Execute** in whatever way fits the plan best: superpowers:subagent-driven-development,
      superpowers:executing-plans, or another approach. Choose without asking; the plan header's required-sub-skill
      line names the usual two and does not bind the choice.
   5. **PR**, reviewed and merged on green CI as `SKILL.md` describes. The next issue's worktree starts from the
      updated `origin/main`.

Then return to `SKILL.md` step 8 for the findings walkthrough, which needs the user.
