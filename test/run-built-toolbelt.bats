#!/usr/bin/env bats

# run-built-toolbelt runs in a fixture repo whose .ci/build-dist is the stub
# test_helper/built-runner describes, so no real build or runner runs.

function setup() {
  load test_helper/common
  load test_helper/built-runner
  make_repo
  CALLS="${BATS_TEST_TMPDIR}/calls"
  export CALLS
  write_build_dist_stub
}

@test 'builds the layout, then runs the built runner with the arguments and both directories set' {
  run "${REPO_DIR}/.ci/run-built-toolbelt" run --hook pre-commit check-x-formatted
  assert_success
  run cat "${CALLS}"
  assert_line --index 0 --regexp '^build-dist /.+/dist$'
  assert_line --index 1 'toolbelt run --hook pre-commit check-x-formatted'
  assert_line --index 2 'lints dir ok'
  assert_line --index 3 'formatters dir ok'
  assert_line --index 4 "ran in ${FIXTURE_REPO}"
  assert_equal "${#lines[@]}" 5
}

@test 'runs the runner in the directory it was called from' {
  mkdir sub
  cd sub
  run "${REPO_DIR}/.ci/run-built-toolbelt" fmt x
  assert_success
  run grep '^ran in ' "${CALLS}"
  assert_output "ran in ${FIXTURE_REPO}/sub"
}

@test 'exits with the runner status and reports no crash' {
  local status_code
  for status_code in 1 2; do
    : > "${CALLS}"
    RUNNER_STATUS="${status_code}" run "${REPO_DIR}/.ci/run-built-toolbelt" fmt x
    assert_failure "${status_code}"
    refute_output --partial 'ERROR: line'
  done
}

@test 'leaves no scratch directory behind, whatever the runner status' {
  local status_code built
  for status_code in 0 1; do
    : > "${CALLS}"
    RUNNER_STATUS="${status_code}" run "${REPO_DIR}/.ci/run-built-toolbelt" fmt x
    built="$(built_scratch_dir)"
    assert [ -n "${built}" ]
    assert [ ! -e "${built}" ]
  done
}

@test 'a layout that does not build is a setup failure: exit 2, no runner, no scratch directory' {
  BUILD_STATUS=1 run "${REPO_DIR}/.ci/run-built-toolbelt" fmt x
  assert_failure 2
  assert_output --partial 'run-built-toolbelt: the layout did not build'
  run grep --count '^toolbelt ' "${CALLS}"
  assert_output 0
  local built
  built="$(built_scratch_dir)"
  assert [ -n "${built}" ]
  assert [ ! -e "${built}" ]
}

@test 'exits 1 with usage when given no arguments, building nothing' {
  run "${REPO_DIR}/.ci/run-built-toolbelt"
  assert_failure 1
  assert_output 'usage: run-built-toolbelt <toolbelt-argument>...'
  assert [ ! -e "${CALLS}" ]
}

@test 'checks the arguments before making a scratch directory' {
  # With no usable TMPDIR, making the scratch directory fails, so only a run
  # that checks its arguments first can print the usage.
  TMPDIR="${BATS_TEST_TMPDIR}/no-such-dir" run "${REPO_DIR}/.ci/run-built-toolbelt"
  assert_failure 1
  assert_output 'usage: run-built-toolbelt <toolbelt-argument>...'
}
