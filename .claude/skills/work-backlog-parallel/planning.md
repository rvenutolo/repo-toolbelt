# Planning the batch

Step 2 of `SKILL.md` "The session". Everything here happens with the user, in the main checkout, before the
workflow starts. It is `/work-backlog`'s `choosing.md` and `feature-flow.md` steps 1–5, with the exceptions below.

## Choosing the batch

`/work-backlog` `choosing.md` holds, minus what `SKILL.md` "Named exceptions to `/work-backlog`" replaces (size, the
bugs-only or one-family rule, every piece of an umbrella issue). What is left to say here is how to propose a size: ask
nothing about it up front; propose the size the `short` or `long` argument named (`SKILL.md` "Arguments"), or the
session shape the user asked for, and when they named none, propose a short one and name the long batch the backlog
would support. Issues the user named as arguments are the batch, whatever their size. While this skill's shakedown
(`shakedown-parallel:`, `SKILL.md` "Shakedown and budgets") is on, the batch is two issues taken from the short batch
with no edge between them ("The dependency graph"), so both build at once and the second meets the merge queue's
rebase: the first two independent doable bugs in its order, or, with fewer than two, the two smallest independent
issues it holds. With named issues, the first two independent named bugs, or named issues when fewer than two are
bugs. When no two issues are independent, the batch is one issue, chosen as `/work-backlog` "Shakedown" says, and the
proposal says the merge queue's rebase goes untried this time.

- **Short:** the doable bugs, in order, up to about two hours by the estimate below (every one when they fit), or one
  feature family when there are none. Bugs past the two hours go to the long batch.
- **Long:** the doable bugs plus as many feature families as fit an unattended run of about eight hours by that
  estimate. Every family adds a brainstorm and a spec to the sitting, so name the families and let the user drop
  some.

The runner-up `/work-backlog`'s proposal names is the other size here (the long batch beside a short proposal, the short
one beside a long proposal), or "none" when the backlog holds nothing more; while the shakedown is on, it is the rest of
the short batch its issues were taken from, then the long batch, both named. The second half of `choosing.md`'s
`Runner-up` holds as it is: the issues the session cannot do at all, each with what the user would have to do.

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
resolved, and one with a change from outside the batch parks the issue (`common.md` "Exceptions to `/work-backlog`").

An issue with no edge runs as soon as a slot is free. An `after` list names only issues in the batch: a
prerequisite outside it is either merged already (drop the edge) or not, and then the dependent stays out of the
batch too.

## The proposal

One message, beginning with the question banner, laid out as "The proposal" in `choosing.md` fixes it: the same
section labels in the same order, so the user reads both skills' proposals the same way. The `Reply` line is the
serial one with two changes, both named exceptions in `SKILL.md`: it leaves out every permission form, since this
skill runs only bypassed, and it adds `OK, short` and `OK, long` to take a batch size without a second proposal,
except while the shakedown is on, when the batch is its two issues whatever the size. `What happens` (`choosing.md`)
adds the workflow's lines: the agents build and review the issues concurrently and merge them one at a time with
nobody watching, the user can watch the run in `/workflows` as well as `backlog-status`, and the stop file reaches the
agents only between steps (`SKILL.md` "Shakedown and budgets"). While the shakedown is on it also says nothing merges
during the run: both merge questions wait for "Ending", after the run completes. Four sections are inserted after
`Chunk`, before `What happens`, in this order:

- `Graph`: the `issue <N> after <M>` lines `current-session.md` will hold, with each umbrella issue's piece, bugs
  ahead of features where the graph allows and each kind in the order "Order" in `choosing.md` gives
  (`SKILL.md` "Named exceptions to `/work-backlog`"). While the shakedown is on, a second part headed `after
  shakedown:` gives the graph the whole short batch would get, so the user sees what a full run of this backlog
  would build concurrently and what it would hold back;
- `Cap`: how many build and fix agents run at once (default 4, and 2 during the shakedown, one per issue; each runs
  the full pre-push gate when it pushes); the one merge agent runs beside them, so at most cap + 1 gates run;
