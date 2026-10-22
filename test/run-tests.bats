#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# Stubs stand in for go, bats, kcov and .ci/check-kcov-hits; each prints a
# marker and the arguments it received, so no real suite runs. run-tests runs
# from a fixture repo, so its `rm` targets the fixture's .coverage/bats.

# @description Write an executable stub that prints "<name>-ran <args>" and
#              exits with $<NAME>_EXIT, defaulting to 0.
# @arg $1 path where the stub is written
# @arg $2 name the stub reports
# @arg $3 exit_var name of the environment variable holding the exit status
function write_stub() {
  # shellcheck disable=SC2016 # the stub's own expansions must stay literal in the file it writes
  printf '#!/usr/bin/env bash\nprintf "%s-ran %%s\\n" "$*"\nexit "${%s:-0}"\n' "$2" "$3" >"$1"
  chmod +x "$1"
}

function setup() {
  load test_helper/common
  make_repo
  STUB_BIN="${BATS_TEST_TMPDIR}/bin"
  mkdir "${STUB_BIN}" .ci lib lints
  write_stub "${STUB_BIN}/go" go GO_EXIT
  write_stub "${STUB_BIN}/bats" bats BATS_EXIT
  write_stub "${STUB_BIN}/kcov" kcov KCOV_EXIT
  write_stub .ci/check-kcov-hits kcov-hits KCOV_HITS_EXIT
  # Only the stub dir goes ahead of the devShell PATH.
  PATH="${STUB_BIN}:${PATH}"
}

@test 'forwards arguments to bats and measures nothing' {
  run "${REPO_DIR}/run-tests" foo.bats --tap
  assert_success
  assert_output 'bats-ran foo.bats --tap'
  refute_output --partial 'go-ran'
  refute_output --partial 'kcov-ran'
}

@test 'runs go test with the documented flags' {
  run --separate-stderr "${REPO_DIR}/run-tests"
  assert_success
  assert_output --partial 'go-ran test -short -race -count=1 -shuffle=on -timeout 10m -coverprofile=.coverage/go/coverage.out ./...'
}

@test 'runs bats under kcov over the measured trees' {
  run --separate-stderr "${REPO_DIR}/run-tests"
  assert_success
  local -r include="--include-path=${FIXTURE_REPO}/lib,${FIXTURE_REPO}/lints,${FIXTURE_REPO}/.ci,${FIXTURE_REPO}/run-all-checks,${FIXTURE_REPO}/run-tests"
  local -r parse="--bash-parse-files-in-dir=${FIXTURE_REPO},${FIXTURE_REPO}/lib,${FIXTURE_REPO}/lints,${FIXTURE_REPO}/.ci"
  assert_output --partial "${include}"
  assert_output --partial "${parse}"
  assert_output --regexp "kcov-ran .* ${FIXTURE_REPO}/.coverage/bats bats --recursive --filter-tags !e2e test/"
}

@test 'skips a measured directory that does not exist' {
  rmdir lints
  run --separate-stderr "${REPO_DIR}/run-tests"
  assert_success
  refute_output --partial '/lints'
  assert_output --partial "--include-path=${FIXTURE_REPO}/lib,${FIXTURE_REPO}/.ci,"
}

@test 'checks the kcov report' {
  run --separate-stderr "${REPO_DIR}/run-tests"
  assert_success
  assert_output --partial 'kcov-hits-ran .coverage/bats/bats/cobertura.xml'
}

@test 'a failing Go suite fails the run and BATS still runs' {
  GO_EXIT=1 run --separate-stderr "${REPO_DIR}/run-tests"
  assert_failure 1
  assert_output --partial 'kcov-ran'
  [[ "${stderr}" == *'one or more test suites failed'* ]]
}

@test 'a failing BATS suite fails the run' {
  KCOV_EXIT=1 run --separate-stderr "${REPO_DIR}/run-tests"
  assert_failure 1
  [[ "${stderr}" == *'one or more test suites failed'* ]]
}

@test 'a failing kcov-hits check fails the run' {
  KCOV_HITS_EXIT=1 run --separate-stderr "${REPO_DIR}/run-tests"
  assert_failure 1
  [[ "${stderr}" == *'one or more test suites failed'* ]]
}

@test 'stops when a required tool is missing' {
  local tool other
  for tool in bats go kcov; do
    # A PATH holding the basics run-tests and the stubs need, and every tool
    # but the one under test.
    local bin="${BATS_TEST_TMPDIR}/path-without-${tool}"
    mkdir "${bin}"
    for other in bash env git date mkdir rm; do
      ln --symbolic "$(type -P "${other}")" "${bin}/${other}"
    done
    for other in bats go kcov; do
      if [[ "${other}" != "${tool}" ]]; then
        ln --symbolic "${STUB_BIN}/${other}" "${bin}/${other}"
      fi
    done
    PATH="${bin}" run --separate-stderr "${REPO_DIR}/run-tests"
    assert_failure 1
    [[ "${stderr}" == *"${tool} not found on PATH"* ]]
  done
}

@test 'clears a previous kcov report' {
  mkdir --parents .coverage/bats
  touch .coverage/bats/marker
  run --separate-stderr "${REPO_DIR}/run-tests"
  assert_success
  [[ ! -e .coverage/bats/marker ]]
}
