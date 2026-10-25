# repo-toolbelt

Shared lints and formatters for the author's repositories, Nix-based or not, shipped as one Docker image. CI, local runs
and git hooks all run that image, so every repo is checked the same way and at the version it pins. Docker is the only
requirement on a consuming machine; Nix is used only to develop and build this repo.

The lints and the formatter framework exist. No formatter ships yet. Publishing workflows are planned.

## Lints

`toolbelt list` prints these from the manifests: `lints.toml`, and `formatters.toml`, where each formatter adds its
check, `check-<name>-formatted`, to the list (see [Formatters](#formatters)).

- `check-workflow-hermetic`: every workflow `run:` step goes through the devShell wrapper.
- `check-issue-numbers`: no local issue or PR number in tracked files.
- `check-coverage-drop`: uncovered code never rises without a recorded reason.
- `check-patch-coverage`: every changed block is covered or marked exempt.
- `check-toolbelt-pins`: every repo-toolbelt pin agrees.

There is no default set. A consumer names the lints it runs in each file that runs the image, and lists only those that
fit the repo. A lint never checks whether it applies: run in a repo it does not fit, it fails.

## Consuming it

Copy the files under `examples/consumer/` into the repo and delete the lints the repo does not want:

- `examples/consumer/.github/workflows/lint.yml` to `.github/workflows/lint.yml`
- `examples/consumer/.githooks/pre-commit` to `.githooks/pre-commit`
- `examples/consumer/.githooks/pre-push` to `.githooks/pre-push`
- `examples/consumer/.repo-toolbelt.yml` to `.repo-toolbelt.yml`

The config file is needed only for exemptions, a lint's required settings, or a formatter's excludes and settings;
without any, leave it out. Its comments
document each lint's section. It is plain YAML: an alias, a merge key (`<<`) or an explicit tag, core tags such as
`!!str` included, is a finding. Quote a value instead of tagging it.

### Pin the image

The examples hold a placeholder pin. Replace it with a release: the image as
`ghcr.io/rvenutolo/repo-toolbelt:<tag>@sha256:<digest>` and the shared workflow by the release's commit SHA with a
`# <tag>` comment. The image reference appears in the workflow's `image:` input and in each hook's `IMAGE`; all copies
must be the same, which `check-toolbelt-pins` enforces. Renovate updates every copy in one PR.

### CI

The workflow is one job that calls the shared lint workflow with the image and the lint list.

- `mode:` picks the scheduling. `split` (the default) runs the quick lints in one job and each slow lint in its own job.
  `parallel` runs every lint concurrently in one job. `matrix` gives every lint its own job.
- Every mode ends in one aggregate check, `lint / toolbelt-ok` (the caller's job ID, then the aggregate job's name).
  Make that the required check in the repo's ruleset. Changing the mode or the lint list never changes it.
- The workflow verifies the image's build provenance before running it, so the image must come from a release.
- The repo's Actions allowlist must permit `actions/checkout`, `actions/download-artifact` and
  `step-security/harden-runner`.
- `harden-runner: false` turns off harden-runner, whose free tier does not cover private repositories.
- The calling workflow may run on `pull_request`, `push`, `workflow_dispatch` and `merge_group`. Do not call it on
  `schedule`, whose payload has no repository default branch (the run exits 2), or on `pull_request_target`, which
  checks out a tree without the pull request's head.

### Git hooks

Turn the hooks on once per clone:

```bash
git config core.hooksPath .githooks
```

Each hook runs the pinned image with `docker run`, mounting the checkout read-only at its host path, with no network and
as the caller's user and group. The lints it lists run concurrently in the one container. Hooks normally list only quick
lints; a slow lint run from pre-commit prints a one-line warning and still runs. The hooks lint the working tree as it
is on disk and warn when it holds unstaged or uncommitted edits. A pre-commit hook that lists formatters first runs them
in a container of their own with the checkout read-write (see [In the pre-commit hook](#in-the-pre-commit-hook)).

A repo that already has a hook keeps it and adds the `docker run` lines shown in the example hook's header comment.

### Coverage lints

`check-coverage-drop` and `check-patch-coverage` read the coverage reports the repo's own tests write; the image never
runs tests. Both sections of `.repo-toolbelt.yml` list the reports as `format: go` (a `go test -coverprofile` file) or
`format: cobertura` (what kcov writes). A missing report is a finding. A `go` report needs the Go module at the repo
root: the lints read `go.mod` there to map the profile's import paths to files.

In CI, the repo's test job uploads the reports as one artifact with paths relative to the workspace, the lint job
declares `needs:` on the test job, and the workflow input `coverage-artifact:` names the artifact. The shared workflow
restores it into the checkout at the same paths.

- **Ratchet.** `check-coverage-drop` compares the count of uncovered statements (Go) or lines (Cobertura) per unit, a Go
  package or a directory, against the committed baseline, `.coverage-baseline.txt` by default. Any rise fails, and a
  fall passes. It skips uncovered code that a `coverage-exempt` marker with a reason covers (see Patch coverage). The
  ratchet does not police markers: only `check-patch-coverage` reports a marker with no reason, one on no measured
  line, or a stale one, so a repo running only `check-coverage-drop` gets none of that. The trade is deliberate: a
  marker on a real but untested line passes the ratchet with no `Coverage-Drop:` trailer, and `check-patch-coverage`
  reports the marker as stale once a test covers the line.
- **Updating the baseline.** The updater mounts the checkout read-write, as `fmt` does. Run the tests first, so the
  reports are current. A consumer may rerun it to lower its baseline when markers or new tests have cut the count:

  ```bash
  root="$(git rev-parse --show-toplevel)"
  common="$(git rev-parse --path-format=absolute --git-common-dir)"
  docker run --rm --network=none --user "$(id -u):$(id -g)" \
    --volume "${root}:${root}" --volume "${common}:${common}:ro" --workdir "${root}" \
    ghcr.io/rvenutolo/repo-toolbelt:<tag>@sha256:<digest> update-coverage-baseline
  ```

  `update-coverage-baseline --check` writes nothing. It compares the committed baseline with the one the updater would
  write, prints one `<format> <unit>: baseline <old>, current <new>` line per differing unit, and exits 1 on any
  difference. A consumer can run it as a gate step to keep slack out of its baseline.

- **Recording a rise.** A commit that raises a baseline line must carry `Coverage-Drop: <unit> - <reason>` on one line in
  the last paragraph of its message, with any other trailers (`Co-Authored-By:` and the like) directly under it and no
  blank line between them. Prefer a test to a raised baseline.
- **Patch coverage.** Every changed block or line must be covered, or carry a marker with a reason:
  `// coverage-exempt: <reason>` in Go and `# coverage-exempt: <reason>` in bash. A marker with no reason, one on no
  measured line, or one whose code is covered is a finding.
- **Lines kcov scores that never run as bash.** kcov counts lines inside multi-line quoted text, such as a yq or awk
  program, or a `done < file` line, as code that no test can hit. Mark each with `# coverage-exempt:` and a reason. A
  marker placed inside single-quoted program text is a comment of that language (a yq comment, for one), so keep
  fragments that are composed into a larger program newline-terminated, or the marker swallows what follows it.
- **kcov setup.** Run kcov with `--bash-parse-files-in-dir` for the directories holding the bash under test. Without it
  kcov reports only the files a test executed, and an untested script is invisible to both lints.
- **kcov output.** kcov writes helper scripts with shebangs into its output directory. Keep that directory in
  `.gitignore` and out of any `find`-based shell linting.

### Issue numbers

`check-issue-numbers` scans each tracked file for `#<digits>` and does not parse languages. It skips hex colours, shell
`$#` and `${#...}`, and numbers in full URLs. A quoted value that is the whole of a string after `:` or `=`, such as
`color: "#540"`, is read as a colour, so a quoted bare issue number in that position goes unreported. State a reason in
comments and docs instead of pointing at an issue; exempt the rest in `.repo-toolbelt.yml` with a `path` glob and, to
narrow it, a `match` regular expression.

It reads the tracked files `git grep` takes for text, skipping empty files and the ones git takes for binary, by content
or by a gitattributes entry such as `-diff`. A tracked text file in the working tree that git does not read, such as one
it cannot open or one marked skip-worktree, fails the run as a setup error rather than passing unchecked.

## Formatters

A formatter rewrites a repo's files in the author's style. Formatters are named by the language they format, not by
the tool they run. The style lives in the image: a consumer can exclude paths and change the few settings a formatter
allowlists, nothing else, and no file in the consumer's tree is read as formatter config.

No formatter ships yet: `formatters.toml` lists none, so `fmt` rejects every name as an unknown formatter.

A formatter is reached two ways and runs the same executable both times:

- **As a lint**, `check-<name>-formatted`. `run`, `plan`, `list` and the shared workflow carry it like any other lint.
  It runs the formatter in check mode, which writes nothing and reports each file that would change. CI only checks.
- **As `toolbelt fmt <name>...`**, which rewrites the files in place. It needs the checkout mounted read-write, so it
  runs by hand or from a pre-commit hook that opts in. Nothing ever re-stages a file.

### Which files a formatter gets

The runner, not the formatter, picks the files, the same way for the lint and for `fmt`:

1. The files git tracks, read in the working tree as they are on disk. Untracked files are never formatted.
2. Only regular files: a tracked file deleted from the working tree, a symlink, a file under a directory that is now a
   symlink, and a submodule are skipped.
3. A file belongs to a formatter when its base name matches one of the formatter's include globs, or when it is
   extensionless, no formatter's include claims it, and its first line is a shebang naming one of the formatter's
   interpreters, directly (`#!/bin/bash`) or through `env` (`#!/usr/bin/env -S bash -e`). A base name is extensionless
   when it holds no `.` after its first character, so `run-tests` and `.envrc` are and `.envrc.local` is not.

   A tracked file whose base name the include globs of two formatters match is a setup failure, exit 2, naming the
   file and both formatters. This is checked before steps 2 and 4 skip anything, so it holds whatever the file's state
   on disk (deleted, a symlink) and when it has `linguist-vendored` set; only a file under a `vendor` directory is
   skipped first.
4. Vendored files are skipped by every formatter: a file under a directory named `vendor`, at any depth, or one with
   `linguist-vendored` set in `.gitattributes`.
5. The excludes of the formatter's config section are removed.

### The config section

A formatter's section of `.repo-toolbelt.yml` is keyed by its lint's name, `check-<name>-formatted`, and both the lint
and `fmt <name>` read it:

```yaml
check-<name>-formatted:
  exclude:
    - test/fixtures/**
  settings:
    <setting>: 4
```

- Both keys are optional, and an empty section is no section.
- `exclude:` is a list of non-empty globs over repo-relative paths, in the glob syntax of `check-issue-numbers`'
  `path`: `*`, `?` and character classes within a segment, `**` across segments, and a glob that matches a leading
  directory matches everything under it. It does not check spelling as that lint does: `./x`, `/x` and `x/` are not
  rejected, so write each glob as git prints the path, with no leading `./` or `/` and no trailing `/`. A glob that
  matches no file is not reported.
- `settings:` maps settings on the formatter's allowlist to positive integers. A formatter with no allowlist rejects
  every setting.

The runner validates the section before the formatter runs, and only for the formatters a run names. Each problem is
printed as `.repo-toolbelt.yml: check-<name>-formatted: <problem>`, then a count line. A bad section fails only that
formatter: in `run` its lint fails with exit 1 and the other lints still run, and in `fmt` its block fails and the
other formatters still run. A problem in the file as a whole, such as an unknown section, still stops the run before
anything runs.

### Running `fmt`

`fmt` needs the checkout mounted read-write:

```bash
root="$(git rev-parse --show-toplevel)"
common="$(git rev-parse --path-format=absolute --git-common-dir)"
docker run --rm --network=none --user "$(id -u):$(id -g)" \
  --volume "${root}:${root}" --volume "${common}:${common}:ro" --workdir "${root}" \
  ghcr.io/rvenutolo/repo-toolbelt:<tag>@sha256:<digest> fmt <name>...
```

The formatters are named as arguments or, with none, in `TOOLBELT_FORMATTERS`, one per line. Naming none, an unknown
formatter or one twice, or using both sources, is a usage error. `fmt` refuses a checkout the caller does not own, and
refuses a read-only one with exit 2 and the line above. It runs as the caller's user and group, so the files it writes
stay theirs.

The named formatters run concurrently, and each prints one block, in the order named: a header giving its verdict
(`pass` when it exited 0 and nothing changed, `formatted` when it exited 0 after rewriting files, `fail` when it exited
1 and `error` when it errored), a `<path>: formatted` line for each file it rewrote, whatever the verdict, so a failing
block can list rewritten files too, then its own findings, such as a file it cannot parse. The runner finds the
rewritten files by hashing every selected file before and after: a file gone after the run, deleted or under a
directory that is now a file, counts as rewritten, and one it cannot read then is a setup failure, exit 2. A summary
line follows, such as
`toolbelt: 2 file(s) formatted, 0 failed, 0 errored; review and stage them`; the advice to review and stage appears
whenever a file was rewritten.

Exit codes:

- 0: every formatter passed or rewrote its files.
- 1: a formatter reported a finding (a file it cannot parse, a bad config section), the config file has problems,
  `--fail-on-change` is given and a file was rewritten, or a runtime error such as a failed write of the output.
- 2: a formatter errored, or a usage or setup error, a read-only checkout among them.
- 130: the run was interrupted.

Run by hand, `fmt` exits 0 after a successful rewrite, like `gofmt -w`. `--fail-on-change` is for a hook: it makes a
rewrite exit 1, so the commit stops for the rewritten files to be reviewed and staged.

### In the pre-commit hook

The example pre-commit hook carries a `FORMATTERS` array, empty by default, beside `LINTS`. To have the hook rewrite
files instead of only failing, add the languages to `FORMATTERS` and delete their `check-<name>-formatted` lines from
`LINTS`. The hook then first runs `fmt --fail-on-change` for those languages with the read-write mount and stops on a
non-zero exit, a rewrite included, leaving the rewritten files unstaged. When `fmt` exits 0, having rewritten nothing,
the hook goes on to run the lints. With `FORMATTERS` empty, it runs only the lints; with `LINTS` empty, it only
formats, and exits 0 once `fmt` does. The pre-push hook only checks: at
push time the commits exist, so a rewrite could only stop the push.

## The calling convention

Every lint in `lints.toml` is an executable at `/opt/toolbelt/lints/<name>/<name>` in the image, whatever its language:
bash for lints that mostly wrap a tool, Go for those that need real parsing. A bash lint's source is
`lints/<name>/<name>`; a Go lint is built from `cmd/<name>`. A formatter's lint, `check-<name>-formatted`, runs the
formatter's executable in check mode instead (see [The formatter calling convention](#the-formatter-calling-convention)).

- It runs with the repo root as its working directory and takes no arguments except `--help`.
- Its environment carries `TOOLBELT_SECTION`, the path to a JSON file holding the lint's config section (JSON `null`
  when there is none); `TOOLBELT_HOOK`, one of `pre-commit`, `pre-push` or `ci`; `TOOLBELT_BASE`, the commit a
  diff-aware lint compares the tree against; and `TOOLBELT_RANGE`, the commits a history-aware lint judges, in
  `git rev-list` syntax. The last two may be empty.
- Findings go to stdout, one per line, as `<path>: <message>`, `<path>:<line>: <message>`,
  `<path>:<start>-<end>: <message>` for a line range, or `<message>` when there is no location. A user-controlled value
  is escaped onto one line. A count line follows when there are findings. Diagnostics that are not findings go to
  stderr.
- Exit codes: 0 no findings, 1 findings (config errors in the lint's own section included), 2 a setup failure that is
  not a finding, such as a missing tool or a malformed report.

The runner, `toolbelt`, is the image's entrypoint. `toolbelt run --hook <hook> <lint>...` runs the named lints
concurrently and prints each lint's output as one block, in the order named. `toolbelt fmt <name>...` rewrites files
with the named formatters, `toolbelt plan` prints the job matrix for the shared workflow, `toolbelt list` the lints, and
`toolbelt update-coverage-baseline` rewrites the baseline. The runner reads `.repo-toolbelt.yml` once, checks the file
as a whole, and hands each lint its own section; each lint validates its section's schema before scanning, except a
formatter's lint, whose section the runner validates because it applies the excludes itself.

### The formatter calling convention

Every formatter is an executable at `/opt/toolbelt/formatters/<name>/<name>` in the image. A bash formatter's source is
`formatters/<name>/<name>`, and any config files for its tool sit beside it in `formatters/<name>/`, ship beside it in
the image and are passed to the tool explicitly; a Go formatter is built from `cmd/fmt-<name>`.

- It runs with the repo root as its working directory and takes no arguments except `--help`, like a lint.
- Its environment carries `TOOLBELT_MODE`, `check` (as the lint) or `write` (from `fmt`); `TOOLBELT_FILES`, the path
  to a file holding the repo-relative paths to format, each terminated by NUL, already selected and with the excludes
  removed, possibly none; and `TOOLBELT_SECTION`, the path to a JSON file holding its config section, already validated
  (JSON `null` when there is none). `TOOLBELT_HOOK`, `TOOLBELT_BASE` and `TOOLBELT_RANGE` are as for a lint in check
  mode and empty in write mode.
- **Check mode** writes nothing anywhere in the checkout. It prints one finding per file that would change,
  `<path>: not formatted`, then the count line. A file the tool cannot parse is a finding,
  `<path>: cannot be parsed: <message>`, not a setup failure. Exit 0 with no findings, 1 with findings.
- **Write mode** rewrites in place each file that would change, truncating and writing it rather than replacing it,
  so the file keeps its inode, owner and mode bits. A file it cannot parse is reported as in check mode and left
  untouched. Exit 0 when every file is now formatted, 1 when any file could not be parsed. The formatter does not
  report which files it rewrote; the runner does, from its hashes.
- A tool's parse error is a finding. Any other tool failure, such as a write error, a denied permission or a crash, is
  a setup failure, exit 2. A tool that uses one exit code for all of these is told apart by its stderr.
- In both modes, exit 2 for a setup failure, such as a missing tool or an unreadable file list. Temporary files go
  under `$TMPDIR`, or `/tmp` when it is unset, as in the image; never in the checkout. The section's `settings:`
  override the image's defaults, and the formatter reads no config file from the checkout.
- An empty file list passes in check mode and does nothing in write mode.

### Adding a lint

1. Write the executable: bash at `lints/<name>/<name>`, sourcing `lib/toolbelt.bash` by a path relative to itself, or Go
   at `cmd/<name>/main.go` calling `lintkit.Main`, with the check in a package under `internal/`.
2. Add an entry to `lints.toml` with its name, language, `quick` or `slow` and a one-line description.
3. Add the lint to every file under `examples/consumer/`, with `# slow` in the pre-commit example if it is slow, and its
   config section to the example config if it takes one. `.ci/check-examples` fails when an example omits a lint,
   names an unknown one, or marks speed differently from the manifest.
4. Write spec-driven tests for every behavior: BATS for bash, Go's `testing` package for Go.

A release that only adds lints changes nothing in a consuming repo, because each consumer names its lints.

### Adding a formatter

1. Write the executable to the formatter calling convention: bash at `formatters/<name>/<name>`, sourcing
   `lib/toolbelt.bash` by a path relative to itself, with any config files for its tool beside it; or Go at
   `cmd/fmt-<name>/main.go`, with the logic in a package under `internal/`.
2. Add a `[[formatter]]` entry to `formatters.toml`: `name`, the language it formats, in lowercase letters and digits
   starting with a letter (it names both `fmt <name>` and the lint `check-<name>-formatted`); `language`, `bash` or
   `go`; `speed`, `quick` or `slow`, which schedules its lint; a one-line `description`; `include`, the non-empty list
   of globs over a file's base name that it owns; and optionally `shebangs`, the interpreters an extensionless file's
   shebang may name, and `settings`, the allowlist a consumer may override. Unknown keys, a lint name that collides
   with a lint in `lints.toml`, and an include glob or shebang that another formatter also lists are errors. Distinct
   globs of two formatters must not match one base name either: a run that finds a tracked file both match fails with
   exit 2.
3. For a bash formatter, name the Nix packages that supply its tool in `nix/formatter-tools.nix` (an empty list when it
   needs none beyond the library's), which puts them on its `PATH` in the image, and add them to the devShell in
   `flake.nix`, where `.ci/build-dist` finds them for local runs.
4. Add its coverage units to `.coverage-baseline.txt` in the same commit, or the baseline check in `run-all-checks`
   fails: `cobertura formatters/<name> 0` for bash, whose directory `run-tests` measures with kcov, or
   `go cmd/fmt-<name> 0` and a line for its `internal/` package for Go.
5. Add `check-<name>-formatted` to every lint list under `examples/consumer/`, with `# slow` in the pre-commit example
   if it is slow, and `# <name>` to the pre-commit example's `FORMATTERS` block. `.ci/check-examples` fails when the
   block omits a formatter, names an unknown one, holds a line that is not an entry, or appears in the pre-push example.
6. Write spec-driven tests for every behavior, both modes included.

## Development

Every tool runs through the devShell wrapper, so nothing needs installing but Nix and Docker:

```bash
./.ci/in-devshell ./run-tests          # Go tests and BATS under kcov; reports go to .coverage/
./.ci/in-devshell ./run-all-checks     # the full gate: linters, tests, then the coverage lints
./.ci/in-devshell ./.ci/test-image     # builds the image, loads it into Docker and runs the e2e suite
./.ci/in-devshell ./.ci/fmt            # formats every tracked file: nix fmt, then the dogfooded formatters
git config core.hooksPath .githooks    # commit-msg, pre-commit and pre-push hooks
```

The pre-commit hook checks the working tree's formatting with the dogfooded formatters' check lints, run by the
runner built from the working tree; `./.ci/in-devshell ./.ci/fmt` rewrites what it finds.

`just test`, `just check`, `just lint` and `just fmt` are shortcuts for the first, second, linter-only and formatter
commands. Nothing requires `just`.

Commit subjects follow the Angular convention, checked by commitlint. This repo runs its own lints on itself, including
the coverage ratchet, so a commit that raises `.coverage-baseline.txt` carries a `Coverage-Drop:` trailer as above.

## Releasing

The image is built with Nix and published to `ghcr.io/rvenutolo/repo-toolbelt` for `linux/amd64`. Consumers see only
tagged releases.

1. The `release` workflow's `propose` job opens a pull request titled `chore(release): vX.Y.Z` that changes
   `CHANGELOG.md` only. The version is computed from the Conventional Commit types merged since the last tag, or named
   with the `version` input.
2. Merging that pull request is the release decision. Its `publish` job builds the image, pushes it to GHCR under the
   version, attests its build provenance against the pushed digest and publishes the release, which creates the tag.
3. After the first push, make the GHCR package public once by hand, so consumers can pull it.

The workflow runs only on `workflow_dispatch` until a `REFRESH_TOKEN` secret exists. Once it does, add the
`push: branches: [main]` trigger, so merging the release pull request starts `publish`. A dispatch with `dry-run` builds
the image and uploads it as a workflow artifact, pushing and tagging nothing; adding `sign` also attests that archive,
which writes a public transparency-log entry that cannot be removed.

A `publish` that fails after the push leaves the image tag and its attestation in place. Re-run it from the Actions page
as that release commit's own workflow run; `release-publish` refuses a run of any other commit. The re-run should push
the same digest, since Nix builds the image reproducibly.

After a release, update the consumers' pins, and fix any consumer a changed lint now fails before its pin moves.