- `Estimate`: a wall-clock range, not a promise. When a report under `.claude/backlog/reports/` opens with a `Skill:`
  line naming this skill and this batch's flow, the last such report's `Timings` are the figures, as `choosing.md` takes
  them; otherwise per issue, guess its work from the issue and spec: a small bug about 30 minutes to a ready PR, a lint
  or workflow feature one to two hours, plus about 15 minutes for review and fixes. A merge round is about 15 minutes,
  and 30 to 40 when it meets a code conflict or a baseline regeneration (every new lint after the first in a batch meets
  one in the shared registry files); merges never overlap. A dependency chain runs end to end, each link waiting for the
  previous one's merge. The run takes about the longer of the longest chain and the first ready PR plus one merge round
  per issue. Say plainly that a short batch of small bugs saves little over `/work-backlog`, and recommend it instead
  when the estimate shows that, except while the shakedown is on: its two-issue batch exists to try this skill, so it
  gains little on `/work-backlog` and is proposed anyway. An estimate longer than the time budget names the issues that
  would likely skip;
- `Agents`: about three per issue (build, review, merge), up to ten with review rounds, merge retries and resolution
  reviews, each retried once if it dies. Say whether that exceeds the session's workflow size guideline, which the
  Workflow tool's description states: it is advice for whoever writes a script, not a limit, and nothing needs
  changing for the run to proceed. With no Workflow tool (a plan-only rehearsal), say `guideline unknown: no Workflow
  tool`. When the batch would reach the tool's hard limits, the concurrent-agent ceiling or the agents-per-run
  ceiling its description gives, name the limit and what it does to the estimate.

Two of the shared sections read differently here. `Budgets and shakedown` states `max-hours`, not a clock time, since
the deadline is computed at launch, after the sitting (`SKILL.md` "Shakedown and budgets"), and `max-parked`, the budget
only this skill uses ("Budgets"); and while the shakedown is on its expected-time line reads `Shakedown: two issues;
expect the first merge question at Ending, about <N> minutes after launch, which follows this sitting, and the second
after the first merges and the second is rebased and green again` (`one issue` and `the merge question` when the batch
is one), in place of the serial line, because the merge OK is asked once the run completes (`SKILL.md` "Named exceptions
to `/work-backlog`"). N is the span from `started:` to the merge question in the `Timings` of the last report whose
opening `Skill:` line names `/work-backlog-parallel` and this batch's flow, as `choosing.md` takes it for the serial
line; with no such report, the `Estimate` section's figure. Its line of the subagents the chunk will dispatch is left
out, since `Agents` gives that count here. `Runner-up` is the other batch size, as "Choosing the batch" says.

For a bug batch, the halt questions ride on the proposal, as in `/work-backlog`; the `Permissions` section reads
`bypassed`, since the session is ("The permission request" below). Ask every likely halt question, as
`/work-backlog` does, working through `/work-backlog` `halt-questions.md` for each issue: a halt parks the issue here
instead of stopping the session, but a park still waits for a later session to ask the same question.

## Feature batches

1. **Brainstorm per family** (`feature-flow.md` step 1). A batch with several unrelated features or families has one
   brainstorm and one spec each, in proposal order, one after another in this sitting.
2. **Spec, builder check, Fable review** per spec (`feature-flow.md` steps 2–4).
3. **One question round** for the whole batch (`feature-flow.md` step 5, without the `started:` line it appends:
   "The run" step 3 in `SKILL.md` writes it at launch): every held question from every spec, the
   halt questions (`halt-questions.md` again, for each issue) and rule conflicts, in one message, with `Permissions:
   bypassed`.

## The permission request

There is none: this skill runs only in a bypassed session (`SKILL.md` "Named exceptions to `/work-backlog`"), which
"The session" step 0 checks before planning and "The run" step 2 checks again before launch, so no rule is asked
for. A plan-only rehearsal's `Permissions` section reads `bypass required`.

## Writing it down

At the batch OK, write `current-session.md` as `/work-backlog` step 6 does, without its `started:` line, which "The
run" step 3 writes at launch (`SKILL.md` "Shakedown and budgets"). Its `flow:` line is `flow: bug` or
`flow: feature` for a batch of one kind, and `flow: mixed` for a batch of both, in which case every `issue` line ends
with that issue's `kind bug` or `kind feature` (`state.md`), so a serial session that later resumes a parked issue
knows its flow once `run-args.json` is gone. A feature batch's brainstorms and question round follow, and
`feature-flow.md` steps 2 and 5 record each spec path and the round's answers in that file, as does any halt in
between ("Halting" in `resume.md`). After the answered question round (or at once, for a bug batch), add `cap:`, the
`issue` lines with their `after`, `piece` and `kind` parts (`SKILL.md` "State") and each halt answer with the issues
it bears on (they become each issue's `haltAnswers`); their presence is what "On invocation" reads as a batch
approved for launch. Then go to "The run".
