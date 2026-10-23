#!/usr/bin/env bats

# run-all-checks runs in a fixture repo, so every step it calls by path is a
# stub there, and a stub `nix` comes first on PATH. Each stub appends
# its name and arguments to CALLS, and exits with the status in its
# <NAME>_STATUS variable (0 when unset).

# The runner's arguments in CI, up to the lints it is given.
readonly TOOLBELT_RUN='toolbelt run --hook ci --base origin/main --head HEAD --default-branch main'

function setup() {
  load test_helper/common
  make_repo
  CALLS="${BATS_TEST_TMPDIR}/calls"
  export CALLS
  mkdir "${BATS_TEST_TMPDIR}/bin" .ci
  write_stub "${BATS_TEST_TMPDIR}/bin/nix" NIX_STATUS
  write_stub .ci/run-lint-checks LINT_STATUS
  write_stub .ci/check-examples EXAMPLES_STATUS
  mkdir --parents examples/consumer
  write_stub run-tests TESTS_STATUS
  # The build-dist stub lays out a runner that records its arguments and
  # whether TOOLBELT_LINTS_DIR points at the lints directory beside it, and a
  # check-toolbelt-pins that records where it ran and what it was handed.
  cat > .ci/build-dist << 'STUB'
#!/usr/bin/env bash
printf 'build-dist %s\n' "$*" >> "${CALLS}"
if [[ -n "${BUILD_STATUS:-}" ]]; then
  exit "${BUILD_STATUS}"
fi
mkdir --parents "$1/bin" "$1/lints"
cat > "$1/bin/toolbelt" << 'RUNNER'
#!/usr/bin/env bash
printf 'toolbelt %s\n' "$*" >> "${CALLS}"
if [[ "$(realpath "${TOOLBELT_LINTS_DIR}")" == "$(realpath "$(dirname "$0")/../lints")" ]]; then
  printf 'lints dir ok\n' >> "${CALLS}"
fi
if [[ "$1" == update-coverage-baseline ]]; then
  exit "${BASELINE_STATUS:-0}"
fi
exit "${RUNNER_STATUS:-0}"
RUNNER
chmod +x "$1/bin/toolbelt"
mkdir --parents "$1/lints/check-toolbelt-pins"
cat > "$1/lints/check-toolbelt-pins/check-toolbelt-pins" << 'PINS'
#!/usr/bin/env bash
printf 'check-toolbelt-pins in %s section %s hook %s\n' \
  "$(git rev-parse --show-prefix)" "$(cat "${TOOLBELT_SECTION}")" "${TOOLBELT_HOOK}" >> "${CALLS}"
exit "${PINS_STATUS:-0}"
PINS
chmod +x "$1/lints/check-toolbelt-pins/check-toolbelt-pins"
STUB
  chmod +x .ci/build-dist
  # The stubs must shadow the real tools for this test only.
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
  git add .ci run-tests
  git -c user.name=t -c user.email=t@example.invalid commit --quiet --message 'test: fixture'
}

# @description Write an executable stub that records its call and exits with
#              the status in the named variable.
# @arg $1 path where to write the stub
# @arg $2 status_var the variable holding its exit status
function write_stub() {
  local -r path="$1"
  local -r status_var="$2"
  cat > "${path}" << STUB
#!/usr/bin/env bash
printf '%s %s\n' "\${0##*/}" "\$*" >> "\${CALLS}"
exit "\${${status_var}:-0}"
STUB
  chmod +x "${path}"
}

@test 'runs every check in order, the dogfooded lints last through the built runner' {
  run "${REPO_DIR}/run-all-checks"
  assert_success
  run cat "${CALLS}"
  assert_line --index 0 "nix flake check --no-eval-cache ${FIXTURE_REPO}"
  assert_line --index 1 'run-lint-checks '
  assert_line --index 2 'check-examples '
  assert_line --index 3 'run-tests '
  assert_line --index 4 --regexp '^build-dist /.+/dist$'
  assert_line --index 5 \
    "${TOOLBELT_RUN} check-workflow-hermetic check-issue-numbers check-coverage-drop check-patch-coverage"
  assert_line --index 6 'lints dir ok'
  assert_line --index 7 'toolbelt update-coverage-baseline --check'
  assert_line --index 8 'check-toolbelt-pins in examples/consumer/ section null hook ci'
}

@test 'leaves no scratch directory behind' {
  run "${REPO_DIR}/run-all-checks"
  assert_success
  local built
  built="$(sed --quiet 's|^build-dist \(.*\)/dist$|\1|p' "${CALLS}")"
  assert [ -n "${built}" ]
  assert [ ! -e "${built}" ]
}

@test 'a failing check fails the gate and the later checks still run' {
  local step
  for step in NIX_STATUS LINT_STATUS EXAMPLES_STATUS RUNNER_STATUS BASELINE_STATUS PINS_STATUS; do
    : > "${CALLS}"
    run env "${step}=1" "${REPO_DIR}/run-all-checks"
    assert_failure 1
    assert_output --partial 'one or more checks failed'
    run grep --count '' "${CALLS}"
    assert_output 9
  done
}

@test 'failing tests fail the gate and run the dogfooded lints without the coverage lints' {
  TESTS_STATUS=1 run "${REPO_DIR}/run-all-checks"
  assert_failure 1
  assert_output --partial 'skipping the coverage lints: the tests failed'
  assert_output --partial 'one or more checks failed'
  run grep '^toolbelt ' "${CALLS}"
  assert_output "${TOOLBELT_RUN} check-workflow-hermetic check-issue-numbers"
  run grep --count '^check-toolbelt-pins ' "${CALLS}"
  assert_output 1
}

@test 'a stale baseline fails the gate' {
  BASELINE_STATUS=1 run "${REPO_DIR}/run-all-checks"
  assert_failure 1
  assert_output --partial 'one or more checks failed'
  run grep '^toolbelt update-coverage-baseline' "${CALLS}"
  assert_output 'toolbelt update-coverage-baseline --check'
}

@test 'the baseline check is skipped when the tests fail' {
  TESTS_STATUS=1 run "${REPO_DIR}/run-all-checks"
  assert_failure 1
  run grep --count '^toolbelt update-coverage-baseline' "${CALLS}"
  assert_output 0
}

@test 'a layout that does not build fails the gate without running the runner' {
  BUILD_STATUS=1 run "${REPO_DIR}/run-all-checks"
  assert_failure 1
  assert_output --partial 'one or more checks failed'
  run grep --count '^toolbelt \|^check-toolbelt-pins ' "${CALLS}"
  assert_output 0
}

@test 'warns about untracked files and passes' {
  touch untracked.txt
  run "${REPO_DIR}/run-all-checks"
  assert_success
  assert_output --partial 'untracked: untracked.txt'
}
