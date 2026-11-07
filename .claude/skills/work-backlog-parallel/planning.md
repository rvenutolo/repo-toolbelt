# Planning the batch

Step 2 of `SKILL.md` "The session". Everything here happens with the user, in the main checkout, before the
workflow starts. It is `/work-backlog`'s `choosing.md` and `feature-flow.md` steps 1–5, with the exceptions below.

## Choosing the batch

`/work-backlog` `choosing.md` holds, minus what `SKILL.md` "Named exceptions" replaces (size, the bugs-only
or one-family rule, every piece of an umbrella issue). What is left to say here is how to propose a size: ask
nothing about it up front; propose the size the `short` or `long` argument named (`SKILL.md` "Arguments"), or the
session shape the user asked for, and when they named none, propose a short one and name the long batch the backlog
would support. Issues the user named as arguments are the batch, whatever their size. While the shakedown is on
(`SKILL.md` "Shakedown and budgets"), the batch is one issue, chosen as `/work-backlog` "Shakedown" says, with the
short batch standing in for the chunk.

- **Short:** the doable bugs, or one feature family when there are none, about two hours by the estimate below.
- **Long:** the doable bugs plus as many feature families as fit an unattended run of about eight hours by that
  estimate. Every family adds a brainstorm and a spec to the sitting, so name the families and let the user drop
  some.

The runner-up `/work-backlog`'s proposal names is the other size here (the long batch beside a short proposal, the
short one beside a long proposal), or "none" when the backlog holds nothing more; while the shakedown is on, it is
the short batch the one issue was taken from.

An umbrella issue's one piece is the next unbuilt piece its spec names, recorded as the issue's `piece`. Each
issue's `kind` in `run-args.json` (`bug` or `feature`) is the kind "Kinds" in `choosing.md` decides.

## The dependency graph

The batch is a graph, not a list. Issue B depends on issue A, written `issue B after A`, when:

- B builds on code A adds (a family piece, an umbrella piece, a lint that uses A's helper);
- A and B both need a helper, runner or template the spec names: the issue that owns it in the spec goes first, the
  rest depend on it;
- A and B are bugs with the same root cause, so B's fix would rebase onto A's change in the same lines;
- A and B change the same lines of a file that is not a registry or doc file listed below (the same step of a
  workflow, the same function), so the second would meet the first as a code conflict. An issue that asks to land
  with another is handled as "Combined changes" in `choosing.md` says; the edge is the order it names.

Registry and doc files every new lint touches (`lints.toml`, `formatters.toml`, README lists and tables,
`examples/consumer/` lint lists, `.coverage-baseline.txt`) are not a dependency: the merge queue resolves conflicts
in them (`merge-queue.md`). README prose follows the code-conflict rule. Any
other overlap the graph misses meets the merge queue as a conflict; one between two of the batch's own PRs is
resolved, and one with a change from outside the batch parks the issue (`SKILL.md` "Named exceptions").

An issue with no edge runs as soon as a slot is free. An `after` list names only issues in the batch: a
prerequisite outside it is either merged already (drop the edge) or not, and then the dependent stays out of the
batch too.

## The proposal

One message, beginning with the question banner, naming everything `/work-backlog`'s proposal names, plus:

- the graph, as the `issue <N> after <M>` lines `current-session.md` will hold, with each umbrella issue's piece,
  in the order "Order" in `choosing.md` gives (by leverage, not bugs before features);
- **the cap**: how many build and fix agents run at once (default 4, and 1 during the shakedown; each runs the full
  pre-push gate when it pushes); the one merge agent runs beside them, so at most cap + 1 gates run;
- **the budgets** and the shakedown as `/work-backlog` "Budgets" and "Shakedown" give them, with what each does to a
  run that reaches it (`SKILL.md` "Shakedown and budgets"); state `max-hours`, not a clock time, since the deadline is
  computed at launch, after the sitting. While the shakedown is on, say that the merge OK is asked at "Ending", once
  the run completes, in place of the fifteen-minute line `/work-backlog`'s proposal carries (`SKILL.md` "Named
  exceptions");
- **a wall-clock estimate**, as a range, not a promise. Per issue, guess its work from the issue and spec: a small
  bug about 30 minutes to a ready PR, a lint or workflow feature one to two hours, plus about 15 minutes for review
  and fixes. A merge round is about 15 minutes (rebase, pre-push gate, CI of about five minutes), and merges never
  overlap. A merge that meets a code conflict or a baseline regeneration takes 30 to 40 minutes (a second test run,
  a resolution review, a second gate), and every new lint after the first in a batch meets one in the shared
  registry files. A dependency chain runs end to end, each link waiting for the previous one's merge. The run takes
  about the longer of the longest chain and the first ready PR plus one merge round per issue; a fix takes a slot
  ahead of builds still queued when the fix asks (not ahead of one already running), and the merge agent runs
  outside the cap, so merging begins as soon as a review passes, whatever the builds are doing. Say plainly that a
  short
  batch of small bugs saves little over `/work-backlog`, and recommend it instead when the estimate shows that; an
  estimate longer than the time budget names the issues that would likely skip;
- **the agent count**, from these bounds: per issue at most ten agents (one build, two reviews, two fixes, three
  merge calls and two resolution reviews between them), each retried once if it dies, plus a resumed run's live
  re-runs; most issues need three (build, review, merge). Name the session's workflow size guideline (the Workflow
  tool's description states it in one of two forms: "This session has the … workflow size guideline" by default, or
  "A workflow size guideline is configured for this session: …") and say whether the estimate exceeds it, adding:
  "This is a guideline for whoever writes the script, not a limit; nothing needs changing for the run to proceed."
  Agents running at once are the build and fix agents up to the cap, plus every review in progress (reviews run
  outside the cap) and one merge agent, so a long batch at cap 4 can approach the hard limit of concurrent agents,
  min(16, max(2, CPUs − 2)); name that limit, and the 1000 agents per run, only when the batch would reach them.

