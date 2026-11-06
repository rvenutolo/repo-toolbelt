# Planning the batch

Step 2 of `SKILL.md` "The session". Everything here happens with the user, in the main checkout, before the
workflow starts. It is `/work-backlog`'s "Choosing the chunk" and `feature-flow.md` steps 1–5, with the exceptions
below.

## Choosing the batch

`/work-backlog` "Choosing the chunk" holds, minus what `SKILL.md` "Named exceptions" replaces (size, the bugs-only
or one-family rule, every piece of an umbrella issue). What is left to say here is how to propose a size: ask
nothing about it up front; propose a batch that fits the session shape the user asked for (a short session or a long
unattended one), and when they named none, propose a short one and name the longer batch the backlog would support.

An umbrella issue's one piece is the next unbuilt piece its spec names, recorded as the issue's `piece`. Each
issue's `kind` in `run-args.json` (`bug` or `feature`) is the kind "Choosing the chunk" decides.

## The dependency graph

The batch is a graph, not a list. Issue B depends on issue A, written `issue B after A`, when:

- B builds on code A adds (a family piece, an umbrella piece, a lint that uses A's helper);
- A and B both need a helper, runner or template the spec names: the issue that owns it in the spec goes first, the
  rest depend on it;
- A and B are bugs with the same root cause, so B's fix would rebase onto A's change in the same lines;
- A and B change the same lines of a file that is not a registry or doc file listed below (the same step of a
  workflow, the same function), so the second would meet the first as a code conflict. When an issue's comment
  says the two belong in one change, say so in the proposal: one PR per issue still holds, so the edge orders them
  and the second PR builds on the first.

Registry and doc files every new lint touches (`lints.toml`, `formatters.toml`, README sections, `examples/consumer/`,
`.coverage-baseline.txt`) are not a dependency: the merge queue resolves conflicts in them (`merge-queue.md`). Any
other overlap the graph misses meets the merge queue as a conflict; one between two of the batch's own PRs is
resolved, and one with a change from outside the batch parks the issue (`SKILL.md` "Named exceptions").

An issue with no edge runs as soon as a slot is free. An `after` list names only issues in the batch: a
prerequisite outside it is either merged already (drop the edge) or not, and then the dependent stays out of the
batch too.

## The proposal

One message, ending with the question banner, naming everything `/work-backlog`'s proposal names, plus:

- the graph, as the `issue <N> after <M>` lines `current-session.md` will hold, with each umbrella issue's piece;
- **the cap**: how many build, fix and merge agents run at once (default 4; each runs the full pre-push gate when it
  pushes);
- **a wall-clock estimate**, as a range, not a promise. Per issue, guess its work from the issue and spec: a small
  bug about 30 minutes to a ready PR, a lint or workflow feature one to two hours, plus about 15 minutes for review
  and fixes. A merge round is about 15 minutes (rebase, pre-push gate, CI of about five minutes), and merges never
  overlap. A merge that meets a code conflict or a baseline regeneration takes 30 to 40 minutes (a second test run,
  a resolution review, a second gate), and every new lint after the first in a batch meets one in the shared
  registry files. A dependency chain runs end to end, each link waiting for the previous one's merge. The run takes
  about the longer of the longest chain and the first ready PR plus one merge round per issue; merges and fixes take
  a free slot ahead of builds not yet started, so merging begins while later builds wait. Say plainly that a short
  batch of small bugs saves little over `/work-backlog`;
- **the agent count**, worked out from `run.workflow.js`'s bounds: per issue, one build, up to two review/fix rounds,
  and up to three merge calls with a resolution review between each; a dead agent's one retry and a resumed run's
  live re-runs come on top, and most issues need far fewer than the bound. Name the session's workflow size
  guideline (the Workflow tool's description states it as "This session has the … workflow size guideline") and say
  whether the estimate exceeds it, adding: "This is a guideline for whoever writes the script, not a limit; nothing
  needs changing for the run to proceed." Name the hard limits (16 concurrent agents, 1000 per run) only when the
  batch would reach them.

For a bug batch, the halt questions and the permission request ride on the proposal, as in `/work-backlog`; the
permission request is left out when the session runs with permissions bypassed. Halt questions are fewer here:
every per-issue halt parks the issue (`SKILL.md` "Named exceptions"), so ask only what would otherwise park an issue
that a one-word answer now could keep going.

## Feature batches

1. **Brainstorm per family** (`feature-flow.md` step 1). A batch with several unrelated features or families has one
   brainstorm and one spec each, in proposal order, one after another in this sitting.
2. **Spec, builder check, Fable review** per spec (`feature-flow.md` steps 2–4).
3. **One question round** for the whole batch (`feature-flow.md` step 5): every held question from every spec, the
   halt questions, rule conflicts and the permission request (left out when bypassed), in one message.

## The permission request

`/work-backlog` `permissions.md` holds the list: its "Rule strings" and "Workflow agents add" both apply here. A
missing rule costs more than it does serially: an agent that meets a permission prompt waits for an answer nobody
gives, holding its slot and stalling its issue and every issue that depends on it. The list is long and one miss
does that, so recommend running the session with permissions bypassed; `SKILL.md` "The session" step 1 asks about
it before planning, and "The run" step 2 checks again before launch.

## Writing it down

After the OK (and, for features, the answered question round), write `current-session.md` as `/work-backlog` step 6
does, plus `cap:`, the `issue` and `piece` lines (`SKILL.md` "State") and each halt answer with the issues it bears
on (they become each issue's `haltAnswers`). Then go to "The run".
