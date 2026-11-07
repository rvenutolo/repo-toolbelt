# Choosing the chunk

Step 6 of "The session" in `SKILL.md`: how the chunk is indexed, classified, formed and proposed. Everything here
happens with the user present, before the unattended part. Section and bullet names in quotes without a file are
`SKILL.md`'s.

## The index

Run `<main>/.claude/skills/work-backlog/backlog-index` (with the issue numbers when the arguments name them), with
`<main>` the main checkout's absolute path, which its allow rule (`permissions.md`) names. It writes every open
issue, body included, to the main checkout's `.claude/tmp/issues.json` and prints one tab-separated row per issue:
`number`, `state`, `kind`, `labels`, `title`, `repro`, `refs`, `umbrella`, `bytes`. The bodies run to well over what
the Bash tool shows, and a classification read from a cut-off listing misses issues, so classify from the rows and
read a full body (`jq --raw-output '.[] | select(.number == <N>) | .body' .claude/tmp/issues.json`) only for a row
whose `kind` is `?`, a non-bug row with `repro` `yes`, and every issue the proposal takes; for those `?` and `repro`
rows read the comments too ("Comments" below) before classifying, since a kind or a size can live only there. A body
the Bash tool would cut off (`bytes` past about 16000) the script also writes to `.claude/tmp/issue-<N>.md`: read
that with the Read tool, which pages. `refs` (open issues only) and `umbrella` show the dependencies and umbrella
issues "Order" below and the proposal need. A non-zero exit is a failed `gh` call (`SKILL.md` step 1).

## Kinds

An issue is a bug when it has the `bug` label, its title starts `fix:`, or its body reports something that is wrong
now with a way to reproduce it; otherwise it is a feature, whatever other labels it carries. The body counts because
few issues carry the label, and a three-line fix sent down the feature flow costs a brainstorm, a spec and a Fable
review. A check that fails on `main` today, or a config whose present value is wrong, is a reproduction; a `ci:` or
`chore:` title is a judgment call the proposal names. The `kind` column decides only the first two (`bug`, or
`feature` from a `feat:` title or the `enhancement` label); `?` is neither. The proposal names every issue whose kind
was a judgment call, so the user can correct it with the chunk OK: a bug found from its body alone, and a `?` row. An
issue whose kind follows from its label or title is not named. A kind the OK corrects re-sorts the chunk: the issue
follows the corrected flow, and a chunk now holding both kinds splits as "Arguments" says, bugs this chunk and
features the runner-up. If any open issue is a bug the session can do, the chunk is bugs; otherwise it is features. A
bug the session cannot do at all ("The proposal" below) does not count, or it would block every later chunk until
the user acts.

## Forming the chunk

- **Bugs:** related bugs together (same code, same root cause). With none related, a batch of several small bugs.
- **Features:** one feature, or several that are closely related: they share code, one is a direct prerequisite of
  another, or they form a family that shares a runner, helper or template (such as the lints that wrap an external
  tool, or the shared workflows). One brainstorm covers a family.
- **Order:** by implementation sequence as the global "Prioritizing Work Items" rule defines it: what unblocks other
  items, then what speeds up the feedback loop, then what retires risk. Not by product value, and not by size. The
  first factor counts any open issue the item unblocks, in the chunk or not, and a prerequisite internal to the chunk
  that no issue names (a shared helper the pieces need) counts under it too.
- **Size:** two to four PRs; one while shakedown is on ("Shakedown"). Each PR waits on its own CI run and merges before
  the next starts, and the findings walkthrough still follows, so more than four rarely finishes in a session. When a
  family is bigger, propose the part that fits; the rest stays open for the next session, and the spec covers only the
  proposed part, with a shared helper designed so the rest can reuse it.
- **Combined changes:** an issue body or comment that asks for its change to land together with another's still
  gets one PR per issue ("PRs"): order the two by the dependency between them, and say so in the proposal. The
  partner is classified with its requester when the body rule allows (a `?` row asked to land with a bug is a bug);
  otherwise it is named as the runner-up, with the dependency, since a chunk follows one flow.
- **Comments:** for the issues in the proposed chunk, also read `gh issue view <number> --comments` (off a terminal
  it prints only the comments, which is what is wanted: the body is in the index). A decision or requirement the
  user left in a comment changes the proposal and binds the build as the body does, so the builder reads the
  comments again when it starts the issue.
- **Earlier pieces:** for each issue in the proposed chunk, also run
  `gh pr list --state merged --limit 1000 --search '"Part of #<number>" in:body' --json number,title,body` (the
  quotes keep the phrase whole; unquoted, the search matches the three words anywhere in a body), then confirm the
  body holds the whole line `Part of #<number>`, as `leftovers.md` matches it. A merged piece means an earlier
  session built part of this family or umbrella from a spec under `docs/superpowers/specs/` that names the issue:
  name that spec in the proposal, and `feature-flow.md` step 1 extends it instead of starting over.

## The proposal

One message, beginning with the question banner, that names:

- the issues, why they belong together, and why they come first by the three ordering factors;
- the runner-up chunk and why it lost, so the user can swap with one word, or "none" when no other chunk could be
  formed (every doable issue of the chosen kind is already in this one);
- the user's own work step 3 saw and left alone, or "none";
- the backlog's trend, as `Backlog: <prev> → <now> open since <date>`, from the `open=` and date of the last
  `session-log.md` line before this session's and the count step 2 took (not `backlog-index`'s row count, which
  covers only the named issues when the arguments name some), or `Backlog: <now> open, first session` when this
  session's line is the first; the stall check sees only three sessions apart, and this shows growth between any two;
- the time budget and whether shakedown is on ("Budgets", "Shakedown"), so the user can change either with the OK,
  and, while shakedown is on, that the session will ask for the merge OK when CI is green, about fifteen minutes
  after the PR opens, so the user knows to stay or come back;
- when the chunk holds several independent issues and a long unattended run, that `/work-backlog-parallel` would
  finish it sooner;
- for a bug chunk, every likely halt question for the unattended part ("Terms"; `halt-questions.md` lists the kinds
  that recur, each with its default answer) and the permission request (`permissions.md`). The OK on the chunk
  answers the questions; the permission request needs its own answer. A feature chunk asks both in
  `feature-flow.md` step 5 instead;
- an issue whose own text asks the session to do something this skill forbids (open further issues, change another
  repo, cut a release), with the question it raises, so the user settles it with the chunk. An issue that describes
  a product which does such things, such as a workflow that opens issues, raises no question;
- an issue the session cannot do at all, such as one that needs a setting only the user can change in a GitHub App's
  settings or in another repo: leave it out of the chunk and name it, with what the user would have to do;
- an umbrella issue in the chunk, one that asks for sub-issues to be filed and then worked: it is worked as one PR
  per piece ("PRs") unless the user opts in the proposal to have the sub-issues opened instead.

The user may swap or trim the chunk with the OK. A swapped-in issue gets the proposal's per-issue items (comments,
earlier pieces, kind, halt questions) in a follow-up message, which is not a second question round (`permissions.md`
"When to ask" says why a follow-up is allowed).
