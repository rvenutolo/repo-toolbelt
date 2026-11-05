# Planning the batch

Step 2 of `SKILL.md` "The session". Everything here happens with the user, in the main checkout, before the
workflow starts. It is `/work-backlog`'s "Choosing the chunk" and `feature-flow.md` steps 1–5, with the exceptions
below.

## Choosing the batch

`/work-backlog` "Choosing the chunk" holds (bugs first, related issues together, ordering factors, comments, earlier
pieces), except "Size": the user picks the batch size. Ask nothing about it up front; propose a batch that fits the
session shape the user asked for (a short session or a long unattended one), and when they named none, propose a
short one and name the longer batch the backlog would support.

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
`.coverage-baseline.txt`) are not a dependency: the merge queue resolves those conflicts (`merge-queue.md`).

An issue with no edge runs as soon as a slot is free.

## The proposal

One message, ending with the question banner, naming everything `/work-backlog`'s proposal names, plus:

- the graph, as the `issue <N> after <M>` lines `current-session.md` will hold;
- **the cap**: how many build, fix and merge agents run at once (default 4; each runs the full pre-push gate when it
  pushes);
- **a wall-clock estimate**, as a range, not a promise. Per issue, guess its work from the issue and spec: a small
  bug about 30 minutes to a ready PR, a lint or workflow feature one to two hours, plus about 15 minutes for review
  and fixes. A merge round is about 15 minutes (rebase, pre-push gate, CI of about six minutes), and merges never
  overlap. A dependency chain runs end to end, each link waiting for the previous one's merge. The run takes about
  the longer of the longest chain and the first ready PR plus one merge round per issue. Say plainly that a short
  batch of small bugs saves little over `/work-backlog`;
- **the agent count** (estimate: per issue, one build, two review and two fix agents at most, and for a code
  conflict up to three merge calls with a resolution review each; plan on eight per issue). Name the session's
  workflow size guideline (the Workflow tool's description states it as "This session has the … workflow size
  guideline") and say whether the estimate exceeds it, adding: "This is a guideline for whoever writes the script,
  not a limit; nothing needs changing for the run to proceed." Name the hard limits (16 concurrent agents, 1000 per
  run) only when the batch would reach them;
- **issues the session cannot do**, such as a setting only the user can change in a GitHub App or another repo:
  leave them out of the batch and name them, as `/work-backlog` names an issue that asks for what it forbids.

For a bug batch, the halt questions and the permission request ride on the proposal, as in `/work-backlog`. Halt
questions are fewer here: every per-issue halt parks the issue (`SKILL.md` "Named exceptions"), so ask only what
would otherwise park an issue that a one-word answer now could keep going.

## Feature batches

1. **Brainstorm per family** (`feature-flow.md` step 1). A batch with several unrelated features or families has one
   brainstorm and one spec each, in proposal order, one after another in this sitting.
2. **Spec, builder check, Fable review** per spec (`feature-flow.md` steps 2–4).
3. **One question round** for the whole batch (`feature-flow.md` step 5): every held question from every spec, the
   halt questions, rule conflicts and the permission request, in one message.

## The permission request

`/work-backlog` "Permissions", widened: a workflow pauses on any permission prompt, which stalls every issue at once.
List every command the build, review, fix and merge agents run (`agents.md`, `merge-queue.md`) that the allowlists
do not cover, including `./.ci/in-devshell` with `./run-tests` and `./run-all-checks`, `git branch -m`,
`git switch --detach`, `git rebase`, `git push` to `HEAD:refs/heads/<branch>`, `git push --force-with-lease`,
`gh pr create`, `gh pr edit`, `gh pr merge`, `gh run rerun`, `git worktree add` and `remove`, `git branch --delete`
and `cp`. Recommend starting the run with permissions bypassed when the list is long.

## Writing it down

After the OK (and, for features, the answered question round), write `current-session.md` as `/work-backlog` step 6
does, plus `cap:` and the `issue` lines (`SKILL.md` "State"). Then go to "The run".
