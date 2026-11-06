# Planning the batch

Step 2 of `SKILL.md` "The session". Everything here happens with the user, in the main checkout, before the
workflow starts. It is `/work-backlog`'s "Choosing the chunk" and `feature-flow.md` steps 1–5, with the exceptions
below.

## Choosing the batch

`/work-backlog` "Choosing the chunk" holds (bugs first, related issues together, ordering factors, comments, earlier
pieces), except "Size" and the rules that a chunk is all bugs or one related feature family (`SKILL.md` "Named
exceptions"): the user picks the batch size, and a batch may mix bugs with unrelated features. Ask nothing about
size up front; propose a batch that fits the session shape the user asked for (a short session or a long unattended
one), and when they named none, propose a short one and name the longer batch the backlog would support.

An umbrella issue contributes one piece per batch (`SKILL.md` "Named exceptions"): the next unbuilt piece its spec
names, as the issue's `piece`. A later piece waits for a later batch, or for `/work-backlog`.

**Each issue's kind** (`bug` or `feature`, which picks `bug-flow.md` or `feature-flow.md`, and is the `kind` in
`run-args.json`) is decided as `/work-backlog` "Choosing the chunk" decides it, and the proposal names the issues
whose kind did not come from the label, as there.

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
- **the agent count** (estimate: per issue, one build, two review and two fix agents at most, and up to three merge
  calls with two resolution reviews between them: eleven at most, about eight in practice; a dead agent's one retry
  and a resumed run's live re-runs add more). Name the session's
  workflow size guideline (the Workflow tool's description states it as "This session has the … workflow size
  guideline") and say whether the estimate exceeds it, adding: "This is a guideline for whoever writes the script,
  not a limit; nothing needs changing for the run to proceed." Name the hard limits (16 concurrent agents, 1000 per
  run) only when the batch would reach them;

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

`/work-backlog` "Permissions" (literal-prefix matching, its candidate list), widened: an agent that meets a
permission prompt waits for an answer nobody gives, holding its slot and stalling its issue and every issue that
depends on it. The request names rule strings, not commands. Build it by reading every command in `common.md`,
`agents.md`, `merge-queue.md` and the `/work-backlog` files they cite, then keep the rules the allowlists lack. The
run needs at least:

- `Bash(./.ci/in-devshell *)`: the tests, the gate, single-test TDD runs, a bug's reproduction, `./.ci/fmt` and
  `go run ./cmd/toolbelt update-coverage-baseline`; the merge agent runs it as `cd <worktree> && ./.ci/in-devshell
  …`, so also `Bash(cd *)`;
- `Bash(git -C *)`: every merge-queue command, and the resolution review's `diff`;
- `Bash(git add *)`, `Bash(git commit *)`, `Bash(git push *)`, `Bash(git fetch *)`, `Bash(git switch *)`,
  `Bash(git branch -m *)`, `Bash(git branch --delete *)`, `Bash(git merge --ff-only *)`, `Bash(git pull --ff-only*)`,
  `Bash(git rebase *)`, `Bash(git checkout --ours *)`, `Bash(git merge-base *)`, `Bash(git rev-parse *)`,
  `Bash(git log *)`, `Bash(git diff *)`, `Bash(git show *)`, `Bash(git status *)`, `Bash(git branch --list *)`;
- `Bash(git worktree list *)`, `Bash(git worktree add *)`, `Bash(git worktree remove *)`,
  `Bash(git worktree unlock *)`, `Bash(git worktree prune)`;
- `Bash(gh *)`, or each of `gh pr create|edit|checks|merge|list|view`, `gh issue view`, `gh run list|rerun` and
  `gh auth status`;
- `Bash(cp *)`, `Bash(mkdir --parents *)`, `Bash(timeout *)` (the waits in `common.md` "Long commands"),
  `Bash(kill -0 *)`, `Bash(sysctl net.ipv4.ip_forward)`, `Bash(ssh-add -L)`, and what `/work-backlog` "Permissions"
  names for executing-plans: its helper scripts and the `rm` of a plan workspace.

The list is long and a miss stalls that issue, its slot and its dependents, so recommend running the session with
permissions bypassed; `SKILL.md` "The session" step 1 asks about it before planning, and "The run" step 2 checks
again before launch.

## Writing it down

After the OK (and, for features, the answered question round), write `current-session.md` as `/work-backlog` step 6
does, plus `cap:`, the `issue` and `piece` lines (`SKILL.md` "State") and each halt answer with the issues it bears
on (they become each issue's `haltAnswers`). Then go to "The run".
