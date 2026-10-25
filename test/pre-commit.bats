#!/usr/bin/env bats

# The pre-commit hook runs in a fixture repo holding a copy of the real
# .ci/run-built-toolbelt, the build-dist stub test_helper/built-runner
# describes, and a .ci/in-devshell stub that records its arguments and then
# runs them, so the whole chain runs with nothing real built or checked.

function setup() {
  load test_helper/common
  load test_helper/built-runner
  make_repo
  CALLS="${BATS_TEST_TMPDIR}/calls"
  export CALLS
  write_build_dist_stub
  cp -- "${REPO_DIR}/.ci/run-built-toolbelt" .ci/run-built-toolbelt
  cat > .ci/in-devshell << 'STUB'
#!/usr/bin/env bash
printf 'in-devshell %s\n' "$*" >> "${CALLS}"
exec "$@"
STUB
  chmod +x .ci/in-devshell
}

@test 'checks the dogfooded formatters through the devShell and the built runner' {
  run "${REPO_DIR}/.githooks/pre-commit"
  assert_success
  assert_output --partial 'pre-commit: formatting check passed'
  run cat "${CALLS}"
  assert_line --index 0 'in-devshell ./.ci/run-built-toolbelt run --hook pre-commit check-shell-formatted'
  assert_line --index 1 --regexp '^build-dist /.+/dist$'
  assert_line --index 2 'toolbelt run --hook pre-commit check-shell-formatted'
  assert_line --index 3 'lints dir ok'
  assert_line --index 4 'formatters dir ok'
  assert_line --index 5 "ran in ${FIXTURE_REPO}"
  assert_equal "${#lines[@]}" 6
}

@test 'never rewrites: the runner is never asked to fmt' {
  run "${REPO_DIR}/.githooks/pre-commit"
  assert_success
  run grep --count '^toolbelt fmt' "${CALLS}"
  assert_output 0
}

@test 'a finding or a runner error fails the commit and names the fix' {
  local status_code
  for status_code in 1 2; do
    : > "${CALLS}"
    RUNNER_STATUS="${status_code}" run "${REPO_DIR}/.githooks/pre-commit"
    assert_failure 1
    assert_output --partial 'pre-commit: formatting check failed; run ./.ci/in-devshell ./.ci/fmt and stage the result'
    refute_output --partial 'ERROR: line'
  done
}

@test 'leaves no scratch directory behind' {
  RUNNER_STATUS=1 run "${REPO_DIR}/.githooks/pre-commit"
  local built
  built="$(built_scratch_dir)"
  assert [ -n "${built}" ]
  assert [ ! -e "${built}" ]
}

@test 'a layout that does not build fails the commit' {
  BUILD_STATUS=1 run "${REPO_DIR}/.githooks/pre-commit"
  assert_failure 1
  assert_output --partial 'pre-commit: formatting check failed'
  run grep --count '^toolbelt ' "${CALLS}"
  assert_output 0
}
