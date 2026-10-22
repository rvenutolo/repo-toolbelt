#!/usr/bin/env bats

# run-all-checks runs in a fixture repo, so every step it calls by path is a
# stub there, and stub `nix` and `go` come first on PATH. Each stub appends
# its name and arguments to CALLS, and exits with the status in its
# <NAME>_STATUS variable (0 when unset).

function setup() {
  load test_helper/common
  make_repo
  CALLS="${BATS_TEST_TMPDIR}/calls"
  export CALLS
  mkdir "${BATS_TEST_TMPDIR}/bin" .ci
  write_stub "${BATS_TEST_TMPDIR}/bin/nix" NIX_STATUS
  write_stub .ci/run-lint-checks LINT_STATUS
  write_stub run-tests TESTS_STATUS
  # The go stub stands in for both builds and the runner. A build creates
  # the file it is told to write; the runner records whether each lint is
  # where TOOLBELT_LINTS_DIR says the runner looks for it.
  cat > "${BATS_TEST_TMPDIR}/bin/go" << 'STUB'
#!/usr/bin/env bash
printf 'go %s\n' "$*" >> "${CALLS}"
case "$1" in
  build)
    if [[ -n "${BUILD_STATUS:-}" ]]; then
      exit "${BUILD_STATUS}"
    fi
    mkdir --parents "$(dirname "$3")"
    touch "$3"
    ;;
  run)
    for lint in check-coverage-drop check-patch-coverage; do
      if [[ -f "${TOOLBELT_LINTS_DIR}/${lint}/${lint}" ]]; then
        printf 'runner sees %s\n' "${lint}" >> "${CALLS}"
      fi
    done
    exit "${RUNNER_STATUS:-0}"
    ;;
esac
STUB
  chmod +x "${BATS_TEST_TMPDIR}/bin/go"
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

@test 'runs every check in order, the coverage lints last through the runner' {
  run "${REPO_DIR}/run-all-checks"
  assert_success
  run cat "${CALLS}"
  assert_line --index 0 "nix flake check --no-eval-cache ${FIXTURE_REPO}"
  assert_line --index 1 'run-lint-checks '
  assert_line --index 2 'run-tests '
  assert_line --index 3 --regexp '^go build -o /.*/lints/check-coverage-drop/check-coverage-drop \./cmd/check-coverage-drop$'
  assert_line --index 4 --regexp '^go build -o /.*/lints/check-patch-coverage/check-patch-coverage \./cmd/check-patch-coverage$'
  assert_line --index 5 'go run ./cmd/toolbelt run --hook ci --base origin/main --head HEAD --default-branch main check-coverage-drop check-patch-coverage'
  assert_line --index 6 'runner sees check-coverage-drop'
  assert_line --index 7 'runner sees check-patch-coverage'
}

@test 'leaves no scratch directory behind' {
  run "${REPO_DIR}/run-all-checks"
  assert_success
  local built
  built="$(sed --quiet 's|^go build -o \(.*\)/lints/check-coverage-drop/.*|\1|p' "${CALLS}")"
  assert [ -n "${built}" ]
  assert [ ! -e "${built}" ]
}

@test 'a failing check fails the gate and the later checks still run' {
  local step
  for step in NIX_STATUS LINT_STATUS RUNNER_STATUS; do
    : > "${CALLS}"
    run env "${step}=1" "${REPO_DIR}/run-all-checks"
    assert_failure 1
    assert_output --partial 'one or more checks failed'
    run grep --count '' "${CALLS}"
    assert_output 8
  done
}

@test 'failing tests fail the gate and skip the coverage lints' {
  TESTS_STATUS=1 run "${REPO_DIR}/run-all-checks"
  assert_failure 1
  assert_output --partial 'skipping the coverage lints: the tests failed'
  assert_output --partial 'one or more checks failed'
  run grep --count '^go ' "${CALLS}"
  assert_output 0
}

@test 'a lint that does not build fails the gate without running the runner' {
  BUILD_STATUS=1 run "${REPO_DIR}/run-all-checks"
  assert_failure 1
  assert_output --partial 'one or more checks failed'
  run grep --count '^go run' "${CALLS}"
  assert_output 0
}

@test 'warns about untracked files and passes' {
  touch untracked.txt
  run "${REPO_DIR}/run-all-checks"
  assert_success
  assert_output --partial 'untracked: untracked.txt'
}