For a bug batch, the halt questions and the permission request ride on the proposal, as in `/work-backlog`; the
permission request is left out when the session runs with permissions bypassed. Ask every likely halt question, as
`/work-backlog` does, working through `/work-backlog` `halt-questions.md` for each issue: a halt parks the issue here
instead of stopping the session, but a park still waits for a later session to ask the same question.

## Feature batches

1. **Brainstorm per family** (`feature-flow.md` step 1). A batch with several unrelated features or families has one
   brainstorm and one spec each, in proposal order, one after another in this sitting.
2. **Spec, builder check, Fable review** per spec (`feature-flow.md` steps 2–4).
3. **One question round** for the whole batch (`feature-flow.md` step 5): every held question from every spec, the
   halt questions (`halt-questions.md` again, for each issue), rule conflicts and the permission request (left out
   when bypassed), in one message.

## The permission request

`/work-backlog` `permissions.md` holds the list: its "Rule strings" and "Workflow agents add" both apply here. A
missing rule costs more than it does serially: an agent that meets a permission prompt waits for an answer nobody
gives, holding its slot and stalling its issue and every issue that depends on it. The list is long and one miss
does that, so recommend running the session with permissions bypassed; `SKILL.md` "The session" step 1 asks about
it before planning (a session tells its mode as `permissions.md` says, by the system-prompt paragraph), and "The
run" step 2 checks again before launch.

## Writing it down

At the batch OK, write `current-session.md` as `/work-backlog` step 6 does, minus its `started:` line (`SKILL.md`
"Shakedown and budgets"): a feature batch's brainstorms and question round follow, and `feature-flow.md` steps 2 and 5
record each spec path and the round's answers in that file, as does any halt in between ("Halting" in `resume.md`).
After the answered question round (or at once, for a bug batch), add `cap:`, the `issue` and `piece` lines
(`SKILL.md` "State") and each halt answer with the issues it bears on (they become each issue's `haltAnswers`); their
presence is what "On invocation" reads as a batch approved for launch. Then go to "The run".
