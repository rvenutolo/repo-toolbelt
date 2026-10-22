# repo-toolbelt

Shared lints for the author's repositories, Nix-based or not, shipped as one Docker image. CI, local runs and git hooks
all run that image, so every repo is checked the same way and at the version it pins. Docker is the only requirement on
a consuming machine; Nix is used only to develop and build this repo.

Linters are what exist today. Formatters and publishing workflows are planned.

## Lints

`toolbelt list` prints these from the manifest, `lints.toml`.

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

The config file is needed only for exemptions or a lint's required settings; without either, leave it out. Its comments
document each lint's section.

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

### Git hooks

Turn the hooks on once per clone:

```bash
git config core.hooksPath .githooks
```

Each hook runs the pinned image with `docker run`, mounting the checkout read-only at its host path, with no network and
as the caller's user and group. The lints it lists run concurrently in the one container. Hooks normally list only quick
lints; a slow lint run from pre-commit prints a one-line warning and still runs. The hooks lint the working tree as it
is on disk and warn when it holds unstaged or uncommitted edits.

A repo that already has a hook keeps it and adds the `docker run` lines shown in the example hook's header comment.

### Coverage lints

`check-coverage-drop` and `check-patch-coverage` read the coverage reports the repo's own tests write; the image never
runs tests. Both sections of `.repo-toolbelt.yml` list the reports as `format: go` (a `go test -coverprofile` file) or
`format: cobertura` (what kcov writes). A missing report is a finding.

In CI, the repo's test job uploads the reports as one artifact with paths relative to the workspace, the lint job
declares `needs:` on the test job, and the workflow input `coverage-artifact:` names the artifact. The shared workflow
restores it into the checkout at the same paths.

- **Ratchet.** `check-coverage-drop` compares the count of uncovered statements (Go) or lines (Cobertura) per unit, a Go
  package or a directory, against the committed baseline, `.coverage-baseline.txt` by default. Any rise fails, and a
  fall passes.
- **Updating the baseline.** The updater is the one command that mounts the checkout read-write. Run the tests first,
  so the reports are current:

  ```bash
  root="$(git rev-parse --show-toplevel)"
  common="$(git rev-parse --path-format=absolute --git-common-dir)"
  docker run --rm --network=none --user "$(id -u):$(id -g)" \
    --volume "${root}:${root}" --volume "${common}:${common}:ro" --workdir "${root}" \
    ghcr.io/rvenutolo/repo-toolbelt:<tag>@sha256:<digest> update-coverage-baseline
  ```

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

## The calling convention

Every lint is an executable at `/opt/toolbelt/lints/<name>/<name>` in the image, whatever its language: bash for lints
that mostly wrap a tool, Go for those that need real parsing. A bash lint's source is `lints/<name>/<name>`; a Go lint is
built from `cmd/<name>`.

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
concurrently and prints each lint's output as one block, in the order named. `toolbelt plan` prints the job matrix for
the shared workflow, `toolbelt list` the lints, and `toolbelt update-coverage-baseline` rewrites the baseline. The runner
reads `.repo-toolbelt.yml` once, checks the file as a whole, and hands each lint its own section; each lint validates its
section's schema before scanning.

### Adding a lint

1. Write the executable: bash at `lints/<name>/<name>`, sourcing `lib/toolbelt.bash` by a path relative to itself, or Go
   at `cmd/<name>/main.go` calling `lintkit.Main`, with the check in a package under `internal/`.
2. Add an entry to `lints.toml` with its name, language, `quick` or `slow` and a one-line description.
3. Add the lint to every file under `examples/consumer/`, with `# slow` in the pre-commit example if it is slow, and its
   config section to the example config if it takes one. `.ci/check-examples` fails when an example omits a lint,
   names an unknown one, or marks speed differently from the manifest.
4. Write spec-driven tests for every behavior: BATS for bash, Go's `testing` package for Go.

A release that only adds lints changes nothing in a consuming repo, because each consumer names its lints.

## Development

Every tool runs through the devShell wrapper, so nothing needs installing but Nix and Docker:

```bash
./.ci/in-devshell ./run-tests          # Go tests and BATS under kcov; reports go to .coverage/
./.ci/in-devshell ./run-all-checks     # the full gate: linters, tests, then the coverage lints
./.ci/in-devshell ./.ci/test-image     # builds the image, loads it into Docker and runs the e2e suite
./.ci/in-devshell nix fmt              # formats every tracked file
git config core.hooksPath .githooks    # commit-msg and pre-push hooks
```

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
as that release commit's own workflow run; `release-publish` refuses a run of any other commit. The re-run pushes the
same digest.

After a release, update the consumers' pins, and fix any consumer a changed lint now fails before its pin moves.
